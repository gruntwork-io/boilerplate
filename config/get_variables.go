package config

import (
	"fmt"
	"log"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/pterm/pterm"
)

const MaxReferenceDepth = 20

// Get a value for each of the variables specified in boilerplateConfig, other than those already in existingVariables.
// The value for a variable can come from the user (if the  non-interactive option isn't set), the default value in the
// config, or a command line option.
func GetVariables(opts *options.BoilerplateOptions, boilerplateConfig, rootBoilerplateConfig *BoilerplateConfig, thisDep variables.Dependency) (map[string]interface{}, error) {
	renderedVariables := map[string]interface{}{}

	// Add a variable for all variables contained in the root config file. This will allow Golang template users
	// to directly access these with an expression like "{{ .BoilerplateConfigVars.foo.Default }}"
	rootConfigVars := map[string]variables.Variable{}
	for _, configVar := range rootBoilerplateConfig.Variables {
		rootConfigVars[configVar.Name()] = configVar
	}
	renderedVariables["BoilerplateConfigVars"] = rootConfigVars

	// Add a variable for all dependencies contained in the root config file. This will allow Golang template users
	// to directly access these with an expression like "{{ .BoilerplateConfigDeps.foo.OutputFolder }}"
	rootConfigDeps := map[string]variables.Dependency{}
	for _, dep := range rootBoilerplateConfig.Dependencies {
		rootConfigDeps[dep.Name] = dep
	}
	renderedVariables["BoilerplateConfigDeps"] = rootConfigDeps

	// Add a variable for "the boilerplate template currently being processed".
	thisTemplateProps := map[string]interface{}{}
	thisTemplateProps["Config"] = boilerplateConfig
	thisTemplateProps["Options"] = opts
	thisTemplateProps["CurrentDep"] = thisDep
	renderedVariables["This"] = thisTemplateProps

	// The variables up to this point don't need any additional processing as they are builtin. User defined variables
	// can reference and use Go template syntax, so we pass them through a rendering pipeline to ensure they are
	// evaluated to values that can be used in the rest of the templates.
	variablesToRender := map[string]interface{}{}

	// Collect the variable values that have been passed in from the command line.
	for key, value := range opts.Vars {
		variablesToRender[key] = value
	}

	// Collect the variable values that are defined in the config and get the value.
	variablesInConfig := getAllVariablesInConfig(boilerplateConfig)
	for _, variable := range variablesInConfig {
		unmarshalled, err := getValueForVariable(variable, variablesInConfig, variablesToRender, opts, 0)
		if err != nil {
			return nil, err
		}
		variablesToRender[variable.Name()] = unmarshalled
	}

	// Pass all the user provided variables through a rendering pipeline to ensure they are evaluated down to
	// primitives.
	newlyRenderedVariables, err := render.RenderVariables(opts, variablesToRender, renderedVariables)
	if err != nil {
		return nil, err
	}

	// Convert all the rendered variables to match the type definition in the boilerplate config.
	for _, variable := range variablesInConfig {
		renderedValue := newlyRenderedVariables[variable.Name()]
		renderedValueWithType, err := variables.ConvertType(renderedValue, variable)
		if err != nil {
			return nil, err
		}
		renderedVariables[variable.Name()] = renderedValueWithType
	}

	return renderedVariables, nil
}

func getValueForVariable(variable variables.Variable, variablesInConfig map[string]variables.Variable, valuesForPreviousVariables map[string]interface{}, opts *options.BoilerplateOptions, referenceDepth int) (interface{}, error) {
	if referenceDepth > MaxReferenceDepth {
		return nil, errors.WithStackTrace(CyclicalReference{VariableName: variable.Name(), ReferenceName: variable.Reference()})
	}

	value, alreadyExists := valuesForPreviousVariables[variable.Name()]
	if alreadyExists {
		return value, nil
	}

	if variable.Reference() != "" {
		value, alreadyExists := valuesForPreviousVariables[variable.Reference()]
		if alreadyExists {
			return value, nil
		}

		reference, containsReference := variablesInConfig[variable.Reference()]
		if !containsReference {
			return nil, errors.WithStackTrace(MissingReference{VariableName: variable.Name(), ReferenceName: variable.Reference()})
		}
		return getValueForVariable(reference, variablesInConfig, valuesForPreviousVariables, opts, referenceDepth+1)
	}

	return getVariable(variable, opts)
}

// Get all the variables defined in the given config and its dependencies
func getAllVariablesInConfig(boilerplateConfig *BoilerplateConfig) map[string]variables.Variable {
	allVariables := map[string]variables.Variable{}

	for _, variable := range boilerplateConfig.Variables {
		allVariables[variable.Name()] = variable
	}

	for _, dependency := range boilerplateConfig.Dependencies {
		for _, variable := range dependency.GetNamespacedVariables() {
			allVariables[variable.Name()] = variable
		}
	}

	return allVariables
}

// Get a value for the given variable. The value can come from the user (if the non-interactive option isn't set), the
// default value in the config, or a command line option.
func getVariable(variable variables.Variable, opts *options.BoilerplateOptions) (interface{}, error) {
	valueFromVars, valueSpecifiedInVars := getVariableFromVars(variable, opts)

	if valueSpecifiedInVars {
		util.Logger.Printf("Using value specified via command line options for variable '%s': %s", variable.FullName(), valueFromVars)
		return valueFromVars, nil
	} else if opts.NonInteractive && variable.Default() != nil {
		util.Logger.Printf("Using default value for variable '%s': %v", variable.FullName(), variable.Default())
		return variable.Default(), nil
	} else if opts.NonInteractive {
		return nil, errors.WithStackTrace(MissingVariableWithNonInteractiveMode(variable.FullName()))
	} else {
		return getVariableFromUser(variable, opts, InvalidEntries{})
	}
}

// Return the value of the given variable from vars passed in as command line options
func getVariableFromVars(variable variables.Variable, opts *options.BoilerplateOptions) (interface{}, bool) {
	for name, value := range opts.Vars {
		if name == variable.Name() {
			return value, true
		}
	}

	return nil, false
}

type InvalidEntries struct {
	Issues []ValidationIssue
}

type ValidationIssue struct {
	Value         interface{}
	ValidationMap map[string]bool
}

func renderValidationErrors(val interface{}, m map[string]bool) {
	clearTerminal()
	pterm.Warning.WithPrefix(pterm.Prefix{Text: "Invalid entry"}).Println(val)
	for k, v := range m {
		if v == false {
			pterm.Error.Println(k)
		} else if v == true {
			pterm.Success.Println(k)
		}
	}
}

func clearTerminal() {
	print("\033[H\033[2J")
}

func renderMapVariablePrompts(variable variables.Variable, requirements []string) {
	clearTerminal()

	pterm.Println(pterm.Green(variable.FullName()))

	if variable.Description() != "" {
		pterm.Println(pterm.LightGreen(variable.Description()))
	}
	pterm.Println(pterm.LightGreen("Validation Requirements:"))
	for _, requirement := range requirements {
		pterm.Println(pterm.LightGreen("- " + requirement))
	}
}

func renderVariablePrompts(variable variables.Variable, invalidEntries InvalidEntries) {
	pterm.Println(pterm.Green(variable.FullName()))

	if variable.Description() != "" {
		pterm.Println(pterm.Yellow(variable.Description()))
	}

	if len(invalidEntries.Issues) > 0 {
		renderValidationErrors(invalidEntries.Issues[0].Value, invalidEntries.Issues[0].ValidationMap)
	}
}

// Get the value for the given variable by prompting the user
func getVariableFromUser(variable variables.Variable, opts *options.BoilerplateOptions, invalidEntries InvalidEntries) (interface{}, error) {

	// Add a newline for legibility and padding
	fmt.Println()

	// Show the current variable's name, description, and also render any validation errors in real-time so the user knows what's wrong
	// with their input
	renderVariablePrompts(variable, invalidEntries)

	value := ""
	if variable.Type() == variables.String {
		prompt := &survey.Input{
			Message: fmt.Sprintf("Please enter your %s", variable.FullName()),
		}
		err := survey.AskOne(prompt, &value)
		if err != nil {
			if err == terminal.InterruptErr {
				log.Fatal("quit")
			}
			return value, err
		}
	} else if variable.Type() == variables.Enum {
		prompt := &survey.Select{
			Message: fmt.Sprintf("Please select your %s", variable.FullName()),
			Options: variable.Options(),
		}
		err := survey.AskOne(prompt, &value)
		if err != nil {
			if err == terminal.InterruptErr {
				log.Fatal("quit")
			}
			return value, err
		}
	}

	if len(variable.Validations()) > 0 {
		m := make(map[string]bool)
		var hasValidationErrs = false
		for _, customValidation := range variable.Validations() {
			// Run the specific validation against the user-provided value and store it in the map
			err := validation.Validate(value, customValidation.Validator)
			val := true
			if err != nil {
				hasValidationErrs = true
				val = false
			}
			m[customValidation.DescriptionText()] = val
		}
		if hasValidationErrs {
			ie := InvalidEntries{
				Issues: []ValidationIssue{
					{Value: value,
						ValidationMap: m},
				},
			}
			return getVariableFromUser(variable, opts, ie)
		}
	}

	if value == "" {
		// TODO: what if the user wanted an empty string instead of the default?
		util.Logger.Printf("Using default value for variable '%s': %v", variable.FullName(), variable.Default())
		return variable.Default(), nil
	}

	clearTerminal()
	return variables.ParseYamlString(value)
}

// Custom error types

type MissingVariableWithNonInteractiveMode string

func (variableName MissingVariableWithNonInteractiveMode) Error() string {
	return fmt.Sprintf("Variable '%s' does not have a default, no value was specified at the command line using the --%s option, and the --%s flag is set, so cannot prompt user for a value.", string(variableName), options.OptVar, options.OptNonInteractive)
}

type MissingReference struct {
	VariableName  string
	ReferenceName string
}

func (err MissingReference) Error() string {
	return fmt.Sprintf("Variable %s references unknown variable %s", err.VariableName, err.ReferenceName)
}

type CyclicalReference struct {
	VariableName  string
	ReferenceName string
}

func (err CyclicalReference) Error() string {
	return fmt.Sprintf("Variable %s seems to have an cyclical reference with variable %s", err.VariableName, err.ReferenceName)
}

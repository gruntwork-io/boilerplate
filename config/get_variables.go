package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

const MaxReferenceDepth = 20

// Get a value for each of the variables specified in boilerplateConfig, other than those already in existingVariables.
// The value for a variable can come from the user (if the  non-interactive option isn't set), the default value in the
// config, or a command line option.
func GetVariables(opts *options.BoilerplateOptions, boilerplateConfig, rootBoilerplateConfig *BoilerplateConfig, thisDep variables.Dependency) (map[string]interface{}, error) {
	renderedVariables := map[string]interface{}{}

	// Add a variable for all variables contained in the root config file. This will allow Golang template users
	// to directly access these with an expression like "{{ .BoilerplateConfigVars.foo.Default }}"
	rootConfigVars := rootBoilerplateConfig.GetVariablesMap()
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
	variablesInConfig := boilerplateConfig.GetVariablesMap()
	for _, variable := range variablesInConfig {
		unmarshalled, err := GetValueForVariable(variable, variablesInConfig, variablesToRender, opts, 0)
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

func GetValueForVariable(
	variable variables.Variable,
	variablesInConfig map[string]variables.Variable,
	valuesForPreviousVariables map[string]interface{},
	opts *options.BoilerplateOptions,
	referenceDepth int,
) (interface{}, error) {
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
		return GetValueForVariable(reference, variablesInConfig, valuesForPreviousVariables, opts, referenceDepth+1)
	}

	return getVariable(variable, opts)
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
		return getVariableFromUser(variable, opts)
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

// Get the value for the given variable by prompting the user
func getVariableFromUser(variable variables.Variable, opts *options.BoilerplateOptions) (interface{}, error) {
	util.BRIGHT_GREEN.Printf("\n%s\n", variable.FullName())
	if variable.Description() != "" {
		fmt.Printf("  %s\n", variable.Description())
	}

	helpText := []string{
		fmt.Sprintf("type: %s", variable.Type()),
	}

	if variable.ExampleValue() != "" {
		helpText = append(helpText, fmt.Sprintf("example value: %s", variable.ExampleValue()))
	}

	if variable.Default() != nil {
		helpText = append(helpText, fmt.Sprintf("default: %s", variable.Default()))
	}

	fmt.Printf("  (%s)\n", strings.Join(helpText, ", "))
	fmt.Println()

	value := ""
	// Display rich prompts to the user, based on the type of variable we're asking for
	switch variable.Type() {
	case variables.String:
		prompt := &survey.Input{
			Message: fmt.Sprintf("Please enter %s", variable.FullName()),
		}
		err := survey.AskOne(prompt, &value)
		if err != nil {
			if err == terminal.InterruptErr {
				log.Fatal("quit")
			}
			return value, err
		}
	case variables.Enum:
		prompt := &survey.Select{
			Message: fmt.Sprintf("Please select %s", variable.FullName()),
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

	if value == "" {
		// TODO: what if the user wanted an empty string instead of the default?
		util.Logger.Printf("Using default value for variable '%s': %v", variable.FullName(), variable.Default())
		return variable.Default(), nil
	}

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

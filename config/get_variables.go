package config

import (
	"fmt"
	"log"
	"sort"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/hashicorp/go-multierror"
	"github.com/inancgumus/screen"
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

	// Prior to prompting the user for the defined variable values, we sort the variables
	// by user-defined presentation order. Users may specify the order: value when defining
	// their variables in boilerplate.yml. This order value is an int, and it is used to
	// determine the relative ordering of the variables by key name
	// In essence, since Go maps do not preserve order for performance reasons, we need to
	// work around that and introduce a concept of use-defined ordering when prompting for
	// variables. We do this to provide a more coherent and easy to work with form filling
	// process for our Ref Arch customers

	// Create a slice of KeyOrderPairs, which we'll be able to sort according to order value
	keyAndOrderPairs := []KeyAndOrderPair{}

	// Pair the keys for each variable to its user-defined presentation order
	for key, variable := range variablesInConfig {
		kop := KeyAndOrderPair{
			Key:   key,
			Order: variable.Order(),
		}
		keyAndOrderPairs = append(keyAndOrderPairs, kop)
	}

	// Sort the KeyAndOrderPairs by their order value
	// N.B. this syntax of sort.Slice requires Go 1.18 or above!
	sort.Slice(keyAndOrderPairs[:], func(i, j int) bool {
		return keyAndOrderPairs[i].Order < keyAndOrderPairs[j].Order
	})

	// Now, instead of just iterating through the map keys naively,
	// iterate through the slice of KeyOrderPairs, which are sorted by order
	// which means that in each iteration of the loop, we can fetch the next variable
	// by looking up its key in the original config-provided variables map
	for _, keyOrderPair := range keyAndOrderPairs {
		variable := variablesInConfig[keyOrderPair.Key]
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

	// Run the value we receive from getVariable through validations, ensuring values provided by --var-files will also be checked
	value, err := getVariable(variable, opts)
	if err != nil {
		return value, err
	}
	var result *multierror.Error
	// Run the value through any defined validations for the variable
	for _, customValidation := range variable.Validations() {
		// Run the specific validation against the user-provided value and store it in the map
		err := validation.Validate(value, customValidation.Validator)
		result = multierror.Append(result, err)
	}
	return value, result.ErrorOrNil()
}

// Get a value for the given variable. The value can come from the user (if the non-interactive option isn't set), the
// default value in the config, or a command line option.
func getVariable(variable variables.Variable, opts *options.BoilerplateOptions) (interface{}, error) {
	valueFromVars, valueSpecifiedInVars := getVariableFromVars(variable, opts)

	if valueSpecifiedInVars {
		opts.Logger.Info(fmt.Sprintf("Using value specified via command line options for variable '%s': %s", variable.FullName(), valueFromVars))
		return valueFromVars, nil
	} else if opts.NonInteractive && variable.Default() != nil {
		opts.Logger.Info(fmt.Sprintf("Using default value for variable '%s': %v", variable.FullName(), variable.Default()))
		return variable.Default(), nil
	} else if opts.NonInteractive {
		return nil, errors.WithStackTrace(MissingVariableWithNonInteractiveMode(variable.FullName()))
	} else {
		return getVariableFromUser(variable, opts, variables.InvalidEntries{})
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
func getVariableFromUser(variable variables.Variable, opts *options.BoilerplateOptions, invalidEntries variables.InvalidEntries) (interface{}, error) {
	// Start by clearing any previous contents
	screen.Clear()

	// Add a newline for legibility and padding
	fmt.Println()

	// Show the current variable's name, description, and also render any validation errors in real-time so the user knows what's wrong
	// with their input
	renderVariablePrompts(variable, invalidEntries)

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

	var valueToValidate interface{}
	if value == "" {
		valueToValidate = variable.Default()
	} else {
		valueToValidate = value
	}
	// If any of the variable's validation rules are not satisfied by the user's submission,
	// store the validation errors in a map. We'll then recursively call get_variable_from_user
	// again, this time passing in the validation errors map, so that we can render to the terminal
	// the exact issues with each submission
	m := make(map[string]bool)
	hasValidationErrs := false
	for _, customValidation := range variable.Validations() {
		// Run the specific validation against the user-provided value and store it in the map
		err := validation.Validate(valueToValidate, customValidation.Validator)
		val := true
		if err != nil {
			hasValidationErrs = true
			val = false
		}
		m[customValidation.DescriptionText()] = val
	}
	if hasValidationErrs {
		ie := variables.InvalidEntries{
			Issues: []variables.ValidationIssue{
				{
					Value:         value,
					ValidationMap: m,
				},
			},
		}
		return getVariableFromUser(variable, opts, ie)
	}

	if value == "" {
		// TODO: what if the user wanted an empty string instead of the default?
		opts.Logger.Info(fmt.Sprintf("Using default value for variable '%s': %v", variable.FullName(), variable.Default()))
		return variable.Default(), nil
	}

	// Clear the terminal of all previous text for legibility
	util.ClearTerminal()

	if variable.Type() == variables.String {
		_, intErr := strconv.Atoi(value)
		if intErr == nil {
			value = fmt.Sprintf(`"%s"`, value)
		}
	}

	return variables.ParseYamlString(value)
}

// RenderValidationErrors displays in user-legible format the exact validation errors
// that the user's last submission generated
func renderValidationErrors(val interface{}, m map[string]bool) {
	util.ClearTerminal()
	pterm.Warning.WithPrefix(pterm.Prefix{Text: "Invalid entry"}).Println(val)
	for k, v := range m {
		if v {
			pterm.Success.Println(k)
		} else {
			pterm.Error.Println(k)
		}
	}
}

func renderVariablePrompts(variable variables.Variable, invalidEntries variables.InvalidEntries) {
	pterm.Println(pterm.Green(variable.FullName()))

	if variable.Description() != "" {
		pterm.Println(pterm.Yellow(variable.Description()))
	}

	if len(invalidEntries.Issues) > 0 {
		renderValidationErrors(invalidEntries.Issues[0].Value, invalidEntries.Issues[0].ValidationMap)
	}
}

// Custom types
// A KeyAndOrderPair is a composite of the user-defined order and the user's variable name
type KeyAndOrderPair struct {
	Key   string
	Order int
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

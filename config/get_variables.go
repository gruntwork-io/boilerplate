package config

import (
	"fmt"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/variables"
	"strings"
)

const MaxReferenceDepth = 20

// Get a value for each of the variables specified in boilerplateConfig, other than those already in existingVariables.
// The value for a variable can come from the user (if the  non-interactive option isn't set), the default value in the
// config, or a command line option.
func GetVariables(options *BoilerplateOptions, boilerplateConfig, rootBoilerplateConfig *BoilerplateConfig, thisDep variables.Dependency) (map[string]interface{}, error) {
	vars := map[string]interface{}{}
	for key, value := range options.Vars {
		vars[key] = value
	}

	// Add a variable for all variables contained in the root config file. This will allow Golang template users
	// to directly access these with an expression like "{{ .BoilerplateConfigVars.foo.Default }}"
	rootConfigVars := map[string]variables.Variable{}
	for _, configVar := range rootBoilerplateConfig.Variables {
		rootConfigVars[configVar.Name()] = configVar
	}
	vars["BoilerplateConfigVars"] = rootConfigVars

	// Add a variable for all dependencies contained in the root config file. This will allow Golang template users
	// to directly access these with an expression like "{{ .BoilerplateConfigDeps.foo.OutputFolder }}"
	rootConfigDeps := map[string]variables.Dependency{}
	for _, dep := range rootBoilerplateConfig.Dependencies {
		rootConfigDeps[dep.Name] = dep
	}
	vars["BoilerplateConfigDeps"] = rootConfigDeps

	// Add a variable for "the boilerplate template currently being processed.
	thisTemplateProps := map[string]interface{}{}
	thisTemplateProps["Config"] = boilerplateConfig
	thisTemplateProps["Options"] = options
	thisTemplateProps["CurrentDep"] = thisDep
	vars["This"] = thisTemplateProps

	variablesInConfig := getAllVariablesInConfig(boilerplateConfig)

	for _, variable := range variablesInConfig {
		unmarshalled, err := getUnmarshalledValueForVariable(variable, variablesInConfig, vars, options, 0)
		if err != nil {
			return nil, err
		}
		vars[variable.Name()] = unmarshalled
	}

	return vars, nil
}

func getUnmarshalledValueForVariable(variable variables.Variable, variablesInConfig map[string]variables.Variable, alreadyUnmarshalledVariables map[string]interface{}, options *BoilerplateOptions, referenceDepth int) (interface{}, error) {
	if referenceDepth > MaxReferenceDepth {
		return nil, errors.WithStackTrace(CyclicalReference{VariableName: variable.Name(), ReferenceName: variable.Reference()})
	}

	value, alreadyExists := alreadyUnmarshalledVariables[variable.Name()]
	if alreadyExists {
		return variables.UnmarshalValueForVariable(value, variable)
	}

	if variable.Reference() != "" {
		value, alreadyExists := alreadyUnmarshalledVariables[variable.Reference()]
		if alreadyExists {
			return variables.UnmarshalValueForVariable(value, variable)
		}

		reference, containsReference := variablesInConfig[variable.Reference()]
		if !containsReference {
			return nil, errors.WithStackTrace(MissingReference{VariableName: variable.Name(), ReferenceName: variable.Reference()})
		}
		return getUnmarshalledValueForVariable(reference, variablesInConfig, alreadyUnmarshalledVariables, options, referenceDepth + 1)
	}

	value, err := getVariable(variable, options)
	if err != nil {
		return nil, err
	}
	return variables.UnmarshalValueForVariable(value, variable)
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
func getVariable(variable variables.Variable, options *BoilerplateOptions) (interface{}, error) {
	valueFromVars, valueSpecifiedInVars := getVariableFromVars(variable, options)

	if valueSpecifiedInVars {
		util.Logger.Printf("Using value specified via command line options for variable '%s': %s", variable.FullName(), valueFromVars)
		return valueFromVars, nil
	} else if options.NonInteractive && variable.Default() != nil {
		util.Logger.Printf("Using default value for variable '%s': %v", variable.FullName(), variable.Default())
		return variable.Default(), nil
	} else if options.NonInteractive {
		return nil, errors.WithStackTrace(MissingVariableWithNonInteractiveMode(variable.FullName()))
	} else {
		return getVariableFromUser(variable, options)
	}
}

// Return the value of the given variable from vars passed in as command line options
func getVariableFromVars(variable variables.Variable, options *BoilerplateOptions) (interface{}, bool) {
	for name, value := range options.Vars {
		if name == variable.Name() {
			return value, true
		}
	}

	return nil, false
}

// Get the value for the given variable by prompting the user
func getVariableFromUser(variable variables.Variable, options *BoilerplateOptions) (interface{}, error) {
	util.BRIGHT_GREEN.Printf("\n%s\n", variable.FullName())
	if variable.Description() != "" {
		fmt.Printf("  %s\n", variable.Description())
	}

	helpText := []string{
		fmt.Sprintf("type: %s", variable.Type()),
		fmt.Sprintf("example value: %s", variable.ExampleValue()),

	}
	if variable.Default() != nil {
		helpText = append(helpText, fmt.Sprintf("default: %s", variable.Default()))
	}

	fmt.Printf("  (%s)\n", strings.Join(helpText, ", "))
	fmt.Println()

	value, err := util.PromptUserForInput("  Enter a value")
	if err != nil {
		return "", err
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
	return fmt.Sprintf("Variable '%s' does not have a default, no value was specified at the command line using the --%s option, and the --%s flag is set, so cannot prompt user for a value.", string(variableName), OPT_VAR, OPT_NON_INTERACTIVE)
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
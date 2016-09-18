package config

import (
	"fmt"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/errors"
)

// A single variable defined in a boilerplate.yml config file
type Variable struct {
	Name        string
	Description string
	Type        BoilerplateType
	Default     interface{}
	Options     []string
}

// Return the full name of this variable, which includes its name and the dependency it is for (if any) in a
// human-readable format
func (variable Variable) FullName() string {
	dependencyName, variableName := SplitIntoDependencyNameAndVariableName(variable.Name)
	if dependencyName == "" {
		return variableName
	} else {
		return fmt.Sprintf("%s (for dependency %s)", variableName, dependencyName)
	}
}

// Return a human-readable string representation of this variable
func (variable Variable) String() string {
	return fmt.Sprintf("Variable {Name: '%s', Description: '%s', Type: '%s', Default: '%v', Options: '%v'}", variable.Name, variable.Description, variable.Type, variable.Default, variable.Options)
}

// Implement the go-yaml unmarshal interface for Variable. We can't let go-yaml handle this itself because we need to:
//
// 1. Set Defaults for missing fields (e.g. Type)
// 2. Validate the type corresponds to the Default value
// 3. Validate Options are only specified for the Enum Type
func (variable *Variable) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var fields map[string]interface{}
	if err := unmarshal(&fields); err != nil {
		return err
	}

	if unmarshalled, err := UnmarshalVariable(fields); err != nil {
		return err
	} else {
		*variable = *unmarshalled
		return nil
	}
}

// Get a value for each of the variables specified in boilerplateConfig, other than those already in existingVariables.
// The value for a variable can come from the user (if the  non-interactive option isn't set), the default value in the
// config, or a command line option.
func GetVariables(options *BoilerplateOptions, boilerplateConfig *BoilerplateConfig) (map[string]interface{}, error) {
	variables := map[string]interface{}{}
	for key, value := range options.Vars {
		variables[key] = value
	}

	variablesInConfig := getAllVariablesInConfig(boilerplateConfig)

	for _, variable := range variablesInConfig {
		var value interface{}
		var err error

		value, alreadyExists := variables[variable.Name]
		if !alreadyExists {
			value, err = getVariable(variable, options)
			if err != nil {
				return variables, err
			}
		}

		variables[variable.Name], err = UnmarshalVariableValue(value, variable)
		if err != nil {
			return variables, err
		}
	}

	return variables, nil
}

// Get all the variables defined in the given config and its dependencies
func getAllVariablesInConfig(boilerplateConfig *BoilerplateConfig) []Variable {
	allVariables := []Variable{}

	allVariables = append(allVariables, boilerplateConfig.Variables...)

	for _, dependency := range boilerplateConfig.Dependencies {
		for _, variable := range dependency.Variables {
			variableName := fmt.Sprintf("%s.%s", dependency.Name, variable.Name)
			allVariables = append(allVariables, Variable{Name: variableName, Description: variable.Description, Type: variable.Type, Default: variable.Default, Options: variable.Options})
		}
	}

	return allVariables
}

// Get a value for the given variable. The value can come from the user (if the non-interactive option isn't set), the
// default value in the config, or a command line option.
func getVariable(variable Variable, options *BoilerplateOptions) (interface{}, error) {
	valueFromVars, valueSpecifiedInVars := getVariableFromVars(variable, options)

	if valueSpecifiedInVars {
		util.Logger.Printf("Using value specified via command line options for variable '%s': %s", variable.FullName(), valueFromVars)
		return valueFromVars, nil
	} else if options.NonInteractive && variable.Default != nil {
		// TODO: how to disambiguate between a default not being specified and a default set to an empty string?
		util.Logger.Printf("Using default value for variable '%s': %v", variable.FullName(), variable.Default)
		return variable.Default, nil
	} else if options.NonInteractive {
		return nil, errors.WithStackTrace(MissingVariableWithNonInteractiveMode(variable.FullName()))
	} else {
		return getVariableFromUser(variable, options)
	}
}

// Return the value of the given variable from vars passed in as command line options
func getVariableFromVars(variable Variable, options *BoilerplateOptions) (interface{}, bool) {
	for name, value := range options.Vars {
		if name == variable.Name {
			return value, true
		}
	}

	return nil, false
}

// Get the value for the given variable by prompting the user
func getVariableFromUser(variable Variable, options *BoilerplateOptions) (interface{}, error) {
	util.BRIGHT_GREEN.Printf("\n%s\n", variable.FullName())
	if variable.Description != "" {
		fmt.Printf("  %s\n", variable.Description)
	}
	if variable.Default != "" {
		fmt.Printf("  (default: %s)\n", variable.Default)
	}
	fmt.Println()

	value, err := util.PromptUserForInput("  Enter a value")
	if err != nil {
		return "", err
	}

	if value == "" {
		// TODO: what if the user wanted an empty string instead of the default?
		util.Logger.Printf("Using default value for variable '%s': %v", variable.FullName(), variable.Default)
		return variable.Default, nil
	}

	return parseYamlString(value)
}

// Custom error types

type MissingVariableWithNonInteractiveMode string
func (variableName MissingVariableWithNonInteractiveMode) Error() string {
	return fmt.Sprintf("Variable '%s' does not have a default, no value was specified at the command line using the --%s option, and the --%s flag is set, so cannot prompt user for a value.", string(variableName), OPT_VAR, OPT_NON_INTERACTIVE)
}
package config

import (
	"fmt"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/errors"
	"strings"
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

// Get a value for each of the variables specified in boilerplateConfig, other than those already in existingVariables.
// The value for a variable can come from the user (if the  non-interactive option isn't set), the default value in the
// config, or a command line option.
func GetVariables(options *BoilerplateOptions, boilerplateConfig *BoilerplateConfig) (map[string]string, error) {
	variables := map[string]string{}
	for key, value := range options.Vars {
		variables[key] = value
	}

	variablesInConfig := getAllVariablesInConfig(boilerplateConfig)

	for _, variable := range variablesInConfig {
		if _, alreadyExists := variables[variable.Name]; !alreadyExists {
			value, err := getVariable(variable, options)
			if err != nil {
				return variables, err
			}
			variables[variable.Name] = value
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
			allVariables = append(allVariables, Variable{Name: variableName, Prompt: variable.Prompt, Default: variable.Default})
		}
	}

	return allVariables
}

// Get a value for the given variable. The value can come from the user (if the non-interactive option isn't set), the
// default value in the config, or a command line option.
func getVariable(variable Variable, options *BoilerplateOptions) (string, error) {
	valueFromVars, valueSpecifiedInVars := getVariableFromVars(variable, options)

	if valueSpecifiedInVars {
		util.Logger.Printf("Using value specified via command line options for variable '%s': %s", variable.Description(), valueFromVars)
		return valueFromVars, nil
	} else if options.NonInteractive && variable.Default != "" {
		// TODO: how to disambiguate between a default not being specified and a default set to an empty string?
		util.Logger.Printf("Using default value for variable '%s': %s", variable.Description(), variable.Default)
		return variable.Default, nil
	} else if options.NonInteractive {
		return "", errors.WithStackTrace(MissingVariableWithNonInteractiveMode(variable.Description()))
	} else {
		return getVariableFromUser(variable, options)
	}
}

// Return the value of the given variable from vars passed in as command line options
func getVariableFromVars(variable Variable, options *BoilerplateOptions) (string, bool) {
	for name, value := range options.Vars {
		if name == variable.Name {
			return value, true
		}
	}

	return "", false
}

// Get the value for the given variable by prompting the user
func getVariableFromUser(variable Variable, options *BoilerplateOptions) (string, error) {
	util.BRIGHT_GREEN.Printf("\n%s\n", variable.Description())
	if variable.Prompt != "" {
		fmt.Printf("  %s\n", variable.Prompt)
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
		util.Logger.Printf("Using default value for variable '%s': %s", variable.Description(), variable.Default)
		return variable.Default, nil
	} else {
		return value, nil
	}
}

// Parse a list of NAME=VALUE pairs into a map.
func ParseVariablesFromKeyValuePairs(varsList []string)  (map[string]string, error) {
	vars := map[string]string{}

	for _, variable := range varsList {
		variableParts := strings.Split(variable, "=")
		if len(variableParts) != 2 {
			return vars, errors.WithStackTrace(InvalidVarSyntax(variable))
		}

		key := variableParts[0]
		value := variableParts[1]
		if key == "" {
			return vars, errors.WithStackTrace(VariableNameCannotBeEmpty(variable))
		}

		vars[key] = value
	}

	return vars, nil
}

// Parse a list of YAML files that define variables into a map.
func ParseVariablesFromVarFiles(varFileList []string) (map[string]string, error) {
	vars := map[string]string{}

	for _, varFile := range varFileList {
		varsInFile, err := ParseVariablesFromVarFile(varFile)
		if err != nil {
			return vars, err
		}
		vars = util.MergeMaps(vars, varsInFile)
	}

	return vars, nil
}

// Parse the NAME: VALUE pairs in the given YAML file into a map
func ParseVariablesFromVarFile(varFilePath string) (map[string]string, error) {
	bytes, err := ioutil.ReadFile(varFilePath)
	if err != nil {
		return map[string]string{}, errors.WithStackTrace(err)
	}
	return parseVariablesFromVarFileContents(bytes)
}

// Parse the NAME: VALUE pairs in the given YAML file contents into a map
func parseVariablesFromVarFileContents(varFileContents []byte)(map[string]string, error) {
	vars := map[string]string{}

	err := yaml.Unmarshal(varFileContents, &vars)
	if err != nil {
		return vars, errors.WithStackTrace(err)
	}

	return vars, nil
}

// Custom error types

type MissingVariableWithNonInteractiveMode string
func (variableName MissingVariableWithNonInteractiveMode) Error() string {
	return fmt.Sprintf("Variable '%s' does not have a default, no value was specified at the command line using the --%s option, and the --%s flag is set, so cannot prompt user for a value.", string(variableName), OPT_VAR, OPT_NON_INTERACTIVE)
}

type InvalidVarSyntax string
func (varSyntax InvalidVarSyntax) Error() string {
	return fmt.Sprintf("Invalid syntax for variable. Expected NAME=VALUE but got %s", string(varSyntax))
}

type VariableNameCannotBeEmpty string
func (varSyntax VariableNameCannotBeEmpty) Error() string {
	return fmt.Sprintf("Variable name cannot be empty. Expected NAME=VALUE but got %s", string(varSyntax))
}
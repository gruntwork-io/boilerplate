package config

import (
	"fmt"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/errors"
)

// Get a value for each of the variables specified in boilerplateConfig. The value can come from the user (if the
// non-interactive option isn't set), the default value in the default value in the config, or a command line option.
func GetVariables(options *BoilerplateOptions, boilerplateConfig *BoilerplateConfig) (map[string]string, error) {
	variables := map[string]string{}

	for _, variable := range boilerplateConfig.Variables {
		value, err := getVariable(variable, options)
		if err != nil {
			return variables, err
		}
		variables[variable.Name] = value
	}

	return variables, nil
}

// Get a value for the given variable. The value can come from the user (if the non-interactive option isn't set), the
// default value in the default value in the config, or a command line option.
func getVariable(variable Variable, options *BoilerplateOptions) (string, error) {
	valueFromVars, valueSpecifiedInVars := getVariableFromVars(variable, options)

	if valueSpecifiedInVars {
		util.Logger.Printf("Using value specified via the --%s flag for variable '%s': %s", OPT_VAR, variable.Name, valueFromVars)
		return valueFromVars, nil
	} else if options.NonInteractive && variable.Default != "" {
		// TODO: how to disambiguate between a default not being specified and a default set to an empty string?
		util.Logger.Printf("Using default value for variable '%s': %s", variable.Name, variable.Default)
		return variable.Default, nil
	} else if options.NonInteractive {
		return "", errors.WithStackTrace(MissingVariableWithNonInteractiveMode(variable.Name))
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
	prompt := fmt.Sprintf("Enter a value for variable '%s'", variable.Name)

	if variable.Prompt != "" {
		prompt = variable.Prompt
	}
	if variable.Default != "" {
		prompt = fmt.Sprintf("%s (default: %s)", prompt, variable.Default)
	}

	value, err := util.PromptUserForInput(prompt)
	if err != nil {
		return "", err
	}

	if value == "" {
		// TODO: what if the user wanted an empty string instead of the default?
		util.Logger.Printf("Using default value for variable '%s': %s", variable.Name, variable.Default)
		return variable.Default, nil
	} else {
		return value, nil
	}
}

// Custom error types

type MissingVariableWithNonInteractiveMode string
func (variableName MissingVariableWithNonInteractiveMode) Error() string {
	return fmt.Sprintf("Variable '%s' does not have a default, no value was specified at the command line using the --%s option, and the --%s flag is set, so cannot prompt user for a value.", variableName, OPT_VAR, OPT_NON_INTERACTIVE)
}
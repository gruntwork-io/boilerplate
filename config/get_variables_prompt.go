//go:build !(js && wasm)

package config

import (
	"errors"
	"fmt"
	"log"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	ozzo "github.com/go-ozzo/ozzo-validation"

	"github.com/gruntwork-io/boilerplate/internal/color"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/variables"
)

// Get the value for the given variable by prompting the user.
func getVariableFromUser(l logging.Logger, variable variables.Variable, invalidEntries variables.InvalidEntries) (any, error) {
	// Add a newline for legibility and padding
	fmt.Println()

	// Show the current variable's name, description, and also render any validation errors in real-time so the user knows what's wrong
	// with their input
	renderVariablePrompts(variable, invalidEntries)

	value, err := getUserInput(variable)
	if err != nil {
		return value, err
	}
	// If any of the variable's validation rules are not satisfied by the user's submission,
	// store the validation errors in a map. We'll then recursively call get_variable_from_user
	// again, this time passing in the validation errors map, so that we can render to the terminal
	// the exact issues with each submission
	validationMap, hasValidationErrs := validateUserInput(value, variable)
	if hasValidationErrs {
		ie := variables.InvalidEntries{
			Issues: []variables.ValidationIssue{
				{
					Value:         value,
					ValidationMap: validationMap,
				},
			},
		}

		return getVariableFromUser(l, variable, ie)
	}

	if value == "" {
		// TODO: what if the user wanted an empty string instead of the default?
		l.Debugf("Using default value for variable '%s': %v", variable.FullName(), variable.Default())
		return variable.Default(), nil
	}

	return value, nil
}

func getUserInput(variable variables.Variable) (string, error) {
	// Display rich prompts to the user, based on the type of variable we're asking for
	value := ""

	switch variable.Type() {
	case variables.String, variables.Int, variables.Float, variables.Bool, variables.List, variables.Map:
		msg := fmt.Sprintf("Enter a value [type %s]", variable.Type())
		if variable.Default() != nil {
			msg = fmt.Sprintf("%s (default: %v)", msg, variable.Default())
		}

		prompt := &survey.Input{
			Message: msg,
		}

		err := survey.AskOne(prompt, &value)
		if err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				log.Fatal("quit")
			}

			return value, err
		}
	case variables.Enum:
		prompt := &survey.Select{
			Message: "Please select " + variable.FullName(),
			Options: variable.Options(),
		}

		err := survey.AskOne(prompt, &value)
		if err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				log.Fatal("quit")
			}

			return value, err
		}
	default:
		if variable.Default() == nil {
			fmt.Println()

			msg := fmt.Sprintf("Variable %s of type '%s' does not support manual input and has no default value.\n"+
				"Please update the variable in the boilerplate.yml file to include a default value or provide a value via the command line using the --var option.",
				color.Green(variable.FullName()), variable.Type())
			log.Fatal(msg)
		}
	}

	return value, nil
}

func validateUserInput(value string, variable variables.Variable) (map[string]bool, bool) {
	var valueToValidate any
	if value == "" {
		valueToValidate = variable.Default()
	} else {
		valueToValidate = value
	}

	m := make(map[string]bool)
	hasValidationErrs := false

	for _, customValidation := range variable.Validations() {
		// Run the specific validation against the user-provided value and store it in the map
		err := ozzo.Validate(valueToValidate, customValidation.Validator)
		val := true

		if err != nil {
			hasValidationErrs = true
			val = false
		}

		m[customValidation.DescriptionText()] = val
	}
	// Validate that the type can be parsed
	if _, err := variables.ConvertType(valueToValidate, variable); err != nil {
		hasValidationErrs = true
		msg := fmt.Sprintf("Value must be of type %s: %s", variable.Type(), err)
		m[msg] = false
	}
	// Validate that the value is not empty if no default is provided
	if value == "" && variable.Default() == nil {
		hasValidationErrs = true
		m["Value must be provided"] = false
	}

	return m, hasValidationErrs
}

// renderValidationErrors displays in user-legible format the exact validation errors
// that the user's last submission generated.
func renderValidationErrors(val any, m map[string]bool) {
	fmt.Printf("%s %v\n", color.Yellow("WARNING: Invalid entry:"), val)

	for k, v := range m {
		if v {
			fmt.Printf("  %s %s\n", color.Green("PASS"), k)
		} else {
			fmt.Printf("  %s %s\n", color.Red("FAIL"), k)
		}
	}
}

func renderVariablePrompts(variable variables.Variable, invalidEntries variables.InvalidEntries) {
	fmt.Println(color.Green(variable.FullName()))

	if variable.Description() != "" {
		fmt.Println(color.Yellow(variable.Description()))
	}

	if len(invalidEntries.Issues) > 0 {
		renderValidationErrors(invalidEntries.Issues[0].Value, invalidEntries.Issues[0].ValidationMap)
	}
}

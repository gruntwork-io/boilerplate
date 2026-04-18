//go:build !(js && wasm)

package config

import (
	"errors"
	"fmt"
	"log"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/gruntwork-io/boilerplate/internal/color"
	"github.com/gruntwork-io/boilerplate/variables"
)

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

// Package options provides configuration options for the boilerplate tool.
package options

import (
	"fmt"
)

const OptTemplateURL = "template-url"
const OptOutputFolder = "output-folder"
const OptNonInteractive = "non-interactive"
const OptVar = "var"
const OptVarFile = "var-file"
const OptMissingKeyAction = "missing-key-action"
const OptMissingConfigAction = "missing-config-action"
const OptNoHooks = "no-hooks"
const OptNoShell = "no-shell"
const OptDisableDependencyPrompt = "disable-dependency-prompt"
const OptSilent = "silent"

// BoilerplateOptions represents the command-line options for the boilerplate app
type BoilerplateOptions struct {
	Vars                    map[string]any
	ShellCommandAnswers     map[string]bool
	TemplateURL             string
	TemplateFolder          string
	OutputFolder            string
	OnMissingKey            MissingKeyAction
	OnMissingConfig         MissingConfigAction
	NonInteractive          bool
	NoHooks                 bool
	NoShell                 bool
	DisableDependencyPrompt bool
	ExecuteAllShellCommands bool
	Silent                  bool
}

// MissingKeyAction is an enum that represents what we can do when a template looks up a missing key. This typically happens
// when there is a typo in the variable name in a template.
type MissingKeyAction string

var (
	Invalid       = MissingKeyAction("invalid") // print <no value> for any missing key
	ZeroValue     = MissingKeyAction("zero")    // print the zero value of the missing key
	ExitWithError = MissingKeyAction("error")   // exit with an error when there is a missing key
)

var AllMissingKeyActions = []MissingKeyAction{Invalid, ZeroValue, ExitWithError}
var DefaultMissingKeyAction = ExitWithError

// ParseMissingKeyAction converts the given string to a MissingKeyAction enum, or returns an error if this is not a valid value for the
// MissingKeyAction enum
func ParseMissingKeyAction(str string) (MissingKeyAction, error) {
	for _, missingKeyAction := range AllMissingKeyActions {
		if string(missingKeyAction) == str {
			return missingKeyAction, nil
		}
	}

	return MissingKeyAction(""), InvalidMissingKeyAction(str)
}

// MissingConfigAction is an enum that represents what to do when the template folder passed to boilerplate does not contain a
// boilerplate.yml file.
type MissingConfigAction string

var (
	Exit   = MissingConfigAction("exit")
	Ignore = MissingConfigAction("ignore")
)
var AllMissingConfigActions = []MissingConfigAction{Exit, Ignore}
var DefaultMissingConfigAction = Exit

// ParseMissingConfigAction converts the given string to a MissingConfigAction enum, or returns an error if this is not a valid value for the
// MissingConfigAction enum
func ParseMissingConfigAction(str string) (MissingConfigAction, error) {
	for _, missingConfigAction := range AllMissingConfigActions {
		if string(missingConfigAction) == str {
			return missingConfigAction, nil
		}
	}

	return MissingConfigAction(""), InvalidMissingConfigAction(str)
}

//nolint:staticcheck
var (
	ErrTemplateURLOptionCannotBeEmpty  = fmt.Errorf("The --%s option cannot be empty", OptTemplateURL)
	ErrOutputFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OptOutputFolder)
)

type InvalidMissingKeyAction string

func (err InvalidMissingKeyAction) Error() string {
	return fmt.Sprintf("Invalid MissingKeyAction '%s'. Value must be one of: %s", string(err), AllMissingKeyActions)
}

type InvalidMissingConfigAction string

func (err InvalidMissingConfigAction) Error() string {
	return fmt.Sprintf("Invalid MissingConfigAction '%s'. Value must be one of: %s", string(err), AllMissingConfigActions)
}

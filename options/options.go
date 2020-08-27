package options

import (
	"fmt"

	"github.com/urfave/cli"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

const OptTemplateFolder = "template-folder"
const OptOutputFolder = "output-folder"
const OptNonInteractive = "non-interactive"
const OptVar = "var"
const OptVarFile = "var-file"
const OptMissingKeyAction = "missing-key-action"
const OptMissingConfigAction = "missing-config-action"
const OptDisableHooks = "disable-hooks"
const OptDisableShell = "disable-shell"

// The command-line options for the boilerplate app
type BoilerplateOptions struct {
	TemplateFolder  string
	OutputFolder    string
	NonInteractive  bool
	Vars            map[string]interface{}
	OnMissingKey    MissingKeyAction
	OnMissingConfig MissingConfigAction
	DisableHooks    bool
	DisableShell    bool
}

// Validate that the options have reasonable values and return an error if they don't
func (options *BoilerplateOptions) Validate() error {
	if options.TemplateFolder == "" {
		return errors.WithStackTrace(TemplateFolderOptionCannotBeEmpty)
	}

	if !util.PathExists(options.TemplateFolder) {
		return errors.WithStackTrace(TemplateFolderDoesNotExist(options.TemplateFolder))
	}

	if options.OutputFolder == "" {
		return errors.WithStackTrace(OutputFolderOptionCannotBeEmpty)
	}

	return nil
}

// Parse the command line options provided by the user
func ParseOptions(cliContext *cli.Context) (*BoilerplateOptions, error) {
	vars, err := variables.ParseVars(cliContext.StringSlice(OptVar), cliContext.StringSlice(OptVarFile))
	if err != nil {
		return nil, err
	}

	missingKeyAction := DefaultMissingKeyAction
	missingKeyActionValue := cliContext.String(OptMissingKeyAction)
	if missingKeyActionValue != "" {
		missingKeyAction, err = ParseMissingKeyAction(missingKeyActionValue)
		if err != nil {
			return nil, err
		}
	}

	missingConfigAction := DefaultMissingConfigAction
	missingConfigActionValue := cliContext.String(OptMissingConfigAction)
	if missingConfigActionValue != "" {
		missingConfigAction, err = ParseMissingConfigAction(missingConfigActionValue)
		if err != nil {
			return nil, err
		}
	}

	options := &BoilerplateOptions{
		TemplateFolder:  cliContext.String(OptTemplateFolder),
		OutputFolder:    cliContext.String(OptOutputFolder),
		NonInteractive:  cliContext.Bool(OptNonInteractive),
		OnMissingKey:    missingKeyAction,
		OnMissingConfig: missingConfigAction,
		Vars:            vars,
		DisableHooks:    cliContext.Bool(OptDisableHooks),
		DisableShell:    cliContext.Bool(OptDisableShell),
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	return options, nil
}

// This type is an enum that represents what we can do when a template looks up a missing key. This typically happens
// when there is a typo in the variable name in a template.
type MissingKeyAction string

var (
	Invalid       = MissingKeyAction("invalid") // print <no value> for any missing key
	ZeroValue     = MissingKeyAction("zero")    // print the zero value of the missing key
	ExitWithError = MissingKeyAction("error")   // exit with an error when there is a missing key
)

var AllMissingKeyActions = []MissingKeyAction{Invalid, ZeroValue, ExitWithError}
var DefaultMissingKeyAction = ExitWithError

// Convert the given string to a MissingKeyAction enum, or return an error if this is not a valid value for the
// MissingKeyAction enum
func ParseMissingKeyAction(str string) (MissingKeyAction, error) {
	for _, missingKeyAction := range AllMissingKeyActions {
		if string(missingKeyAction) == str {
			return missingKeyAction, nil
		}
	}
	return MissingKeyAction(""), errors.WithStackTrace(InvalidMissingKeyAction(str))
}

// This type is an enum that represents what to do when the template folder passed to boilerplate does not contain a
// boilerplate.yml file.
type MissingConfigAction string

var (
	Exit   = MissingConfigAction("exit")
	Ignore = MissingConfigAction("ignore")
)
var AllMissingConfigActions = []MissingConfigAction{Exit, Ignore}
var DefaultMissingConfigAction = Exit

// Convert the given string to a MissingConfigAction enum, or return an error if this is not a valid value for the
// MissingConfigAction enum
func ParseMissingConfigAction(str string) (MissingConfigAction, error) {
	for _, missingConfigAction := range AllMissingConfigActions {
		if string(missingConfigAction) == str {
			return missingConfigAction, nil
		}
	}
	return MissingConfigAction(""), errors.WithStackTrace(InvalidMissingConfigAction(str))
}

// Custom error types

var TemplateFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OptTemplateFolder)
var OutputFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OptOutputFolder)

type TemplateFolderDoesNotExist string

func (err TemplateFolderDoesNotExist) Error() string {
	return fmt.Sprintf("Folder %s does not exist", string(err))
}

type InvalidMissingKeyAction string

func (err InvalidMissingKeyAction) Error() string {
	return fmt.Sprintf("Invalid MissingKeyAction '%s'. Value must be one of: %s", string(err), AllMissingKeyActions)
}

type InvalidMissingConfigAction string

func (err InvalidMissingConfigAction) Error() string {
	return fmt.Sprintf("Invalid MissingConfigAction '%s'. Value must be one of: %s", string(err), AllMissingConfigActions)
}

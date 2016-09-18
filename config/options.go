package config

import (
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/util"
	"fmt"
	"github.com/urfave/cli"
)

// The command-line options for the boilerplate app
type BoilerplateOptions struct {
	TemplateFolder 	 string
	OutputFolder 	 string
	NonInteractive	 bool
	Vars		 map[string]interface{}
	OnMissingKey     MissingKeyAction
	OnMissingConfig  MissingConfigAction
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
	vars, err := parseVars(cliContext.StringSlice(OPT_VAR), cliContext.StringSlice(OPT_VAR_FILE))
	if err != nil {
		return nil, err
	}

	missingKeyAction := DEFAULT_MISSING_KEY_ACTION
	missingKeyActionValue := cliContext.String(OPT_MISSING_KEY_ACTION)
	if missingKeyActionValue != "" {
		missingKeyAction, err = ParseMissingKeyAction(missingKeyActionValue)
		if err != nil {
			return nil, err
		}
	}

	missingConfigAction := DEFAULT_MISSING_CONFIG_ACTION
	missingConfigActionValue := cliContext.String(OPT_MISSING_CONFIG_ACTION)
	if missingConfigActionValue != "" {
		missingConfigAction, err = ParseMissingConfigAction(missingConfigActionValue)
		if err != nil {
			return nil, err
		}
	}

	options := &BoilerplateOptions{
		TemplateFolder: cliContext.String(OPT_TEMPLATE_FOLDER),
		OutputFolder: cliContext.String(OPT_OUTPUT_FOLDER),
		NonInteractive: cliContext.Bool(OPT_NON_INTERACTIVE),
		OnMissingKey: missingKeyAction,
		OnMissingConfig: missingConfigAction,
		Vars: vars,
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
	Invalid = MissingKeyAction("invalid")		// print <no value> for any missing key
	ZeroValue = MissingKeyAction("zero")		// print the zero value of the missing key
	ExitWithError = MissingKeyAction("error")	// exit with an error when there is a missing key
)

var ALL_MISSING_KEY_ACTIONS = []MissingKeyAction{Invalid, ZeroValue, ExitWithError}
var DEFAULT_MISSING_KEY_ACTION = ExitWithError

// Convert the given string to a MissingKeyAction enum, or return an error if this is not a valid value for the
// MissingKeyAction enum
func ParseMissingKeyAction(str string) (MissingKeyAction, error) {
	for _, missingKeyAction := range ALL_MISSING_KEY_ACTIONS {
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
	Exit = MissingConfigAction("exit")
	Ignore = MissingConfigAction("ignore")
)
var ALL_MISSING_CONFIG_ACTIONS = []MissingConfigAction{Exit, Ignore}
var DEFAULT_MISSING_CONFIG_ACTION = Exit

// Convert the given string to a MissingConfigAction enum, or return an error if this is not a valid value for the
// MissingConfigAction enum
func ParseMissingConfigAction(str string) (MissingConfigAction, error) {
	for _, missingConfigAction := range ALL_MISSING_CONFIG_ACTIONS {
		if string(missingConfigAction) == str {
			return missingConfigAction, nil
		}
	}
	return MissingConfigAction(""), errors.WithStackTrace(InvalidMissingConfigAction(str))
}

// Custom error types

var TemplateFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OPT_TEMPLATE_FOLDER)
var OutputFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OPT_OUTPUT_FOLDER)

type TemplateFolderDoesNotExist string
func (err TemplateFolderDoesNotExist) Error() string {
	return fmt.Sprintf("Folder %s does not exist", string(err))
}

type InvalidMissingKeyAction string
func (err InvalidMissingKeyAction) Error() string {
	return fmt.Sprintf("Invalid MissingKeyAction '%s'. Value must be one of: %s", string(err), ALL_MISSING_KEY_ACTIONS)
}

type InvalidMissingConfigAction string
func (err InvalidMissingConfigAction) Error() string {
	return fmt.Sprintf("Invalid MissingConfigAction '%s'. Value must be one of: %s", string(err), ALL_MISSING_CONFIG_ACTIONS)
}


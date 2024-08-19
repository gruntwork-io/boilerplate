package options

import (
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v2"

	"github.com/gruntwork-io/boilerplate/errors"
	getter_helper "github.com/gruntwork-io/boilerplate/getter-helper"
	"github.com/gruntwork-io/boilerplate/variables"
)

const OptTemplateUrl = "template-url"
const OptOutputFolder = "output-folder"
const OptNonInteractive = "non-interactive"
const OptVar = "var"
const OptVarFile = "var-file"
const OptMissingKeyAction = "missing-key-action"
const OptMissingConfigAction = "missing-config-action"
const OptDisableHooks = "disable-hooks"
const OptDisableShell = "disable-shell"
const OptDisableDependencyPrompt = "disable-dependency-prompt"
const OptSilent = "silent"

// The command-line options for the boilerplate app
type BoilerplateOptions struct {
	// go-getter supported URL where the template can be sourced.
	TemplateUrl string
	// Working directory where the go-getter defined template is downloaded.
	TemplateFolder string

	OutputFolder            string
	NonInteractive          bool
	Vars                    map[string]interface{}
	OnMissingKey            MissingKeyAction
	OnMissingConfig         MissingConfigAction
	DisableHooks            bool
	DisableShell            bool
	DisableDependencyPrompt bool
	Silent                  bool
	Logger                  *slog.Logger
}

// Validate that the options have reasonable values and return an error if they don't
func (options *BoilerplateOptions) Validate() error {
	if options.TemplateUrl == "" {
		return errors.WithStackTrace(TemplateUrlOptionCannotBeEmpty)
	}

	if err := getter_helper.ValidateTemplateUrl(options.TemplateUrl); err != nil {
		return err
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

	templateUrl, templateFolder, err := DetermineTemplateConfig(cliContext.String(OptTemplateUrl))
	if err != nil {
		return nil, err
	}

	options := &BoilerplateOptions{
		TemplateUrl:             templateUrl,
		TemplateFolder:          templateFolder,
		OutputFolder:            cliContext.String(OptOutputFolder),
		NonInteractive:          cliContext.Bool(OptNonInteractive),
		OnMissingKey:            missingKeyAction,
		OnMissingConfig:         missingConfigAction,
		Vars:                    vars,
		DisableHooks:            cliContext.Bool(OptDisableHooks),
		DisableShell:            cliContext.Bool(OptDisableShell),
		DisableDependencyPrompt: cliContext.Bool(OptDisableDependencyPrompt),
		Silent:                  cliContext.Bool(OptSilent),
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	return options, nil
}

// DetermineTemplateConfig decides what should be passed to TemplateUrl and TemplateFolder. This parses the templateUrl
// and determines if it is a local path. If so, use that path directly instead of downloading it to a temp working dir.
// We do this by setting the template folder, which will instruct the process routine to skip downloading the template.
// Returns TemplateUrl, TemplateFolder, error
func DetermineTemplateConfig(templateUrl string) (string, string, error) {
	url, err := getter_helper.ParseGetterUrl(templateUrl)
	if err != nil {
		return "", "", err
	}
	if url.Scheme == "file" {
		// Intentionally return as both TemplateUrl and TemplateFolder so that validation passes, but still skip
		// download.
		return templateUrl, templateUrl, nil
	}
	return templateUrl, "", nil
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

var TemplateUrlOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OptTemplateUrl)
var OutputFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OptOutputFolder)

type InvalidMissingKeyAction string

func (err InvalidMissingKeyAction) Error() string {
	return fmt.Sprintf("Invalid MissingKeyAction '%s'. Value must be one of: %s", string(err), AllMissingKeyActions)
}

type InvalidMissingConfigAction string

func (err InvalidMissingConfigAction) Error() string {
	return fmt.Sprintf("Invalid MissingConfigAction '%s'. Value must be one of: %s", string(err), AllMissingConfigActions)
}

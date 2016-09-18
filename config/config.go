package config

import (
	"io/ioutil"
	"path"
	"gopkg.in/yaml.v2"
	"github.com/gruntwork-io/boilerplate/util"
	"fmt"
	"github.com/gruntwork-io/boilerplate/errors"
	"strings"
)

const BOILERPLATE_CONFIG_FILE = "boilerplate.yml"

const OPT_TEMPLATE_FOLDER = "template-folder"
const OPT_OUTPUT_FOLDER = "output-folder"
const OPT_NON_INTERACTIVE = "non-interactive"
const OPT_VAR = "var"
const OPT_VAR_FILE = "var-file"
const OPT_MISSING_KEY_ACTION = "missing-key-action"
const OPT_MISSING_CONFIG_ACTION = "missing-config-action"

// The command-line options for the boilerplate app
type BoilerplateOptions struct {
	TemplateFolder 	 string
	OutputFolder 	 string
	NonInteractive	 bool
	Vars		 map[string]interface{}
	OnMissingKey     MissingKeyAction
	OnMissingConfig  MissingConfigAction
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

// The contents of a boilerplate.yml config file
type BoilerplateConfig struct {
	Variables    []Variable
	Dependencies []Dependency
}

// Given a unique variable name, return a tuple that contains the dependency name (if any) and the variable name.
// Variable and dependency names are split by a dot, so for "foo.bar", this will return ("foo", "bar"). For just "foo",
// it will return ("", "foo").
func SplitIntoDependencyNameAndVariableName(uniqueVariableName string) (string, string) {
	parts := strings.SplitAfterN(uniqueVariableName, ".", 2)
	if len(parts) == 2 {
		// The split method leaves the character you split on at the end of the string, so we have to trim it
		return strings.TrimSuffix(parts[0], "."), parts[1]
	} else {
		return "", parts[0]
	}
}

// A single boilerplate template that this boilerplate.yml depends on being executed first
type Dependency struct {
	Name                  string
	TemplateFolder        string `yaml:"template-folder"`
	OutputFolder          string `yaml:"output-folder"`
	DontInheritVariables  bool   `yaml:"dont-inherit-variables"`
	Variables             []Variable
}

// Return the default path for a boilerplate.yml config file in the given folder
func BoilerplateConfigPath(templateFolder string) string {
	return path.Join(templateFolder, BOILERPLATE_CONFIG_FILE)
}

// Load the boilerplate.yml config contents for the folder specified in the given options
func LoadBoilerplateConfig(options *BoilerplateOptions) (*BoilerplateConfig, error) {
	configPath := BoilerplateConfigPath(options.TemplateFolder)

	if util.PathExists(configPath) {
		util.Logger.Printf("Loading boilerplate config from %s", configPath)
		bytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		return ParseBoilerplateConfig(bytes)
	} else if options.OnMissingConfig == Ignore {
		util.Logger.Printf("Warning: boilerplate config file not found at %s. The %s flag is set, so ignoring. Note that no variables will be available while generating.", configPath, OPT_MISSING_CONFIG_ACTION)
		return &BoilerplateConfig{}, nil
	} else {
		return nil, BoilerplateConfigNotFound(configPath)
	}
}

// Parse the given configContents as a boilerplate.yml config file
func ParseBoilerplateConfig(configContents []byte) (*BoilerplateConfig, error) {
	boilerplateConfig := &BoilerplateConfig{}

	if err := yaml.Unmarshal(configContents, boilerplateConfig); err != nil {
		return nil, err
	}

	if err := boilerplateConfig.validate(); err != nil {
		return nil, err
	}

	return boilerplateConfig, nil
}

// Validate that the config file has reasonable contents and return an error if there is a problem
func (boilerplateConfig BoilerplateConfig) validate() error {
	if err := validateDependencies(boilerplateConfig.Dependencies); err != nil {
		return err
	}

	return nil
}

// Validate that the list of dependencies has reasonable contents and return an error if there is a problem
func validateDependencies(dependencies []Dependency) error {
	dependencyNames := []string{}
	for i, dependency := range dependencies {
		if dependency.Name == "" {
			return errors.WithStackTrace(MissingNameForDependency(i))
		}
		if util.ListContains(dependency.Name, dependencyNames) {
			return errors.WithStackTrace(DuplicateDependencyName(dependency.Name))
		}
		dependencyNames = append(dependencyNames, dependency.Name)

		if dependency.TemplateFolder == "" {
			return errors.WithStackTrace(MissingTemplateFolderForDependency(dependency.Name))
		}
		if dependency.OutputFolder == "" {
			return errors.WithStackTrace(MissingOutputFolderForDependency(dependency.Name))
		}
	}

	return nil
}

// Custom error types

var TemplateFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OPT_TEMPLATE_FOLDER)

var OutputFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OPT_OUTPUT_FOLDER)

type RequiredFieldMissing string
func (err RequiredFieldMissing) Error() string {
	return fmt.Sprintf("Variable is missing required field %s", string(err))
}

type VariableMissingOptions string
func (err VariableMissingOptions) Error() string {
	return fmt.Sprintf("Variable %s has type %s but does not specify any options. You must specify at least one option.", string(err), Enum)
}

type OptionsCanOnlyBeUsedWithEnum struct {
	VariableName string
	VariableType BoilerplateType
}
func (err OptionsCanOnlyBeUsedWithEnum) Error() string {
	return fmt.Sprintf("Variable %s has type %s and tries to specify options. Options may only be specified for variables of type %s.", err.VariableName, err.VariableType.String(), Enum)
}

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

type InvalidBoilerplateType string
func (err InvalidBoilerplateType) Error() string {
	return fmt.Sprintf("Invalid InvalidBoilerplateType '%s'. Value must be one of: %s", string(err), ALL_BOILERPLATE_TYPES)
}

type BoilerplateConfigNotFound string
func (err BoilerplateConfigNotFound) Error() string {
	return fmt.Sprintf("Could not find %s in %s and the %s flag is set to %s", BOILERPLATE_CONFIG_FILE, string(err), OPT_MISSING_CONFIG_ACTION, Exit)
}

type MissingNameForDependency int
func (index MissingNameForDependency) Error() string {
	return fmt.Sprintf("The name parameter was missing for dependency number %d", int(index) + 1)
}

type DuplicateDependencyName string
func (name DuplicateDependencyName) Error() string {
	return fmt.Sprintf("Found a duplicate dependency name: %s. All dependency names must be unique!", string(name))
}

type MissingTemplateFolderForDependency string
func (name MissingTemplateFolderForDependency) Error() string {
	return fmt.Sprintf("The %s parameter was missing for dependency %s", OPT_TEMPLATE_FOLDER, string(name))
}

type MissingOutputFolderForDependency string
func (name MissingOutputFolderForDependency) Error() string {
	return fmt.Sprintf("The %s parameter was missing for dependency %s", OPT_OUTPUT_FOLDER, string(name))
}
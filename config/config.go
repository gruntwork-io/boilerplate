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

// The command-line options for the boilerplate app
type BoilerplateOptions struct {
	TemplateFolder 	 string
	OutputFolder 	 string
	NonInteractive	 bool
	Vars		 map[string]string
	OnMissingKey     MissingKeyAction
}

// This type is an enum that represents what we can do when a template looks up a missing key. This typically happens
// when there is a typo in the variable name in a template.
type MissingKeyAction int

func (action MissingKeyAction) String() string {
	return missingKeyNames[int(action)]
}

// Convert the given string to a MissingKeyAction enum, or return an error if this is not a valid value for the
// MissingKeyAction enum
func ParseMissingKeyAction(keyName string) (MissingKeyAction, error) {
	for i, missingKeyName := range missingKeyNames {
		if missingKeyName == keyName {
			return MissingKeyAction(i), nil
		}
	}
	return MissingKeyAction(-1), errors.WithStackTrace(InvalidMissingKeyAction(keyName))
}

// The names of the missing keys. Go doesn't have enums, so we have to roll our own based on this example:
// https://gist.github.com/skarllot/102a5e5ea73861ff5afe
var missingKeyNames = []string{}

// Create a new MissingKeyAction enum with the given name
func newMissingKeyAction(name string) MissingKeyAction {
	missingKeyNames = append(missingKeyNames, name)
	return MissingKeyAction(len(missingKeyNames) - 1)
}

// Here are the MissingKeyAction enum values
var (
	Invalid = newMissingKeyAction("invalid")	// print <no value> for any missing key
	ZeroValue = newMissingKeyAction("zero")		// print the zero value of the missing key
	ExitWithError = newMissingKeyAction("error")	// exit with an error when there is a missing key
)

var ALL_MISSING_KEY_ACTIONS = []MissingKeyAction{Invalid, ZeroValue, ExitWithError}
var DEFAULT_MISSING_KEY_ACTION = ExitWithError

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

// A single variable defined in a boilerplate.yml config file
type Variable struct {
	Name 	      string
	Prompt 	      string
	Default	      string
	ForDependency string `yaml:"for-dependency"`
}

// Return the full, unique name of this variable, including the dependency it is for (if any). The dependency name and
// variable name will be separated by a dot (e.g. <DEPENDENCY_NAME>.<VARIABLE_NAME>).
func (variable Variable) UniqueName() string {
	if variable.ForDependency == "" {
		return variable.Name
	} else {
		return fmt.Sprintf("%s.%s", variable.ForDependency, variable.Name)
	}
}

// Return a description of this variable, which includes its name and the dependency it is for (if any) in a
// human-readable format
func (variable Variable) Description() string {
	if variable.ForDependency == "" {
		return variable.Name
	} else {
		return fmt.Sprintf("%s (for dependency %s)", variable.Name, variable.ForDependency)
	}
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
}

// Return the default path for a boilerplate.yml config file in the given folder
func BoilerPlateConfigPath(templateFolder string) string {
	return path.Join(templateFolder, BOILERPLATE_CONFIG_FILE)
}

// Load the boilerplate.yml config contents for the folder specified in the given options
func LoadBoilerPlateConfig(options *BoilerplateOptions) (*BoilerplateConfig, error) {
	configPath := BoilerPlateConfigPath(options.TemplateFolder)

	if util.PathExists(configPath) {
		util.Logger.Printf("Loading boilerplate config from %s", configPath)
		bytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		return ParseBoilerplateConfig(bytes)
	} else {
		util.Logger.Printf("Warning: boilerplate config file not found at %s. No variables will be available while generating.", configPath)
		return &BoilerplateConfig{}, nil
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
	for _, variable := range boilerplateConfig.Variables {
		if variable.Name == "" {
			return errors.WithStackTrace(VariableMissingName)
		}
	}

	dependencyNames := []string{}
	for i, dependency := range boilerplateConfig.Dependencies {
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

var VariableMissingName = fmt.Errorf("Error: found a variable without a name.")

var TemplateFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OPT_TEMPLATE_FOLDER)

var OutputFolderOptionCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OPT_OUTPUT_FOLDER)

type TemplateFolderDoesNotExist string
func (err TemplateFolderDoesNotExist) Error() string {
	return fmt.Sprintf("Folder %s does not exist", string(err))
}

type InvalidMissingKeyAction string
func (err InvalidMissingKeyAction) Error() string {
	return fmt.Sprintf("Invalid MissingKeyAction '%s'. Value must be one of: %s", string(err), missingKeyNames)
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
package config

import (
	"io/ioutil"
	"path"
	"gopkg.in/yaml.v2"
	"github.com/gruntwork-io/boilerplate/util"
	"fmt"
	"github.com/gruntwork-io/boilerplate/errors"
)

const BOILERPLATE_CONFIG_FILE = "boilerplate.yml"

const OPT_TEMPLATE_FOLDER = "template-folder"
const OPT_OUTPUT_FOLDER = "output-folder"
const OPT_NON_INTERACTIVE = "non-interactive"
const OPT_VAR = "var"

// The command-line options for the boilerplate app
type BoilerplateOptions struct {
	TemplateFolder 	string
	OutputFolder 	string
	NonInteractive	bool
	Vars		map[string]string
}

// Validate that the options have reasonable values and return an error if they don't
func (options *BoilerplateOptions) Validate() error {
	if options.TemplateFolder == "" {
		return errors.WithStackTrace(TemplateFolderCannotBeEmpty)
	}

	if !util.PathExists(options.TemplateFolder) {
		return errors.WithStackTrace(TemplateFolderDoesNotExist(options.TemplateFolder))
	}

	if options.OutputFolder == "" {
		return errors.WithStackTrace(OutputFolderCannotBeEmpty)
	}

	return nil
}

// The contents of a boilerplate.yml config file
type BoilerplateConfig struct {
	Variables []Variable
}

// A single variable defined in a boilerplate.yml config file
type Variable struct {
	Name 	string
	Prompt 	string
	Default	string
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

	return nil
}

// Custom error types

var VariableMissingName = fmt.Errorf("Error: found a variable without a name.")

var TemplateFolderCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OPT_TEMPLATE_FOLDER)

var OutputFolderCannotBeEmpty = fmt.Errorf("The --%s option cannot be empty", OPT_OUTPUT_FOLDER)

type TemplateFolderDoesNotExist string
func (err TemplateFolderDoesNotExist) Error() string {
	return fmt.Sprintf("Folder %s does not exist", err)
}


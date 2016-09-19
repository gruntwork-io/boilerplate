package config

import (
	"io/ioutil"
	"path"
	"gopkg.in/yaml.v2"
	"github.com/gruntwork-io/boilerplate/util"
	"fmt"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/variables"
)

const BOILERPLATE_CONFIG_FILE = "boilerplate.yml"

const OPT_TEMPLATE_FOLDER = "template-folder"
const OPT_OUTPUT_FOLDER = "output-folder"
const OPT_NON_INTERACTIVE = "non-interactive"
const OPT_VAR = "var"
const OPT_VAR_FILE = "var-file"
const OPT_MISSING_KEY_ACTION = "missing-key-action"
const OPT_MISSING_CONFIG_ACTION = "missing-config-action"

// The contents of a boilerplate.yml config file
type BoilerplateConfig struct {
	Variables    []variables.Variable
	Dependencies []variables.Dependency
}

// Implement the go-yaml unmarshal interface for BoilerplateConfig. We can't let go-yaml handle this itself because:
//
// 1. Variable is an interface
// 2. We need to provide Defaults for optional fields, such as "type"
// 3. We want to validate the variable as part of the unmarshalling process so we never have invalid Variable or
//    Dependency classes floating around
func (config *BoilerplateConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var fields map[string]interface{}
	if err := unmarshal(&fields); err != nil {
		return err
	}

	vars, err := variables.UnmarshalVariablesFromBoilerplateConfigYaml(fields)
	if err != nil {
		return err
	}

	deps, err := variables.UnmarshalDependenciesFromBoilerplateConfigYaml(fields)
	if err != nil {
		return err
	}

	*config = BoilerplateConfig{
		Variables: vars,
		Dependencies: deps,
	}
	return nil
}

// Load the boilerplate.yml config contents for the folder specified in the given options
func LoadBoilerplateConfig(options *BoilerplateOptions) (*BoilerplateConfig, error) {
	configPath := BoilerplateConfigPath(options.TemplateFolder)

	if util.PathExists(configPath) {
		util.Logger.Printf("Loading boilerplate config from %s", configPath)
		bytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}

		return ParseBoilerplateConfig(bytes)
	} else if options.OnMissingConfig == Ignore {
		util.Logger.Printf("Warning: boilerplate config file not found at %s. The %s flag is set, so ignoring. Note that no variables will be available while generating.", configPath, OPT_MISSING_CONFIG_ACTION)
		return &BoilerplateConfig{}, nil
	} else {
		return nil, errors.WithStackTrace(BoilerplateConfigNotFound(configPath))
	}
}

// Parse the given configContents as a boilerplate.yml config file
func ParseBoilerplateConfig(configContents []byte) (*BoilerplateConfig, error) {
	boilerplateConfig := &BoilerplateConfig{}

	if err := yaml.Unmarshal(configContents, boilerplateConfig); err != nil {
		return nil, errors.WithStackTrace(err)
	}

	return boilerplateConfig, nil
}

// Return the default path for a boilerplate.yml config file in the given folder
func BoilerplateConfigPath(templateFolder string) string {
	return path.Join(templateFolder, BOILERPLATE_CONFIG_FILE)
}

// Custom error types

type BoilerplateConfigNotFound string
func (err BoilerplateConfigNotFound) Error() string {
	return fmt.Sprintf("Could not find %s in %s and the %s flag is set to %s", BOILERPLATE_CONFIG_FILE, string(err), OPT_MISSING_CONFIG_ACTION, Exit)
}
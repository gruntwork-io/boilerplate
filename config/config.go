package config

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"strings"

	"github.com/gruntwork-io/go-commons/version"
	goversion "github.com/hashicorp/go-version"
	"gopkg.in/yaml.v2"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

const BOILERPLATE_CONFIG_FILE = "boilerplate.yml"

// The contents of a boilerplate.yml config file
type BoilerplateConfig struct {
	RequiredVersion *string
	Variables       []variables.Variable
	Dependencies    []variables.Dependency
	Hooks           variables.Hooks
	Partials        []string
	SkipFiles       []variables.SkipFile
	Engines         []variables.Engine
}

// GetVariablesMap returns a map that maps variable names to the variable config.
func (config *BoilerplateConfig) GetVariablesMap() map[string]variables.Variable {
	out := make(map[string]variables.Variable)
	for _, variable := range config.Variables {
		out[variable.Name()] = variable
	}
	return out
}

// Implement the go-yaml unmarshal interface for BoilerplateConfig. We can't let go-yaml handle this itself because:
//
//  1. Variable is an interface
//  2. We need to provide Defaults for optional fields, such as "type"
//  3. We want to validate the variable as part of the unmarshalling process so we never have invalid Variable or
//     Dependency classes floating around
func (config *BoilerplateConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var fields map[string]interface{}
	if err := unmarshal(&fields); err != nil {
		return err
	}

	requiredVersion, err := variables.UnmarshalString(fields, "required_version", false)
	if err != nil {
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

	hooks, err := variables.UnmarshalHooksFromBoilerplateConfigYaml(fields)
	if err != nil {
		return err
	}

	partials, err := variables.UnmarshalListOfStrings(fields, "partials")
	if err != nil {
		return err
	}

	skipFiles, err := variables.UnmarshalSkipFilesFromBoilerplateConfigYaml(fields)
	if err != nil {
		return err
	}

	engines, err := variables.UnmarshalEnginesFromBoilerplateConfigYaml(fields)
	if err != nil {
		return err
	}

	*config = BoilerplateConfig{
		RequiredVersion: requiredVersion,
		Variables:       vars,
		Dependencies:    deps,
		Hooks:           hooks,
		Partials:        partials,
		SkipFiles:       skipFiles,
		Engines:         engines,
	}
	return nil
}

// Implement the go-yaml marshaler interface so that the config can be marshaled into yaml. We use a custom marshaler
// instead of defining the fields as tags so that we skip the attributes that are empty.
func (config *BoilerplateConfig) MarshalYAML() (interface{}, error) {
	configYml := map[string]interface{}{}
	if len(config.Variables) > 0 {
		// Due to go type system, we can only pass through []interface{}, even though []Variable is technically
		// polymorphic to that type. So we reconstruct the list using the right type before passing it in to the marshal
		// function.
		interfaceList := []interface{}{}
		for _, variable := range config.Variables {
			interfaceList = append(interfaceList, variable)
		}
		varsYml, err := util.MarshalListOfObjectsToYAML(interfaceList)
		if err != nil {
			return nil, err
		}
		configYml["variables"] = varsYml
	}
	if len(config.Dependencies) > 0 {
		// Due to go type system, we can only pass through []interface{}, even though []Dependency is technically
		// polymorphic to that type. So we reconstruct the list using the right type before passing it in to the marshal
		// function.
		interfaceList := []interface{}{}
		for _, dep := range config.Dependencies {
			interfaceList = append(interfaceList, dep)
		}
		depsYml, err := util.MarshalListOfObjectsToYAML(interfaceList)
		if err != nil {
			return nil, err
		}
		configYml["dependencies"] = depsYml
	}
	if len(config.Hooks.BeforeHooks) > 0 || len(config.Hooks.AfterHooks) > 0 {
		hooksYml, err := config.Hooks.MarshalYAML()
		if err != nil {
			return nil, err
		}
		configYml["hooks"] = hooksYml
	}
	if len(config.Partials) > 0 {
		configYml["partials"] = config.Partials
	}
	if len(config.SkipFiles) > 0 {
		// Due to go type system, we can only pass through []interface{}, even though []SkipFile is technically
		// polymorphic to that type. So we reconstruct the list using the right type before passing it in to the marshal
		// function.
		interfaceList := []interface{}{}
		for _, skipFile := range config.SkipFiles {
			interfaceList = append(interfaceList, skipFile)
		}
		skipFilesYml, err := util.MarshalListOfObjectsToYAML(interfaceList)
		if err != nil {
			return nil, err
		}
		configYml["skip_files"] = skipFilesYml
	}
	if len(config.Engines) > 0 {
		// Due to go type system, we can only pass through []interface{}, even though []Engine is technically
		// polymorphic to that type. So we reconstruct the list using the right type before passing it in to the marshal
		// function.
		interfaceList := []interface{}{}
		for _, engine := range config.Engines {
			interfaceList = append(interfaceList, engine)
		}
		enginesYml, err := util.MarshalListOfObjectsToYAML(interfaceList)
		if err != nil {
			return nil, err
		}
		configYml["engines"] = enginesYml
	}
	return configYml, nil
}

// Load the boilerplate.yml config contents for the folder specified in the given options
func LoadBoilerplateConfig(opts *options.BoilerplateOptions) (*BoilerplateConfig, error) {
	configPath := BoilerplateConfigPath(opts.TemplateFolder)

	if util.PathExists(configPath) {
		opts.Logger.Info(fmt.Sprintf("Loading boilerplate config from %s", configPath))
		bytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}

		return ParseBoilerplateConfig(bytes)
	} else if opts.OnMissingConfig == options.Ignore {
		opts.Logger.Info(fmt.Sprintf("Warning: boilerplate config file not found at %s. The %s flag is set, so ignoring. Note that no variables will be available while generating.", configPath, options.OptMissingConfigAction))
		return &BoilerplateConfig{}, nil
	} else {
		// If the template URL is similar to a git URL, surface in error message that there may be a misspelling/typo.
		return nil, errors.WithStackTrace(BoilerplateConfigNotFound(configPath))
	}
}

// Parse the given configContents as a boilerplate.yml config file
func ParseBoilerplateConfig(configContents []byte) (*BoilerplateConfig, error) {
	boilerplateConfig := &BoilerplateConfig{}

	if err := yaml.Unmarshal(configContents, boilerplateConfig); err != nil {
		return nil, errors.WithStackTrace(err)
	}

	converted, err := variables.ConvertYAMLToStringMap(boilerplateConfig)
	if err != nil {
		return boilerplateConfig, err
	}

	boilerplateConfig, ok := converted.(*BoilerplateConfig)
	if !ok {
		return nil, variables.YAMLConversionErr{Key: converted}
	}

	return boilerplateConfig, nil
}

// Return the default path for a boilerplate.yml config file in the given folder
func BoilerplateConfigPath(templateFolder string) string {
	return path.Join(templateFolder, BOILERPLATE_CONFIG_FILE)
}

// EnforceRequiredVersion enforces any required_version string that is configured on the boilerplate config by checking
// against the current version of the CLI.
func EnforceRequiredVersion(boilerplateConfig *BoilerplateConfig) error {
	// Base case: if required_version is not set, then there is no version to enforce.
	if boilerplateConfig == nil || boilerplateConfig.RequiredVersion == nil {
		return nil
	}
	constraint := *boilerplateConfig.RequiredVersion

	// Base case: if using a development version, then bypass required version check
	currentVersion := version.GetVersion()
	if currentVersion == "" {
		return nil
	}

	// At this point there is a valid version that needs to be checked against the constraint
	boilerplateVersion, err := goversion.NewVersion(currentVersion)
	if err != nil {
		return errors.WithStackTrace(err)
	}
	versionConstraint, err := goversion.NewConstraint(constraint)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	if !versionConstraint.Check(boilerplateVersion) {
		return errors.WithStackTrace(InvalidBoilerplateVersion{CurrentVersion: boilerplateVersion, VersionConstraints: versionConstraint})
	}
	return nil
}

// maybeGitURL uses heuristics to attempt to decide if the URL may be a github URL that is encoded incorrectly.
func maybeGitURL(templateURL string) bool {
	potentialGitURLs := []string{
		"github.com",
		"gitlab.com",
		"bitbucket.org",
	}
	for _, url := range potentialGitURLs {
		if strings.Contains(templateURL, url) {
			return true
		}
	}

	// If the URL can be parsed and any non-file URL part is parsed out, return that this may be a git URL.
	parsed, err := url.Parse(templateURL)
	if err != nil {
		return false
	}
	return parsed.Scheme != "" || parsed.Hostname() != "" || parsed.RawQuery != ""
}

// Custom error types

type BoilerplateConfigNotFound string

func (err BoilerplateConfigNotFound) Error() string {
	errMsg := fmt.Sprintf("Could not find %s in %s and the %s flag is set to %s", BOILERPLATE_CONFIG_FILE, string(err), options.OptMissingConfigAction, options.Exit)

	configPath := string(err)
	if maybeGitURL(configPath) {
		errMsg += ". Template URL looks like a git repo. Did you misspell the URL? Should be encoded as one of the following: `git::ssh://git@github.com/ORG/REPO`, `github.com/ORG/REPO`, `https://github.com/ORG/REPO`, or `git@github.com:ORG/REPO`."
	}

	return errMsg
}

type InvalidBoilerplateVersion struct {
	CurrentVersion     *goversion.Version
	VersionConstraints goversion.Constraints
}

func (err InvalidBoilerplateVersion) Error() string {
	return fmt.Sprintf("The currently installed version of Boilerplate (%s) is not compatible with the version constraint requiring (%s).", err.CurrentVersion.String(), err.VersionConstraints.String())
}

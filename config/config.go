package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/gruntwork-io/boilerplate/errors"
	getter_helper "github.com/gruntwork-io/boilerplate/getter-helper"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

const boilerplateConfigFile = "boilerplate.yml"

// The contents of a boilerplate.yml config file
type BoilerplateConfig struct {
	Variables    []variables.Variable
	Dependencies []variables.Dependency
	Hooks        variables.Hooks
	Includes     variables.Includes
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

	includes, err := variables.UnmarshalIncludesFromBoilerplateConfigYaml(fields)
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

	*config = BoilerplateConfig{
		Variables:    vars,
		Dependencies: deps,
		Hooks:        hooks,
		Includes:     includes,
	}
	return nil
}

// Load the boilerplate.yml config contents for the folder specified in the given options
func LoadBoilerplateConfig(opts *options.BoilerplateOptions) (*BoilerplateConfig, error) {
	configPath := ""
	if opts.IncludeFile != "" {
		// Included files are absolute file paths rather than directories with boilerplate.yml files
		configPath = opts.IncludeFile
	} else {
		configPath = BoilerplateConfigPath(opts.TemplateFolder)
	}

	if util.PathExists(configPath) {
		util.Logger.Printf("Loading boilerplate config from %s", configPath)
		bytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}

		boilerplateConfig, err := ParseBoilerplateConfig(bytes)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}

		// Reverse the order of the BeforeIncludes so that the usage is more intuitive.
		// If we didn't reverse, the last item of the "before" list would be the first item included, but intuitively,
		// one would expect it to be the last item. This happens because in the before case, the items are prepended to
		// the list, not appended, and hence each included template prepends to the previous one.
		beforeIncludes := util.ReverseStringSlice(boilerplateConfig.Includes.BeforeIncludes)

		if err = mergeIncludes(boilerplateConfig, opts, beforeIncludes, mergeBeforeIncludes); err != nil {
			return nil, errors.WithStackTrace(err)
		}

		if err = mergeIncludes(boilerplateConfig, opts, boilerplateConfig.Includes.AfterIncludes, mergeAfterIncludes); err != nil {
			return nil, errors.WithStackTrace(err)
		}

		return boilerplateConfig, nil
	} else if opts.OnMissingConfig == options.Ignore {
		util.Logger.Printf("Warning: boilerplate config file not found at %s. The %s flag is set, so ignoring. Note that no variables will be available while generating.", configPath, options.OptMissingConfigAction)
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
	return path.Join(templateFolder, boilerplateConfigFile)
}

// getIncludeFilePath returns the working directory and the absolute path to an included file, downloading the file first
// if the include is a URL. The caller must clean up the working directory.
func getIncludeFilePath(include, templateFolder string) (string, string, error) {
	includeURL, includeFolder, err := options.DetermineTemplateConfig(include)
	if includeFolder == "" {
		workingDir, includeFile, err := getter_helper.DownloadTemplatesToTemporaryFolder(includeURL)
		if err != nil {
			return "", "", err
		}
		return workingDir, includeFile, nil
	}

	absPath, err := filepath.Abs(path.Join(templateFolder, include))
	if err != nil {
		return "", "", errors.WithStackTrace(err)
	}
	return "", absPath, nil
}

// mergeIncludes merges a list of included files with the original configuration
func mergeIncludes(dstConfig *BoilerplateConfig, opts *options.BoilerplateOptions, includes []string, mergeInclude func(*BoilerplateConfig, *BoilerplateConfig)) error {
	for _, include := range includes {
		workingDir, includeFilePath, err := getIncludeFilePath(include, opts.TemplateFolder)
		defer func() {
			util.Logger.Printf("Cleaning up working directory.")
			os.RemoveAll(workingDir)
		}()
		if err != nil {
			return err
		}

		if includeFilePath == opts.IncludeFile {
			err := fmt.Errorf("Included file %s includes itself!", includeFilePath)
			return err
		}

		opts := &options.BoilerplateOptions{
			TemplateFolder: path.Dir(includeFilePath),
			IncludeFile:    includeFilePath,
		}
		srcConfig, err := LoadBoilerplateConfig(opts)
		if err != nil {
			return err
		}

		mergeInclude(srcConfig, dstConfig)
	}
	return nil
}

// mergeBeforeIncludes prepends the merge configuration to the original configuration
func mergeBeforeIncludes(src *BoilerplateConfig, dst *BoilerplateConfig) {
	dst.Variables = append(src.Variables, dst.Variables...)
	dst.Dependencies = append(src.Dependencies, dst.Dependencies...)
	dst.Hooks.BeforeHooks = append(src.Hooks.BeforeHooks, dst.Hooks.BeforeHooks...)
	dst.Hooks.AfterHooks = append(src.Hooks.AfterHooks, dst.Hooks.AfterHooks...)
}

// mergeAfterIncludes appends the merge configuration to the original configuration
func mergeAfterIncludes(src *BoilerplateConfig, dst *BoilerplateConfig) {
	dst.Variables = append(dst.Variables, src.Variables...)
	dst.Dependencies = append(dst.Dependencies, src.Dependencies...)
	dst.Hooks.BeforeHooks = append(dst.Hooks.BeforeHooks, src.Hooks.BeforeHooks...)
	dst.Hooks.AfterHooks = append(dst.Hooks.AfterHooks, src.Hooks.AfterHooks...)
}

// Custom error types

type BoilerplateConfigNotFound string

func (err BoilerplateConfigNotFound) Error() string {
	return fmt.Sprintf("Could not find %s in %s and the %s flag is set to %s", boilerplateConfigFile, string(err), options.OptMissingConfigAction, options.Exit)
}

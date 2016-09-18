package config

import (
	"strings"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/util"
	"fmt"
)

// A single boilerplate template that this boilerplate.yml depends on being executed first
type Dependency struct {
	Name                  string
	TemplateFolder        string `yaml:"template-folder"`
	OutputFolder          string `yaml:"output-folder"`
	DontInheritVariables  bool   `yaml:"dont-inherit-variables"`
	Variables             []Variable
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
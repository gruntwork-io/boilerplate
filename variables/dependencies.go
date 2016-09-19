package variables

import (
	"strings"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/util"
	"fmt"
)

// A single boilerplate template that this boilerplate.yml depends on being executed first
type Dependency struct {
	Name                  string
	TemplateFolder        string
	OutputFolder          string
	DontInheritVariables  bool
	Variables             []Variable
}

func (dependency Dependency) GetNamespacedVariables() []Variable {
	variables := []Variable{}

	for _, variable := range dependency.Variables {
		variableNameForDependency := fmt.Sprintf("%s.%s", dependency.Name, variable.Name())
		variables = append(variables, variable.WithName(variableNameForDependency))
	}

	return variables
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

func UnmarshalDependencies(fields map[string]interface{}, fieldName string) ([]Dependency, error) {
	unmarshalledDependencies := []Dependency{}
	dependencyNames := []string{}

	listOfFields, err := unmarshalListOfFields(fields, fieldName)
	if err != nil {
		return unmarshalledDependencies, err
	}

	for _, fields := range listOfFields {
		dependency, err := UnmarshalDependency(fields)
		if err != nil {
			return unmarshalledDependencies, err
		}

		if util.ListContains(dependency.Name, dependencyNames) {
			return unmarshalledDependencies, errors.WithStackTrace(DuplicateDependencyName(dependency.Name))
		}
		dependencyNames = append(dependencyNames, dependency.Name)

		unmarshalledDependencies = append(unmarshalledDependencies, *dependency)
	}

	return unmarshalledDependencies, nil
}

func UnmarshalDependency(fields map[string]interface{}) (*Dependency, error) {
	name, err := unmarshalStringField(fields, "name", true, "")
	if err != nil {
		return nil, err
	}

	templateFolder, err := unmarshalStringField(fields, "template-folder", true, *name)
	if err != nil {
		return nil, err
	}

	outputFolder, err := unmarshalStringField(fields, "output-folder", true, *name)
	if err != nil {
		return nil, err
	}

	dontInheritVariables, err := unmarshalBooleanField(fields, "dont-inherit-variables", false, *name)
	if err != nil {
		return nil, err
	}

	variables, err := UnmarshalVariables(fields, "variables")
	if err != nil {
		return nil, err
	}

	return &Dependency{
		Name: *name,
		TemplateFolder: *templateFolder,
		OutputFolder: *outputFolder,
		DontInheritVariables: dontInheritVariables,
		Variables: variables,
	}, nil
}

// Custom error types

type DuplicateDependencyName string
func (name DuplicateDependencyName) Error() string {
	return fmt.Sprintf("Found a duplicate dependency name: %s. All dependency names must be unique!", string(name))
}

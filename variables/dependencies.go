// Package variables provides functionality for handling template variables and dependencies.
package variables

import (
	"fmt"
	"strings"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/util"
)

// Dependency represents a single boilerplate template that this boilerplate.yml depends on being executed first
type Dependency struct {
	Name                 string
	TemplateURL          string
	OutputFolder         string
	Skip                 string
	ForEachReference     string
	Variables            []Variable
	VarFiles             []string
	ForEach              []string
	DontInheritVariables bool
}

// TemplateUrl provides backward compatibility for accessing TemplateURL with lowercase "url"
//
//nolint:staticcheck
func (d Dependency) TemplateUrl() string {
	return d.TemplateURL
}

// MarshalYAML implements the go-yaml marshaler interface so that the config can be marshaled into yaml. We use a custom marshaler
// instead of defining the fields as tags so that we skip the attributes that are empty.
func (d Dependency) MarshalYAML() (any, error) {
	depYml := map[string]any{}
	if d.Name != "" {
		depYml["name"] = d.Name
	}

	if d.TemplateURL != "" {
		depYml["template-url"] = d.TemplateURL
	}

	if d.OutputFolder != "" {
		depYml["output-folder"] = d.OutputFolder
	}

	if d.Skip != "" {
		depYml["skip"] = d.Skip
	}

	if len(d.Variables) > 0 {
		// Due to go type system, we can only pass through []interface{}, even though []Variable is technically
		// polymorphic to that type. So we reconstruct the list using the right type before passing it in to the marshal
		// function.
		interfaceList := []any{}
		for _, variable := range d.Variables {
			interfaceList = append(interfaceList, variable)
		}

		varsYml, err := util.MarshalListOfObjectsToYAML(interfaceList)
		if err != nil {
			return nil, err
		}

		depYml["variables"] = varsYml
	}

	if len(d.VarFiles) > 0 {
		depYml["var_files"] = d.VarFiles
	}

	if len(d.ForEach) > 0 {
		depYml["for_each"] = d.ForEach
	}

	if len(d.ForEachReference) > 0 {
		depYml["for_each_reference"] = d.ForEachReference
	}

	return depYml, nil
}

// SplitIntoDependencyNameAndVariableName given a unique variable name, returns a tuple that contains the dependency name (if any) and the variable name.
// Variable and dependency names are split by a dot, so for "foo.bar", this will return ("foo", "bar"). For just "foo",
// it will return ("", "foo").
func SplitIntoDependencyNameAndVariableName(uniqueVariableName string) (string, string) {
	const maxSplitParts = 2

	parts := strings.SplitAfterN(uniqueVariableName, ".", maxSplitParts)
	if len(parts) == maxSplitParts {
		// The split method leaves the character you split on at the end of the string, so we have to trim it
		return strings.TrimSuffix(parts[0], "."), parts[1]
	} else {
		return "", parts[0]
	}
}

// UnmarshalDependenciesFromBoilerplateConfigYaml given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// dependencies:
//
//   - name: <NAME>
//     template-url: <TEMPLATE_URL>
//     output-folder: <OUTPUT_FOLDER>
//
//   - name: <NAME>
//     template-url: <TEMPLATE_URL>
//     output-folder: <OUTPUT_FOLDER>
//
// This method takes the data above and unmarshals it into a list of Dependency objects
func UnmarshalDependenciesFromBoilerplateConfigYaml(fields map[string]any) ([]Dependency, error) {
	unmarshalledDependencies := []Dependency{}
	dependencyNames := []string{}

	listOfFields, err := unmarshalListOfFields(fields, "dependencies")
	if err != nil {
		return unmarshalledDependencies, err
	}

	for _, fields := range listOfFields {
		dependency, err := UnmarshalDependencyFromBoilerplateConfigYaml(fields)
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

// UnmarshalDependencyFromBoilerplateConfigYaml given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// name: <NAME>
// template-url: <TEMPLATE_URL>
// output-folder: <OUTPUT_FOLDER>
//
// This method takes the data above and unmarshals it into a Dependency object
func UnmarshalDependencyFromBoilerplateConfigYaml(fields map[string]any) (*Dependency, error) {
	name, err := unmarshalStringField(fields, "name", true, "")
	if err != nil {
		return nil, err
	}

	templateURL, err := unmarshalStringField(fields, "template-url", true, *name)
	if err != nil {
		return nil, err
	}

	outputFolder, err := unmarshalStringField(fields, "output-folder", true, *name)
	if err != nil {
		return nil, err
	}

	skipPtr, err := unmarshalStringField(fields, "skip", false, *name)
	if err != nil {
		return nil, err
	}

	var skip string
	if skipPtr != nil {
		skip = *skipPtr
	}

	dontInheritVariables, err := unmarshalBooleanField(fields, "dont-inherit-variables", false, *name)
	if err != nil {
		return nil, err
	}

	variables, err := UnmarshalVariablesFromBoilerplateConfigYaml(fields)
	if err != nil {
		return nil, err
	}

	varFiles, err := UnmarshalListOfStrings(fields, "var_files")
	if err != nil {
		return nil, err
	}

	forEach, err := UnmarshalListOfStrings(fields, "for_each")
	if err != nil {
		return nil, err
	}

	forEachReferencePtr, err := UnmarshalString(fields, "for_each_reference", false)
	if err != nil {
		return nil, err
	}

	forEachReference := ""
	if forEachReferencePtr != nil {
		forEachReference = *forEachReferencePtr
	}

	return &Dependency{
		Name:                 *name,
		TemplateURL:          *templateURL,
		OutputFolder:         *outputFolder,
		Skip:                 skip,
		DontInheritVariables: dontInheritVariables,
		Variables:            variables,
		VarFiles:             varFiles,
		ForEach:              forEach,
		ForEachReference:     forEachReference,
	}, nil
}

// Custom error types

type DuplicateDependencyName string

func (name DuplicateDependencyName) Error() string {
	return fmt.Sprintf("Found a duplicate dependency name: %s. All dependency names must be unique!", string(name))
}

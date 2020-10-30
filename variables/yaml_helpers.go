package variables

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/util"
	"gopkg.in/yaml.v2"
)

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName:
//   key1: value1
//   key2: value2
//   key3: value3
//
// This method looks up the given fieldName in the map and unmarshals the data inside of it it into a map of the
// key:value pairs.
func unmarshalMapOfFields(fields map[string]interface{}, fieldName string) (map[string]interface{}, error) {
	fieldAsYaml, containsField := fields[fieldName]
	if !containsField || fieldAsYaml == nil {
		return nil, nil
	}

	asYamlMap, isYamlMap := fieldAsYaml.(map[interface{}]interface{})
	if !isYamlMap {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "map[string]interface{}", ActualType: reflect.TypeOf(fieldAsYaml)})
	}

	stringMap := map[string]interface{}{}
	for key, value := range asYamlMap {
		if keyAsString, isString := key.(string); isString {
			stringMap[keyAsString] = value
		} else {
			return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeOf(key)})
		}
	}

	return stringMap, nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName:
//   key1: value1
//   key2: value2
//   key3: value3
//
// This method looks up the given fieldName in the map and unmarshals the data inside of it it into a map of the
// key:value pairs, where both the keys and values are strings.
func unmarshalMapOfStrings(fields map[string]interface{}, fieldName string) (map[string]string, error) {
	rawMap, err := unmarshalMapOfFields(fields, fieldName)
	if err != nil {
		return nil, err
	}
	if len(rawMap) == 0 {
		return nil, nil
	}

	stringMap := map[string]string{}

	for key, value := range rawMap {
		stringMap[key] = fmt.Sprintf("%v", value)
	}

	return stringMap, nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName:
//   - value1
//   - value2
//   - value3
//
// This method takes looks up the given fieldName in the map and unmarshals the data inside of it it into a list of
// strings with the given values.
func UnmarshalListOfStrings(fields map[string]interface{}, fieldName string) ([]string, error) {
	fieldAsYaml, containsField := fields[fieldName]
	if !containsField || fieldAsYaml == nil {
		return nil, nil
	}

	asYamlList, isYamlList := fieldAsYaml.([]interface{})
	if !isYamlList {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "[]interface{}", ActualType: reflect.TypeOf(fieldAsYaml)})
	}

	listOfStrings := []string{}

	for _, asYaml := range asYamlList {
		if valueAsString, isString := asYaml.(string); isString {
			listOfStrings = append(listOfStrings, valueAsString)
		} else {
			return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeOf(asYamlList)})
		}
	}

	return listOfStrings, nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName:
//   - key1: value1
//     key2: value2
//     key3: value3
//
//   - key1: value1
//     key2: value2
//     key3: value3
//
// This method takes looks up the given fieldName in the map and unmarshals the data inside of it it into a list of
// maps, where each map contains the set of key:value pairs
func unmarshalListOfFields(fields map[string]interface{}, fieldName string) ([]map[string]interface{}, error) {
	fieldAsYaml, containsField := fields[fieldName]
	if !containsField || fieldAsYaml == nil {
		return nil, nil
	}

	asYamlList, isYamlList := fieldAsYaml.([]interface{})
	if !isYamlList {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "[]interface{}", ActualType: reflect.TypeOf(fieldAsYaml)})
	}

	listOfFields := []map[string]interface{}{}

	for _, asYaml := range asYamlList {
		asYamlMap, isYamlMap := asYaml.(map[interface{}]interface{})
		if !isYamlMap {
			return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "map[string]interface{}", ActualType: reflect.TypeOf(asYaml)})
		}

		listOfFields = append(listOfFields, util.ToStringToGenericMap(asYamlMap))
	}

	return listOfFields, nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// options:
//   - foo
//   - bar
//   - baz
//
// This method takes looks up the options object in the map and unmarshals the data inside of it it into a list of
// strings. This is meant to be used to parse the options field of an Enum variable. If the given variableType is not
// an Enum and options have been specified, or this variable is an Enum and options have not been specified, this
// method will return an error.
func unmarshalOptionsField(fields map[string]interface{}, context string, variableType BoilerplateType) ([]string, error) {
	options, hasOptions := fields["options"]

	if !hasOptions {
		if variableType == Enum {
			return nil, errors.WithStackTrace(OptionsMissing(context))
		} else {
			return nil, nil
		}
	}

	if variableType != Enum {
		return nil, errors.WithStackTrace(OptionsCanOnlyBeUsedWithEnum{Context: context, Type: variableType})
	}

	optionsAsList, isList := options.([]interface{})
	if !isList {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: "options", ExpectedType: "List", ActualType: reflect.TypeOf(options), Context: context})
	}

	return util.ToStringList(optionsAsList), nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// type: <TYPE>
//
// This method takes looks up the options key in the map and unmarshals the data inside of it it into a
// BoilerplateType. If no type is specified, this method returns the default type (String). If an unrecognized type is
// specified, this method returns an error.
func unmarshalTypeField(fields map[string]interface{}, context string) (BoilerplateType, error) {
	variableTypeAsString, err := unmarshalStringField(fields, "type", false, context)
	if err != nil {
		return BOILERPLATE_TYPE_DEFAULT, err
	}

	if variableTypeAsString != nil {
		variableType, err := ParseBoilerplateType(*variableTypeAsString)
		if err != nil {
			return BOILERPLATE_TYPE_DEFAULT, err
		}
		return *variableType, nil
	}

	return BOILERPLATE_TYPE_DEFAULT, nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName: <VALUE>
//
// This method takes looks up the given fieldName in the map and unmarshals the data inside of it into a string. If
// requiredField is true and fieldName was not in the map, this method will return an error.
func unmarshalStringField(fields map[string]interface{}, fieldName string, requiredField bool, context string) (*string, error) {
	value, hasValue := fields[fieldName]
	if !hasValue {
		if requiredField {
			return nil, errors.WithStackTrace(RequiredFieldMissing(fieldName))
		} else {
			return nil, nil
		}
	}

	if valueAsString, isString := value.(string); isString {
		return &valueAsString, nil
	} else {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeOf(value), Context: context})
	}
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName: <VALUE>
//
// This method takes looks up the given fieldName in the map and unmarshals the data inside of it into a bool. If
// requiredField is true and fieldName was not in the map, this method will return an error.
func unmarshalBooleanField(fields map[string]interface{}, fieldName string, requiredField bool, context string) (bool, error) {
	value, hasValue := fields[fieldName]
	if !hasValue {
		if requiredField {
			return false, errors.WithStackTrace(RequiredFieldMissing(fieldName))
		} else {
			return false, nil
		}
	}

	if valueAsBool, isBool := value.(bool); isBool {
		return valueAsBool, nil
	} else {
		return false, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "bool", ActualType: reflect.TypeOf(value), Context: context})
	}
}

// Parse a list of NAME=VALUE pairs passed in as command-line options into a map of variable names to variable values.
// Along the way, each value is parsed as YAML.
func parseVariablesFromKeyValuePairs(varsList []string) (map[string]interface{}, error) {
	vars := map[string]interface{}{}

	for _, variable := range varsList {
		variableParts := strings.Split(variable, "=")
		if len(variableParts) != 2 {
			return vars, errors.WithStackTrace(InvalidVarSyntax(variable))
		}

		key := variableParts[0]
		value := variableParts[1]
		if key == "" {
			return vars, errors.WithStackTrace(VariableNameCannotBeEmpty(variable))
		}

		parsedValue, err := ParseYamlString(value)
		if err != nil {
			return vars, err
		}

		vars[key] = parsedValue
	}

	return vars, nil
}

// Parse a YAML string into a Go type
func ParseYamlString(str string) (interface{}, error) {
	var parsedValue interface{}

	err := yaml.Unmarshal([]byte(str), &parsedValue)
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	return Convert(parsedValue), nil
}

// Parse a list of YAML files that define variables into a map from variable name to variable value. Along the way,
// each value is parsed as YAML.
func parseVariablesFromVarFiles(varFileList []string) (map[string]interface{}, error) {
	vars := map[string]interface{}{}

	for _, varFile := range varFileList {
		varsInFile, err := ParseVariablesFromVarFile(varFile)
		if err != nil {
			return vars, err
		}
		vars = util.MergeMaps(vars, varsInFile)
	}

	return vars, nil
}

// Parse the variables in the given YAML file into a map of variable name to variable value. Along the way, each value
// is parsed as YAML.
func ParseVariablesFromVarFile(varFilePath string) (map[string]interface{}, error) {
	bytes, err := ioutil.ReadFile(varFilePath)
	if err != nil {
		return map[string]interface{}{}, errors.WithStackTrace(err)
	}
	return parseVariablesFromVarFileContents(bytes)
}

// Parse the variables in the given YAML contents into a map of variable name to variable value. Along the way, each
// value is parsed as YAML.
func parseVariablesFromVarFileContents(varFileContents []byte) (map[string]interface{}, error) {
	vars := make(map[string]interface{})

	if err := yaml.Unmarshal(varFileContents, &vars); err != nil {
		return vars, err
	}

	return Convert(vars).(map[string]interface{}), nil
}

// Parse variables passed in via command line options, either as a list of NAME=VALUE variable pairs in varsList, or a
// list of paths to YAML files that define NAME: VALUE pairs. Return a map of the NAME: VALUE pairs. Along the way,
// each VALUE is parsed as YAML.
func ParseVars(varsList []string, varFileList []string) (map[string]interface{}, error) {
	variables := map[string]interface{}{}

	varsFromVarsList, err := parseVariablesFromKeyValuePairs(varsList)
	if err != nil {
		return variables, err
	}

	varsFromVarFiles, err := parseVariablesFromVarFiles(varFileList)
	if err != nil {
		return variables, err
	}

	return util.MergeMaps(varsFromVarsList, varsFromVarFiles), nil
}

// Convert updates i of type map[interface{}]interface{} to a map[string]interface{} so that it may be properly
// marshalled in to JSON.
// See: https://github.com/go-yaml/yaml/issues/139
func Convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = Convert(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = Convert(v)
		}
	}
	return i
}

// Custom error types

type OptionsMissing string

func (err OptionsMissing) Error() string {
	return fmt.Sprintf("%s has type %s but does not specify any options. You must specify at least one option.", string(err), Enum)
}

type InvalidVariableValue struct {
	Value    interface{}
	Variable Variable
}

func (err InvalidVariableValue) Error() string {
	message := fmt.Sprintf("Value '%v' is not a valid value for variable '%s' with type '%s'.", err.Value, err.Variable.Name(), err.Variable.Type().String())
	if err.Variable.Type() == Enum {
		message = fmt.Sprintf("%s. Value must be one of: %s.", message, err.Variable.Options())
	}
	return message
}

type OptionsCanOnlyBeUsedWithEnum struct {
	Context string
	Type    BoilerplateType
}

func (err OptionsCanOnlyBeUsedWithEnum) Error() string {
	return fmt.Sprintf("%s has type %s and tries to specify options. Options may only be specified for the %s type.", err.Context, err.Type.String(), Enum)
}

type InvalidTypeForField struct {
	FieldName    string
	ExpectedType string
	ActualType   reflect.Type
	Context      string
}

func (err InvalidTypeForField) Error() string {
	message := fmt.Sprintf("Field %s in %s must have type %s but got %s", err.FieldName, err.Context, err.ExpectedType, err.ActualType)
	return message
}

type VariableNameCannotBeEmpty string

func (varSyntax VariableNameCannotBeEmpty) Error() string {
	return fmt.Sprintf("Variable name cannot be empty. Expected NAME=VALUE but got %s", string(varSyntax))
}

type InvalidVarSyntax string

func (varSyntax InvalidVarSyntax) Error() string {
	return fmt.Sprintf("Invalid syntax for variable. Expected NAME=VALUE but got %s", string(varSyntax))
}

type RequiredFieldMissing string

func (err RequiredFieldMissing) Error() string {
	return fmt.Sprintf("Required field %s is missing", string(err))
}

type UnrecognizedBoilerplateType BoilerplateType

func (err UnrecognizedBoilerplateType) Error() string {
	return fmt.Sprintf("Unrecognized type: %s", BoilerplateType(err).String())
}

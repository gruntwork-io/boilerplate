package variables

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/gruntwork-io/boilerplate/errors"
)

// An interface for a variable defined in a boilerplate.yml config file
type Variable interface {
	// The name of the variable
	Name() string

	// The full name of this variable, which includes its name and the dependency it is for (if any) in a
	// human-readable format
	FullName() string

	// The description of the variable, if any
	Description() string

	// The type of the variable
	Type() BoilerplateType

	// The user-defined sorting position of the variable
	Order() int

	// The default value for the variable, if any
	Default() interface{}

	// The name of another variable from which this variable should take its value
	Reference() string

	// The values this variable can take. Applies only if Type() is Enum.
	Options() []string

	// Return a copy of this variable but with the name set to the given name
	WithName(string) Variable

	// Return a copy of this variable but with the description set to the given description
	WithDescription(string) Variable

	// Return a copy of this variable but with the default set to the given value
	WithDefault(interface{}) Variable

	// Create a human-readable, string representation of the variable
	String() string

	// Show an example value that would be valid for this variable
	ExampleValue() string

	// A custom marshaling function to translate back to YAML, as expected by go-yaml.
	MarshalYAML() (interface{}, error)

	// Validations that should be run on the variable
	Validations() []CustomValidationRule
}

// A private implementation of the Variable interface that forces all users to use our public constructors
type defaultVariable struct {
	name         string
	description  string
	defaultValue interface{}
	reference    string
	variableType BoilerplateType
	order        int
	options      []string
	validations  []CustomValidationRule
}

// Create a new variable that holds a string
func NewStringVariable(name string) Variable {
	return defaultVariable{
		name:         name,
		variableType: String,
	}
}

// Create a new variable that holds an int
func NewIntVariable(name string) Variable {
	return defaultVariable{
		name:         name,
		variableType: Int,
	}
}

// Create a new variable that holds a float
func NewFloatVariable(name string) Variable {
	return defaultVariable{
		name:         name,
		variableType: Float,
	}
}

// Create a new variable that holds a bool
func NewBoolVariable(name string) Variable {
	return defaultVariable{
		name:         name,
		variableType: Bool,
	}
}

// Create a new variable that holds a list of strings
func NewListVariable(name string) Variable {
	return defaultVariable{
		name:         name,
		variableType: List,
	}
}

// Create a new variable that holds a map of string to string
func NewMapVariable(name string) Variable {
	return defaultVariable{
		name:         name,
		variableType: Map,
	}
}

// Create a new variable that holds an enum with the given possible values
func NewEnumVariable(name string, options []string) Variable {
	return defaultVariable{
		name:         name,
		variableType: Enum,
		options:      options,
	}
}

func (variable defaultVariable) Name() string {
	return variable.name
}

func (variable defaultVariable) FullName() string {
	dependencyName, variableName := SplitIntoDependencyNameAndVariableName(variable.Name())
	if dependencyName == "" {
		return variableName
	} else {
		return fmt.Sprintf("%s (for dependency %s)", variableName, dependencyName)
	}
}

func (variable defaultVariable) Description() string {
	return variable.description
}

func (variable defaultVariable) Type() BoilerplateType {
	return variable.variableType
}

func (variable defaultVariable) Order() int {
	return variable.order
}

func (variable defaultVariable) Default() interface{} {
	return variable.defaultValue
}

func (variable defaultVariable) Reference() string {
	return variable.reference
}

func (variable defaultVariable) Options() []string {
	return variable.options
}

func (variable defaultVariable) Validations() []CustomValidationRule {
	return variable.validations
}

func (variable defaultVariable) WithName(name string) Variable {
	variable.name = name
	return variable
}

func (variable defaultVariable) WithDescription(description string) Variable {
	variable.description = description
	return variable
}

func (variable defaultVariable) WithDefault(value interface{}) Variable {
	variable.defaultValue = value
	return variable
}

func (variable defaultVariable) String() string {
	return fmt.Sprintf("Variable {Name: '%s', Description: '%s', Type: '%v', Default: '%v', Options: '%v', Reference: '%v'}", variable.Name(), variable.Description(), variable.Type(), variable.Default(), variable.Options(), variable.Reference())
}

func (variable defaultVariable) ExampleValue() string {
	switch variable.Type() {
	case String:
		return "foo"
	case Int:
		return "42"
	case Float:
		return "3.1415926"
	case Bool:
		return "true or false"
	case List:
		return "[foo, bar, baz]"
	case Map:
		return "{foo: bar, baz: blah}"
	case Enum:
		return fmt.Sprintf("must be one of: %s", variable.Options())
	default:
		return ""
	}
}

// Define a custom marshaler for YAML so that variables (and thus any struct using it) can be marshaled into YAML.
func (variable defaultVariable) MarshalYAML() (interface{}, error) {
	varYml := map[string]interface{}{}
	// We avoid a straight assignment to ensure that only fields that are actually set are rendered out.
	if variable.Name() != "" {
		varYml["name"] = variable.Name()
	}
	if variable.Description() != "" {
		varYml["description"] = variable.Description()
	}
	if variable.Type() != "" {
		varYml["type"] = variable.Type()
	}
	if variable.Default() != nil {
		varYml["default"] = variable.Default()
	}
	if variable.Reference() != "" {
		varYml["reference"] = variable.Reference()
	}
	if len(variable.Options()) > 0 {
		varYml["options"] = variable.Options()
	}
	if len(variable.Validations()) > 0 {
		varYml["validations"] = variable.Validations()
	}
	return varYml, nil
}

// Check that the given value matches the type we're expecting in the given variable and return an error if it doesn't
func ConvertType(value interface{}, variable Variable) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	asString, isString := value.(string)

	switch variable.Type() {
	case String:
		if isString {
			return asString, nil
		}
	case Int:
		if asInt, isInt := value.(int); isInt {
			return asInt, nil
		}
		if isString {
			return strconv.Atoi(asString)
		}
	case Float:
		if asFloat, isFloat := value.(float64); isFloat {
			return asFloat, nil
		}
		if isString {
			return strconv.ParseFloat(asString, 64)
		}
	case Bool:
		if asBool, isBool := value.(bool); isBool {
			return asBool, nil
		}
		if isString {
			return strconv.ParseBool(asString)
		}
	case List:
		if reflect.TypeOf(value).Kind() == reflect.Slice {
			return value, nil
		}
		if isString {
			return parseStringAsList(asString)
		}
	case Map:
		if reflect.TypeOf(value).Kind() == reflect.Map {
			value, err := ConvertYAMLToStringMap(value)
			if err != nil {
				return nil, err
			}
			return value, nil
		}
		if isString {
			return parseStringAsMap(asString)
		}
	case Enum:
		if isString {
			for _, option := range variable.Options() {
				if asString == option {
					return asString, nil
				}
			}
		}
	}

	return nil, InvalidVariableValue{Variable: variable, Value: value}
}

var GO_LIST_SYNTAX_REGEX = regexp.MustCompile(`\[(.*)]`)
var GO_MAP_SYNTAX_REGEX = regexp.MustCompile(`map\[(.*)]`)

// This method converts a string to a list. The string can either be a valid JSON list or the string output of a Go
// list.
func parseStringAsList(str string) ([]string, error) {
	jsonOut, jsonErr := parseStringAsJsonList(str)
	if jsonErr == nil {
		return jsonOut, nil
	}

	goOut, goErr := parseStringAsGoList(str)
	if goErr == nil {
		return goOut, nil
	}

	return nil, errors.WithStackTrace(FormatNotJsonOrGo{
		ExpectedJsonFormat: `["value1", "value2", "value3"]`,
		ExpectedGoFormat:   `[value1 value2 value3]`,
		ActualFormat:       str,
		JsonErr:            jsonErr,
		GoErr:              goErr,
	})
}

// If you render a list in Go, it'll have the format [<value> <value> <value>]. This method parses this format back
// into a Go list. This allows us to use Golang template syntax in variable values and still have the rendered value
// converted back to the proper type rather than a string.
//
// Note that this is a bit of a hack and should generally not be used, as it's not possible to unambiguously parse
// lists in Go that had spaces in the values.
func parseStringAsGoList(str string) ([]string, error) {
	matches := GO_LIST_SYNTAX_REGEX.FindStringSubmatch(str)

	if len(matches) != 2 {
		return nil, errors.WithStackTrace(ParseError{ExpectedType: "list", ExpectedFormat: "[<value> <value> <value>]", ActualFormat: str})
	}

	items := strings.TrimSpace(matches[1])

	if len(items) == 0 {
		return []string{}, nil
	}

	return strings.Split(items, " "), nil
}

// Parse a string as a JSON list
func parseStringAsJsonList(str string) ([]string, error) {
	var out []string

	if err := json.Unmarshal([]byte(str), &out); err != nil {
		return nil, errors.WithStackTrace(err)
	}

	return out, nil
}

// This method converts a string to a map. The string can either be a valid JSON map or the string output of a Go map.
func parseStringAsMap(str string) (map[string]string, error) {
	jsonOut, jsonErr := parseStringAsJsonMap(str)
	if jsonErr == nil {
		return jsonOut, nil
	}

	goOut, goErr := parseStringAsGoMap(str)
	if goErr == nil {
		return goOut, nil
	}

	return nil, errors.WithStackTrace(FormatNotJsonOrGo{
		ExpectedJsonFormat: `{"key1": "value1", "key2": "value2", "key3": "value3"}`,
		ExpectedGoFormat:   `map[key1:value1 key2:value2 key3:value3]`,
		ActualFormat:       str,
		JsonErr:            jsonErr,
		GoErr:              goErr,
	})
}

// If you render a map in Go, it'll have the format map[<key>:<value> <key>:<value> <key>:<value>]. This method parses
// this format back into a Go map. This allows us to use Golang template syntax in variable values and still have the
// rendered value converted back to the proper type rather than a string.
//
// Note that this is a bit of a hack and should generally not be used, as it's not possible to unambiguously parse
// maps in Go that had spaces in the keys or values.
func parseStringAsGoMap(str string) (map[string]string, error) {
	matches := GO_MAP_SYNTAX_REGEX.FindStringSubmatch(str)

	if len(matches) != 2 {
		return nil, errors.WithStackTrace(ParseError{ExpectedType: "map", ExpectedFormat: "[<key>:<value> <key>:<value> <key>:<value>]", ActualFormat: str})
	}

	items := strings.TrimSpace(matches[1])

	if len(items) == 0 {
		return map[string]string{}, nil
	}

	keysAndValues := strings.Split(items, " ")
	result := map[string]string{}

	for _, keyAndValue := range keysAndValues {
		parts := strings.Split(keyAndValue, ":")
		if len(parts) < 2 {
			return nil, errors.WithStackTrace(ParseError{ExpectedType: "map", ExpectedFormat: "<key>:<value> for each item in the map", ActualFormat: str})
		}

		key := strings.Join(parts[:(len(parts)-1)], ":")
		value := parts[len(parts)-1]

		result[key] = value
	}

	return result, nil
}

// Parse a string as a JSON map
func parseStringAsJsonMap(str string) (map[string]string, error) {
	var out map[string]string

	if err := json.Unmarshal([]byte(str), &out); err != nil {
		return nil, errors.WithStackTrace(err)
	}

	return out, nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// variables:
//   - name: <NAME>
//     description: <DESCRIPTION>
//     type: <TYPE>
//
//   - name: <NAME>
//     description: <DESCRIPTION>
//     default: <DEFAULT>
//
// This method takes the data above and unmarshals it into a list of Variable objects
func UnmarshalVariablesFromBoilerplateConfigYaml(fields map[string]interface{}) ([]Variable, error) {
	unmarshalledVariables := []Variable{}

	listOfFields, err := unmarshalListOfFields(fields, "variables")
	if err != nil {
		return unmarshalledVariables, err
	}

	for _, fields := range listOfFields {
		variable, err := UnmarshalVariableFromBoilerplateConfigYaml(fields)
		if err != nil {
			return unmarshalledVariables, err
		}
		unmarshalledVariables = append(unmarshalledVariables, variable)
	}

	return unmarshalledVariables, nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// name: <NAME>
// description: <DESCRIPTION>
// type: <TYPE>
// default: <DEFAULT>
//
// This method takes the data above and unmarshals it into a Variable object
func UnmarshalVariableFromBoilerplateConfigYaml(fields map[string]interface{}) (Variable, error) {
	variable := defaultVariable{}

	name, err := unmarshalStringField(fields, "name", true, "")
	if err != nil {
		return nil, err
	}
	variable.name = *name

	variableType, err := unmarshalTypeField(fields, *name)
	if err != nil {
		return nil, err
	}
	variable.variableType = variableType

	order, err := unmarshalIntField(fields, *name, false)
	if err != nil {
		return nil, err
	}
	variable.order = order

	description, err := unmarshalStringField(fields, "description", false, *name)
	if err != nil {
		return nil, err
	}
	if description != nil {
		variable.description = *description
	}

	reference, err := unmarshalStringField(fields, "reference", false, *name)
	if err != nil {
		return nil, err
	}
	if reference != nil {
		variable.reference = *reference
	}

	options, err := unmarshalOptionsField(fields, *name, variableType)
	if err != nil {
		return nil, err
	}
	variable.options = options

	validationRules, err := unmarshalValidationsField(fields, *name, variableType)
	if err != nil {
		return nil, err
	}
	variable.validations = validationRules

	variable.defaultValue = fields["default"]

	return variable, nil
}

// Custom error types

type ParseError struct {
	ExpectedType   string
	ExpectedFormat string
	ActualFormat   string
}

func (err ParseError) Error() string {
	return fmt.Sprintf("Expected type '%s' with format '%s', but got format '%s'.", err.ExpectedType, err.ExpectedFormat, err.ActualFormat)
}

type FormatNotJsonOrGo struct {
	ExpectedJsonFormat string
	ExpectedGoFormat   string
	ActualFormat       string
	JsonErr            error
	GoErr              error
}

func (err FormatNotJsonOrGo) Error() string {
	return fmt.Sprintf("Expected a string in JSON format (e.g., %s) or Go format (e.g., %s), but got: %s. JSON parsing error: %v. Go parsing error: %v.", err.ExpectedJsonFormat, err.ExpectedGoFormat, err.ActualFormat, err.JsonErr, err.GoErr)
}

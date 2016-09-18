package config

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"reflect"
	"github.com/gruntwork-io/boilerplate/errors"
	"gopkg.in/yaml.v2"
)

func TestGetVariableFromVarsEmptyVars(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo"}
	options := &BoilerplateOptions{}

	_, containsValue := getVariableFromVars(variable, options)
	assert.False(t, containsValue)
}

func TestGetVariableFromVarsNoMatch(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo"}
	options := &BoilerplateOptions{
		Vars: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	_, containsValue := getVariableFromVars(variable, options)
	assert.False(t, containsValue)
}

func TestGetVariableFromVarsMatch(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo"}
	options := &BoilerplateOptions{
		Vars: map[string]interface{}{
			"key1": "value1",
			"foo": "bar",
			"key3": "value3",
		},
	}

	actual, containsValue := getVariableFromVars(variable, options)
	expected := "bar"

	assert.True(t, containsValue)
	assert.Equal(t, expected, actual)
}

func TestGetVariableFromVarsForDependencyNoMatch(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "bar.foo"}
	options := &BoilerplateOptions{
		Vars: map[string]interface{}{
			"key1": "value1",
			"foo": "bar",
			"key3": "value3",
		},
	}

	_, containsValue := getVariableFromVars(variable, options)
	assert.False(t, containsValue)
}

func TestGetVariableFromVarsForDependencyMatch(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "bar.foo"}
	options := &BoilerplateOptions{
		Vars: map[string]interface{}{
			"key1": "value1",
			"bar.foo": "bar",
			"key3": "value3",
		},
	}

	actual, containsValue := getVariableFromVars(variable, options)
	expected := "bar"

	assert.True(t, containsValue)
	assert.Equal(t, expected, actual)
}

func TestGetVariableNoMatchNonInteractive(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo"}
	options := &BoilerplateOptions{NonInteractive: true}

	_, err := getVariable(variable, options)

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, MissingVariableWithNonInteractiveMode("foo")), "Expected a MissingVariableWithNonInteractiveMode error but got %s", reflect.TypeOf(err))
}

func TestGetVariableInVarsNonInteractive(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo"}
	options := &BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]interface{}{
			"key1": "value1",
			"foo": "bar",
			"key3": "value3",
		},
	}

	actual, err := getVariable(variable, options)
	expected := "bar"

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariableDefaultNonInteractive(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo", Default: "bar"}
	options := &BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	actual, err := getVariable(variable, options)
	expected := "bar"

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesNoVariables(t *testing.T) {
	t.Parallel()

	options := &BoilerplateOptions{NonInteractive: true}
	boilerplateConfig := &BoilerplateConfig{}

	actual, err := GetVariables(options, boilerplateConfig)
	expected := map[string]interface{}{}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesNoMatchNonInteractive(t *testing.T) {
	t.Parallel()

	options := &BoilerplateOptions{NonInteractive: true}
	boilerplateConfig := &BoilerplateConfig{
		Variables: []Variable{
			{Name: "foo", Type: String},
		},
	}

	_, err := GetVariables(options, boilerplateConfig)

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, MissingVariableWithNonInteractiveMode("foo")), "Expected a MissingVariableWithNonInteractiveMode error but got %s", reflect.TypeOf(err))
}

func TestGetVariablesMatchFromVars(t *testing.T) {
	t.Parallel()

	options := &BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]interface{}{
			"foo": "bar",
		},
	}

	boilerplateConfig := &BoilerplateConfig{
		Variables: []Variable{
			{Name: "foo", Type: String},
		},
	}

	actual, err := GetVariables(options, boilerplateConfig)
	expected := map[string]interface{}{
		"foo": "bar",
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesMatchFromVarsAndDefaults(t *testing.T) {
	t.Parallel()

	options := &BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}

	boilerplateConfig := &BoilerplateConfig{
		Variables: []Variable{
			{Name: "key1", Type: String},
			{Name: "key2", Type: String},
			{Name: "key3", Type: String, Default: "value3"},
		},
	}

	actual, err := GetVariables(options, boilerplateConfig)
	expected := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestParseVariablesFromKeyValuePairs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		keyValuePairs []string
		expectedError error
		expectedVars  map[string]interface{}
	}{
		{[]string{}, nil, map[string]interface{}{}},
		{[]string{"key=value"}, nil, map[string]interface{}{"key": "value"}},
		{[]string{"key="}, nil, map[string]interface{}{"key": nil}},
		{[]string{"key1=value1", "key2=value2", "key3=value3"}, nil, map[string]interface{}{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{[]string{"invalidsyntax"}, InvalidVarSyntax("invalidsyntax"), map[string]interface{}{}},
		{[]string{"="}, VariableNameCannotBeEmpty("="), map[string]interface{}{}},
		{[]string{"=foo"}, VariableNameCannotBeEmpty("=foo"), map[string]interface{}{}},
	}

	for _, testCase := range testCases {
		actualVars, err := ParseVariablesFromKeyValuePairs(testCase.keyValuePairs)
		if testCase.expectedError == nil {
			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedVars, actualVars)
		} else {
			assert.NotNil(t, err)
			assert.True(t, errors.IsError(err, testCase.expectedError), "Expected an error of type '%s' with value '%s' but got an error of type '%s' with value '%s'", reflect.TypeOf(testCase.expectedError), testCase.expectedError.Error(), reflect.TypeOf(err), err.Error())
		}
	}
}

const YAML_FILE_ONE_VAR =
`
key: value
`

const YAML_FILE_MULTIPLE_VARS =
`
key1: value1
key2: value2
key3: value3
`

func TestParseVariablesFromVarFileContents(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		fileContents  	    string
		expectYamlTypeError bool
		expectedVars        map[string]interface{}
	}{
		{"", false, map[string]interface{}{}},
		{YAML_FILE_ONE_VAR, false, map[string]interface{}{"key": "value"}},
		{YAML_FILE_MULTIPLE_VARS, false, map[string]interface{}{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{"invalid yaml", true, map[string]interface{}{}},
	}

	for _, testCase := range testCases {
		actualVars, err := parseVariablesFromVarFileContents([]byte(testCase.fileContents))
		if testCase.expectYamlTypeError {
			assert.NotNil(t, err)
			unwrapped := errors.Unwrap(err)
			_, isYamlTypeError := unwrapped.(*yaml.TypeError)
			assert.True(t, isYamlTypeError, "Expected a YAML type error for an invalid yaml file but got %s", reflect.TypeOf(unwrapped))
		} else {
			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedVars, actualVars)
		}
	}
}
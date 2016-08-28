package config

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"reflect"
	"github.com/gruntwork-io/boilerplate/errors"
	"gopkg.in/yaml.v2"
)

func TestFormatPrompt(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo"}
	options := &BoilerplateOptions{}

	actual := formatPrompt(variable, options)
	expected := "Enter a value for variable 'foo'"

	assert.Equal(t, expected, actual)
}

func TestFormatPromptWithDefault(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo", Default: "bar"}
	options := &BoilerplateOptions{}

	actual := formatPrompt(variable, options)
	expected := "Enter a value for variable 'foo' (default: 'bar')"

	assert.Equal(t, expected, actual)
}

func TestFormatPromptWithCustomPrompt(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo", Prompt: "custom"}
	options := &BoilerplateOptions{}

	actual := formatPrompt(variable, options)
	expected := "custom"

	assert.Equal(t, expected, actual)
}

func TestFormatPromptWithCustomPromptAndDefault(t *testing.T) {
	t.Parallel()

	variable := Variable{Name: "foo", Prompt: "custom", Default: "bar"}
	options := &BoilerplateOptions{}

	actual := formatPrompt(variable, options)
	expected := "custom (default: 'bar')"

	assert.Equal(t, expected, actual)
}

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
		Vars: map[string]string{
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
		Vars: map[string]string{
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
		Vars: map[string]string{
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
		Vars: map[string]string{
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

	actual, err := GetVariables(options, boilerplateConfig, map[string]string{})
	expected := map[string]string{}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesNoMatchNonInteractive(t *testing.T) {
	t.Parallel()

	options := &BoilerplateOptions{NonInteractive: true}
	boilerplateConfig := &BoilerplateConfig{
		Variables: []Variable{
			Variable{Name: "foo"},
		},
	}

	_, err := GetVariables(options, boilerplateConfig, map[string]string{})

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, MissingVariableWithNonInteractiveMode("foo")), "Expected a MissingVariableWithNonInteractiveMode error but got %s", reflect.TypeOf(err))
}

func TestGetVariablesMatchFromVars(t *testing.T) {
	t.Parallel()

	options := &BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]string{
			"foo": "bar",
		},
	}

	boilerplateConfig := &BoilerplateConfig{
		Variables: []Variable{
			Variable{Name: "foo"},
		},
	}

	actual, err := GetVariables(options, boilerplateConfig, map[string]string{})
	expected := map[string]string{
		"foo": "bar",
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesMatchFromVarsAlreadyExists(t *testing.T) {
	t.Parallel()

	options := &BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]string{
			"foo": "bar",
			"bar": "should-not-be-used",
		},
	}

	boilerplateConfig := &BoilerplateConfig{
		Variables: []Variable{
			Variable{Name: "foo"},
			Variable{Name: "bar"},
		},
	}

	actual, err := GetVariables(options, boilerplateConfig, map[string]string{"bar": "already-exists"})
	expected := map[string]string{
		"foo": "bar",
		"bar": "already-exists",
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesMatchFromVarsAndDefaults(t *testing.T) {
	t.Parallel()

	options := &BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	boilerplateConfig := &BoilerplateConfig{
		Variables: []Variable{
			Variable{Name: "key1"},
			Variable{Name: "key2"},
			Variable{Name: "key3", Default: "value3"},
		},
	}

	actual, err := GetVariables(options, boilerplateConfig, map[string]string{})
	expected := map[string]string{
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
		expectedVars  map[string]string
	}{
		{[]string{}, nil, map[string]string{}},
		{[]string{"key=value"}, nil, map[string]string{"key": "value"}},
		{[]string{"key="}, nil, map[string]string{"key": ""}},
		{[]string{"key1=value1", "key2=value2", "key3=value3"}, nil, map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{[]string{"invalidsyntax"}, InvalidVarSyntax("invalidsyntax"), map[string]string{}},
		{[]string{"="}, VariableNameCannotBeEmpty("="), map[string]string{}},
		{[]string{"=foo"}, VariableNameCannotBeEmpty("=foo"), map[string]string{}},
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
		expectedVars        map[string]string
	}{
		{"", false, map[string]string{}},
		{YAML_FILE_ONE_VAR, false, map[string]string{"key": "value"}},
		{YAML_FILE_MULTIPLE_VARS, false, map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{"invalid yaml", true, map[string]string{}},
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
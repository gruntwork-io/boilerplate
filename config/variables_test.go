package config

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"reflect"
	"github.com/gruntwork-io/boilerplate/errors"
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



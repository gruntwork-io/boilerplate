package config

import (
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/variables"
)

func TestGetVariableFromVarsEmptyVars(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo")
	opts := &options.BoilerplateOptions{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}

	_, containsValue := getVariableFromVars(variable, opts)
	assert.False(t, containsValue)
}

func TestGetVariableFromVarsNoMatch(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo")
	opts := &options.BoilerplateOptions{
		Vars: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	_, containsValue := getVariableFromVars(variable, opts)
	assert.False(t, containsValue)
}

func TestGetVariableFromVarsMatch(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo")
	opts := &options.BoilerplateOptions{
		Vars: map[string]interface{}{
			"key1": "value1",
			"foo":  "bar",
			"key3": "value3",
		},
	}

	actual, containsValue := getVariableFromVars(variable, opts)
	expected := "bar"

	assert.True(t, containsValue)
	assert.Equal(t, expected, actual)
}

func TestGetVariableFromVarsForDependencyNoMatch(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("bar.foo")
	opts := &options.BoilerplateOptions{
		Vars: map[string]interface{}{
			"key1": "value1",
			"foo":  "bar",
			"key3": "value3",
		},
	}

	_, containsValue := getVariableFromVars(variable, opts)
	assert.False(t, containsValue)
}

func TestGetVariableFromVarsForDependencyMatch(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("bar.foo")
	opts := &options.BoilerplateOptions{
		Vars: map[string]interface{}{
			"key1":    "value1",
			"bar.foo": "bar",
			"key3":    "value3",
		},
	}

	actual, containsValue := getVariableFromVars(variable, opts)
	expected := "bar"

	assert.True(t, containsValue)
	assert.Equal(t, expected, actual)
}

func TestGetVariableNoMatchNonInteractive(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo")
	opts := &options.BoilerplateOptions{NonInteractive: true}

	_, err := getVariable(variable, opts)

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, MissingVariableWithNonInteractiveMode("foo")), "Expected a MissingVariableWithNonInteractiveMode error but got %s", reflect.TypeOf(err))
}

func TestGetVariableInVarsNonInteractive(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo")
	opts := &options.BoilerplateOptions{
		Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
		NonInteractive: true,
		Vars: map[string]interface{}{
			"key1": "value1",
			"foo":  "bar",
			"key3": "value3",
		},
	}

	actual, err := getVariable(variable, opts)
	expected := "bar"

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariableDefaultNonInteractive(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo").WithDefault("bar")
	opts := &options.BoilerplateOptions{
		Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
		NonInteractive: true,
		Vars: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	actual, err := getVariable(variable, opts)
	expected := "bar"

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesNoVariables(t *testing.T) {
	t.Parallel()

	opts := &options.BoilerplateOptions{NonInteractive: true}
	boilerplateConfig := &BoilerplateConfig{}
	rootBoilerplateConfig := &BoilerplateConfig{}
	dependency := variables.Dependency{}

	actual, err := GetVariables(opts, boilerplateConfig, rootBoilerplateConfig, dependency)
	expected := map[string]interface{}{
		"BoilerplateConfigVars": map[string]variables.Variable{},
		"BoilerplateConfigDeps": map[string]variables.Dependency{},
		"This": map[string]interface{}{
			"Config":     boilerplateConfig,
			"Options":    opts,
			"CurrentDep": dependency,
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesNoMatchNonInteractive(t *testing.T) {
	t.Parallel()

	opts := &options.BoilerplateOptions{NonInteractive: true}
	boilerplateConfig := &BoilerplateConfig{
		Variables: []variables.Variable{
			variables.NewStringVariable("foo"),
		},
	}
	rootBoilerplateConfig := &BoilerplateConfig{}
	dependency := variables.Dependency{}

	_, err := GetVariables(opts, boilerplateConfig, rootBoilerplateConfig, dependency)

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, MissingVariableWithNonInteractiveMode("foo")), "Expected a MissingVariableWithNonInteractiveMode error but got %s", reflect.TypeOf(err))
}

func TestGetVariablesMatchFromVars(t *testing.T) {
	t.Parallel()

	opts := &options.BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]interface{}{
			"foo": "bar",
		},
		OnMissingKey: options.ExitWithError,
	}

	boilerplateConfig := &BoilerplateConfig{
		Variables: []variables.Variable{
			variables.NewStringVariable("foo"),
		},
	}

	rootBoilerplateConfig := &BoilerplateConfig{}

	dependency := variables.Dependency{}

	actual, err := GetVariables(opts, boilerplateConfig, rootBoilerplateConfig, dependency)
	expected := map[string]interface{}{
		"foo":                   "bar",
		"BoilerplateConfigVars": map[string]variables.Variable{},
		"BoilerplateConfigDeps": map[string]variables.Dependency{},
		"This": map[string]interface{}{
			"Config":     boilerplateConfig,
			"Options":    opts,
			"CurrentDep": dependency,
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesMatchFromVarsAndDefaults(t *testing.T) {
	t.Parallel()

	opts := &options.BoilerplateOptions{
		Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
		NonInteractive: true,
		Vars: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
		OnMissingKey: options.ExitWithError,
	}

	boilerplateConfig := &BoilerplateConfig{
		Variables: []variables.Variable{
			variables.NewStringVariable("key1"),
			variables.NewStringVariable("key2"),
			variables.NewStringVariable("key3").WithDefault("value3"),
		},
	}

	rootBoilerplateConfig := &BoilerplateConfig{}

	dependency := variables.Dependency{}

	actual, err := GetVariables(opts, boilerplateConfig, rootBoilerplateConfig, dependency)
	expected := map[string]interface{}{
		"key1":                  "value1",
		"key2":                  "value2",
		"key3":                  "value3",
		"BoilerplateConfigVars": map[string]variables.Variable{},
		"BoilerplateConfigDeps": map[string]variables.Dependency{},
		"This": map[string]interface{}{
			"Config":     boilerplateConfig,
			"Options":    opts,
			"CurrentDep": dependency,
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

package config //nolint:testpackage

import (
	"reflect"
	"testing"

	"golang.org/x/exp/maps"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/testutil"
	"github.com/gruntwork-io/boilerplate/variables"
)

func TestGetVariableFromVarsEmptyVars(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo")
	opts := testutil.CreateTestOptions("")

	_, containsValue := getVariableFromVars(variable, opts)
	assert.False(t, containsValue)
}

func TestGetVariableFromVarsNoMatch(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo")
	opts := &options.BoilerplateOptions{
		Vars: map[string]any{
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
		Vars: map[string]any{
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
		Vars: map[string]any{
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
		Vars: map[string]any{
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
	opts := testutil.CreateTestOptionsForShell(true, false)

	_, err := getVariable(variable, opts)

	require.Error(t, err)
	assert.ErrorIs(t, err, MissingVariableWithNonInteractiveMode("foo"), "Expected a MissingVariableWithNonInteractiveMode error but got %s", reflect.TypeOf(err))
}

func TestGetVariableInVarsNonInteractive(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo")
	opts := &options.BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]any{
			"key1": "value1",
			"foo":  "bar",
			"key3": "value3",
		},
	}

	actual, err := getVariable(variable, opts)
	expected := "bar"

	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariableDefaultNonInteractive(t *testing.T) {
	t.Parallel()

	variable := variables.NewStringVariable("foo").WithDefault("bar")
	opts := &options.BoilerplateOptions{
		NonInteractive: true,
		Vars: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	actual, err := getVariable(variable, opts)
	expected := "bar"

	require.NoError(t, err)
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

	require.NoError(t, err)
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

	require.Error(t, err)
	assert.ErrorIs(t, err, MissingVariableWithNonInteractiveMode("foo"), "Expected a MissingVariableWithNonInteractiveMode error but got %s", reflect.TypeOf(err))
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

	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetVariablesMatchFromVarsAndDefaults(t *testing.T) {
	t.Parallel()

	opts := &options.BoilerplateOptions{
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

	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestValidateUserInput(t *testing.T) {
	t.Parallel()

	// An empty variable with no default value should fail validation
	v := variables.NewStringVariable("foo")
	m, hasValidationErrs := validateUserInput("", v)
	assert.True(t, hasValidationErrs)
	assert.Equal(t, map[string]bool{"Value must be provided": false}, m)

	// An empty variable with a default value should pass validation
	v = variables.NewStringVariable("foo").WithDefault("bar")
	m, hasValidationErrs = validateUserInput("", v)
	assert.False(t, hasValidationErrs)
	assert.Empty(t, m)

	// A non-empty variable should pass validation
	v = variables.NewStringVariable("foo")
	m, hasValidationErrs = validateUserInput("bar", v)
	assert.False(t, hasValidationErrs)
	assert.Empty(t, m)

	// A variable that cannot be parsed should fail validation
	v = variables.NewIntVariable("foo")
	m, hasValidationErrs = validateUserInput("bar", v)
	assert.True(t, hasValidationErrs)

	key := maps.Keys(m)[0]
	assert.Contains(t, key, "Value must be of type int")
}

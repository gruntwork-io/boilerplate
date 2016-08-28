package config

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"reflect"
	"gopkg.in/yaml.v2"
	"github.com/gruntwork-io/boilerplate/errors"
)

func TestParseBoilerplateConfigEmpty(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(""))
	expected := &BoilerplateConfig{}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestParseBoilerplateConfigInvalid(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte("not-a-valid-yaml-file"))

	assert.NotNil(t, err)

	unwrapped := errors.Unwrap(err)
	_, isYamlTypeError := unwrapped.(*yaml.TypeError)
	assert.True(t, isYamlTypeError, "Expected a YAML type error for an invalid yaml file but got %s", reflect.TypeOf(unwrapped))
}

func TestParseBoilerplateConfigEmptyVariables(t *testing.T) {
	t.Parallel()

	configContents := `variables:`

	actual, err := ParseBoilerplateConfig([]byte(configContents))
	expected := &BoilerplateConfig{}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_MINIMAL =
`variables:
  - name: foo
`

func TestParseBoilerplateConfigOneVariableMinimal(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_MINIMAL))
	expected := &BoilerplateConfig{
		Variables: []Variable{
			Variable{Name: "foo"},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_FULL =
`variables:
  - name: foo
    prompt: prompt
    default: default
`

func TestParseBoilerplateConfigOneVariableFull(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_FULL))
	expected := &BoilerplateConfig{
		Variables: []Variable{
			Variable{Name: "foo", Prompt: "prompt", Default: "default"},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_MISSING_NAME =
`variables:
  - prompt: prompt
    default: default
`

func TestParseBoilerplateConfigOneVariableMissingName(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_MISSING_NAME))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, VariableMissingName), "Expected a VariableMissingName error but got %s", reflect.TypeOf(err))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_MULTIPLE_VARIABLES =
`variables:
  - name: foo

  - name: bar
    prompt: prompt

  - name: baz
    prompt: prompt
    default: default
`

func TestParseBoilerplateConfigMultipleVariables(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_MULTIPLE_VARIABLES))
	expected := &BoilerplateConfig{
		Variables: []Variable{
			Variable{Name: "foo"},
			Variable{Name: "bar", Prompt: "prompt"},
			Variable{Name: "baz", Prompt: "prompt", Default: "default"},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_DEPENDENCY =
`dependencies:
  - template-folder: /template/folder1
    output-folder: /output/folder1
`

func TestParseBoilerplateConfigOneDependency(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_DEPENDENCY))
	expected := &BoilerplateConfig{
		Dependencies: []Dependency{
			Dependency{TemplateFolder: "/template/folder1", OutputFolder: "/output/folder1", DontInheritVariables: false},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_MULTIPLE_DEPENDENCIES =
`dependencies:
  - template-folder: /template/folder1
    output-folder: /output/folder1

  - template-folder: /template/folder2
    output-folder: /output/folder2
    dont-inherit-variables: true

  - template-folder: /template/folder3
    output-folder: /output/folder3
`

func TestParseBoilerplateConfigMultipleDependencies(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_MULTIPLE_DEPENDENCIES))
	expected := &BoilerplateConfig{
		Dependencies: []Dependency{
			Dependency{TemplateFolder: "/template/folder1", OutputFolder: "/output/folder1", DontInheritVariables: false},
			Dependency{TemplateFolder: "/template/folder2", OutputFolder: "/output/folder2", DontInheritVariables: true},
			Dependency{TemplateFolder: "/template/folder3", OutputFolder: "/output/folder3", DontInheritVariables: false},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_TEMPLATE_FOLDER =
`dependencies:
  - output-folder: /output/folder1
`

func TestParseBoilerplateConfigDependencyMissingTemplateFolder(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_TEMPLATE_FOLDER))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, TemplateFolderCannotBeEmptyForDependency(0)), "Expected a TemplateFolderCannotBeEmptyForDependency error but got %s", reflect.TypeOf(err))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_OUTPUT_FOLDER =
`dependencies:
  - template-folder: /template/folder1
    output-folder: /output/folder1

  - template-folder: /template/folder2
`

func TestParseBoilerplateConfigDependencyMissingOutputFolder(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_OUTPUT_FOLDER))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, OutputFolderCannotBeEmptyForDependency(1)), "Expected a OutputFolderCannotBeEmptyForDependency error but got %s", reflect.TypeOf(err))
}

func TestLoadBoilerPlateConfigFullConfig(t *testing.T) {
	t.Parallel()

	actual, err := LoadBoilerPlateConfig(&BoilerplateOptions{TemplateFolder: "../test-fixtures/config-test/full-config"})
	expected := &BoilerplateConfig{
		Variables: []Variable{
			Variable{Name: "foo"},
			Variable{Name: "bar", Prompt: "prompt"},
			Variable{Name: "baz", Prompt: "prompt", Default: "default"},
		},
		Dependencies: []Dependency{
			Dependency{TemplateFolder: "/template/folder1", OutputFolder: "/output/folder1", DontInheritVariables: false},
			Dependency{TemplateFolder: "/template/folder2", OutputFolder: "/output/folder2", DontInheritVariables: true},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestLoadBoilerPlateConfigNoConfig(t *testing.T) {
	t.Parallel()

	actual, err := LoadBoilerPlateConfig(&BoilerplateOptions{TemplateFolder: "../test-fixtures/config-test/no-config"})
	expected := &BoilerplateConfig{}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestLoadBoilerPlateConfigInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := LoadBoilerPlateConfig(&BoilerplateOptions{TemplateFolder: "../test-fixtures/config-test/invalid-config"})

	assert.NotNil(t, err)

	unwrapped := errors.Unwrap(err)
	_, isYamlTypeError := unwrapped.(*yaml.TypeError)
	assert.True(t, isYamlTypeError, "Expected a YAML type error for an invalid yaml file but got %s", reflect.TypeOf(unwrapped))
}

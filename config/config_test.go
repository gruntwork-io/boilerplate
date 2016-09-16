package config

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"reflect"
	"gopkg.in/yaml.v2"
	"github.com/gruntwork-io/boilerplate/errors"
	"path"
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

  - name: dep1.baz
    prompt: another-prompt
    default: another-default
`

func TestParseBoilerplateConfigMultipleVariables(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_MULTIPLE_VARIABLES))
	expected := &BoilerplateConfig{
		Variables: []Variable{
			Variable{Name: "foo"},
			Variable{Name: "bar", Prompt: "prompt"},
			Variable{Name: "baz", Prompt: "prompt", Default: "default"},
			Variable{Name: "dep1.baz", Prompt: "another-prompt", Default: "another-default"},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_DEPENDENCY =
`dependencies:
  - name: dep1
    template-folder: /template/folder1
    output-folder: /output/folder1
`

func TestParseBoilerplateConfigOneDependency(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_DEPENDENCY))
	expected := &BoilerplateConfig{
		Dependencies: []Dependency{
			Dependency{Name: "dep1", TemplateFolder: "/template/folder1", OutputFolder: "/output/folder1", DontInheritVariables: false},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_MULTIPLE_DEPENDENCIES =
`dependencies:
  - name: dep1
    template-folder: /template/folder1
    output-folder: /output/folder1

  - name: dep2
    template-folder: /template/folder2
    output-folder: /output/folder2
    dont-inherit-variables: true
    variables:
      - name: var1
        prompt: Enter var1
        default: foo

  - name: dep3
    template-folder: /template/folder3
    output-folder: /output/folder3
`

func TestParseBoilerplateConfigMultipleDependencies(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_MULTIPLE_DEPENDENCIES))
	expected := &BoilerplateConfig{
		Dependencies: []Dependency{
			Dependency{
				Name: "dep1",
				TemplateFolder: "/template/folder1",
				OutputFolder: "/output/folder1",
				DontInheritVariables: false,
			},
			Dependency{
				Name: "dep2",
				TemplateFolder: "/template/folder2",
				OutputFolder: "/output/folder2",
				DontInheritVariables: true,
				Variables: []Variable{Variable{Name: "var1", Prompt: "Enter var1", Default: "foo"}},
			},
			Dependency{
				Name: "dep3",
				TemplateFolder: "/template/folder3",
				OutputFolder: "/output/folder3",
				DontInheritVariables: false,
			},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_NAME =
`dependencies:
  - template-folder: /template/folder1
    output-folder: /output/folder1
`

func TestParseBoilerplateConfigDependencyMissingName(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_NAME))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, MissingNameForDependency(0)), "Expected a MissingNameForDependency error but got %s", reflect.TypeOf(err))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_TEMPLATE_FOLDER =
`dependencies:
  - name: dep1
    output-folder: /output/folder1
`

func TestParseBoilerplateConfigDependencyMissingTemplateFolder(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_TEMPLATE_FOLDER))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, MissingTemplateFolderForDependency("dep1")), "Expected a MissingTemplateFolderForDependency error but got %s", reflect.TypeOf(err))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_VARIABLE_NAME =
`dependencies:
  - name: dep1
    template-folder: /template/folder1
    output-folder: /output/folder1
    variables:
      - prompt: Enter foo
        default: foo
`

func TestParseBoilerplateConfigDependencyMissingVariableName(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_VARIABLE_NAME))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, VariableMissingName), "Expected a VariableMissingName error but got %s", reflect.TypeOf(err))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_OUTPUT_FOLDER =
`dependencies:
  - name: dep1
    template-folder: /template/folder1
    output-folder: /output/folder1

  - name: dep2
    template-folder: /template/folder2
`

func TestParseBoilerplateConfigDependencyMissingOutputFolder(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_OUTPUT_FOLDER))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, MissingOutputFolderForDependency("dep2")), "Expected a MissingOutputFolderForDependency error but got %s", reflect.TypeOf(err))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_DUPLICATE_NAMES =
`dependencies:
  - name: dep1
    template-folder: /template/folder1
    output-folder: /output/folder1

  - name: dep2
    template-folder: /template/folder2
    output-folder: /output/folder2

  - name: dep1
    template-folder: /template/folder3
    output-folder: /output/folder3
`

func TestParseBoilerplateConfigDependencyDuplicateNames(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_DUPLICATE_NAMES))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, DuplicateDependencyName("dep1")), "Expected a DuplicateDependencyName error but got %s", reflect.TypeOf(err))
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
			Dependency{Name: "dep1", TemplateFolder: "/template/folder1", OutputFolder: "/output/folder1", DontInheritVariables: false},
			Dependency{Name: "dep2", TemplateFolder: "/template/folder2", OutputFolder: "/output/folder2", DontInheritVariables: true, Variables: []Variable{
				Variable{Name: "baz", Prompt: "prompt", Default: "other-default"},
				Variable{Name: "abc", Prompt: "prompt", Default: "default"},
			}},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestLoadBoilerPlateConfigNoConfig(t *testing.T) {
	t.Parallel()

	templateFolder := "../test-fixtures/config-test/no-config"
	_, err := LoadBoilerPlateConfig(&BoilerplateOptions{TemplateFolder: templateFolder})
	expectedErr := BoilerplateConfigNotFound(path.Join(templateFolder, "boilerplate.yml"))

	assert.True(t, errors.IsError(err, expectedErr), "Expected error %v but got %v", expectedErr, err)
}

func TestLoadBoilerPlateConfigNoConfigIgnore(t *testing.T) {
	t.Parallel()

	templateFolder := "../test-fixtures/config-test/no-config"
	actual, err := LoadBoilerPlateConfig(&BoilerplateOptions{TemplateFolder: templateFolder, OnMissingConfig: Ignore})
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

func TestSplitIntoDependencyNameAndVariableName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		variableName                 string
		expectedDependencyName       string
		expectedOriginalVariableName string
	}{
		{"", "", ""},
		{"foo", "", "foo"},
		{"foo-bar baz_blah", "", "foo-bar baz_blah"},
		{"foo.bar", "foo", "bar"},
		{"foo.bar.baz", "foo", "bar.baz"},
	}

	for _, testCase := range testCases {
		actualDependencyName, actualOriginalVariableName := SplitIntoDependencyNameAndVariableName(testCase.variableName)
		assert.Equal(t, testCase.expectedDependencyName, actualDependencyName, "Variable name: %s", testCase.variableName)
		assert.Equal(t, testCase.expectedOriginalVariableName, actualOriginalVariableName, "Variable name: %s", testCase.variableName)
	}
}
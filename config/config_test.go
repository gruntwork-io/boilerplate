package config

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"reflect"
	"gopkg.in/yaml.v2"
	"github.com/gruntwork-io/boilerplate/errors"
	"path"
	"github.com/gruntwork-io/boilerplate/variables"
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
	assert.True(t, isYamlTypeError, "Expected a YAML type error for an invalid yaml file but got %s: %v", reflect.TypeOf(unwrapped), unwrapped)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_EMPTY_VARIABLES_AND_DEPENDENCIES =
`variables:
dependencies:
`

func TestParseBoilerplateConfigEmptyVariablesAndDependencies(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_EMPTY_VARIABLES_AND_DEPENDENCIES))
	expected := &BoilerplateConfig{
		Variables: []variables.Variable{},
		Dependencies: []variables.Dependency{},
	}

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
		Variables: []variables.Variable{
			variables.NewStringVariable("foo"),
		},
		Dependencies: []variables.Dependency{},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_FULL =
`variables:
  - name: foo
    description: example description
    type: string
    default: default
`

func TestParseBoilerplateConfigOneVariableFull(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_FULL))
	expected := &BoilerplateConfig{
		Variables: []variables.Variable{
			variables.NewStringVariable("foo").WithDescription("example description").WithDefault("default"),
		},
		Dependencies: []variables.Dependency{},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_MISSING_NAME =
`variables:
  - description: example description
    default: default
`

func TestParseBoilerplateConfigOneVariableMissingName(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_MISSING_NAME))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.RequiredFieldMissing("name")), "Expected a RequiredFieldMissing error but got %s: %v", reflect.TypeOf(errors.Unwrap(err)), err)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_INVALID_TYPE =
`variables:
  - name: foo
    type: foo
`

func TestParseBoilerplateConfigOneVariableInvalidType(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_INVALID_TYPE))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.InvalidBoilerplateType("foo")), "Expected a InvalidBoilerplateType error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_INVALID_TYPE_FOR_NAME_FIELD =
`variables:
  - name:
      - foo
      - bar
`

func TestParseBoilerplateConfigInvalidTypeForNameField(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_INVALID_TYPE_FOR_NAME_FIELD))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.InvalidTypeForField{FieldName: "name", ExpectedType: "string", ActualType: reflect.TypeOf([]interface{}{})}), "Expected a InvalidTypeForField error but got %s: %v", reflect.TypeOf(errors.Unwrap(err)), err)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_ENUM_NO_OPTIONS =
`variables:
  - name: foo
    type: enum
`

func TestParseBoilerplateConfigOneVariableEnumNoOptions(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_ENUM_NO_OPTIONS))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.OptionsMissing("foo")), "Expected a VariableMissingOptions error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_ENUM_OPTIONS_WRONG_TYPE =
`variables:
  - name: foo
    type: enum
    options: foo
`

func TestParseBoilerplateConfigOneVariableEnumWrongType(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_ENUM_OPTIONS_WRONG_TYPE))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.InvalidTypeForField{FieldName: "options", ExpectedType: "List", ActualType: reflect.TypeOf("string"), Context: "foo"}), "Expected a InvalidTypeForField error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_ENUM_INVALID_DEFAULT =
`variables:
  - name: foo
    type: enum
    options:
      - foo
      - bar
      - baz
    default: invalid
`

func TestParseBoilerplateConfigOneVariableEnumInvalidDefault(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_ENUM_INVALID_DEFAULT))

	assert.NotNil(t, err)
	_, isInvalidVariableValueErr := errors.Unwrap(err).(variables.InvalidVariableValue)
	assert.True(t, isInvalidVariableValueErr, "Expected a InvalidVariableValue error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_OPTIONS_FOR_NON_ENUM =
`variables:
  - name: foo
    options:
      - foo
      - bar
`

func TestParseBoilerplateConfigOneVariableOptionsForNonEnum(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_VARIABLE_OPTIONS_FOR_NON_ENUM))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.OptionsCanOnlyBeUsedWithEnum{Context: "foo", Type: variables.String}), "Expected a OptionsCanOnlyBeUsedWithEnum error but got %v", err)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_MULTIPLE_VARIABLES =
`variables:
  - name: foo

  - name: bar
    description: example description

  - name: baz
    description: example description
    type: int
    default: 3

  - name: dep1.baz
    description: another example description
    type: bool
    default: true
`

func TestParseBoilerplateConfigMultipleVariables(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_MULTIPLE_VARIABLES))
	expected := &BoilerplateConfig{
		Variables: []variables.Variable{
			variables.NewStringVariable("foo"),
			variables.NewStringVariable("bar").WithDescription("example description"),
			variables.NewIntVariable("baz").WithDescription("example description").WithDefault(3),
			variables.NewBoolVariable("dep1.baz").WithDescription("another example description").WithDefault(true),
		},
		Dependencies: []variables.Dependency{},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ALL_TYPES =
`variables:
  - name: var1
    default: foo

  - name: var2
    type: string
    default: foo

  - name: var3
    type: int
    default: 5

  - name: var4
    type: float
    default: 5.5

  - name: var5
    type: bool
    default: true

  - name: var6
    type: list
    default:
      - foo
      - bar
      - baz

  - name: var7
    type: map
    default:
      key1: value1
      key2: value2
      key3: value3

  - name: var8
    type: enum
    options:
      - foo
      - bar
      - baz
    default: bar
`

func TestParseBoilerplateConfigAllTypes(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_ALL_TYPES))
	expected := &BoilerplateConfig{
		Variables: []variables.Variable{
			variables.NewStringVariable("var1").WithDefault("foo"),
			variables.NewStringVariable("var2").WithDefault("foo"),
			variables.NewIntVariable("var3").WithDefault(5),
			variables.NewFloatVariable("var4").WithDefault(5.5),
			variables.NewBoolVariable("var5").WithDefault(true),
			variables.NewListVariable("var6").WithDefault([]string{"foo", "bar", "baz"}),
			variables.NewMapVariable("var7").WithDefault(map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"}),
			variables.NewEnumVariable("var8", []string{"foo", "bar", "baz"}).WithDefault("bar"),
		},
		Dependencies: []variables.Dependency{},
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
		Variables: []variables.Variable{},
		Dependencies: []variables.Dependency{
			{Name: "dep1", TemplateFolder: "/template/folder1", OutputFolder: "/output/folder1", DontInheritVariables: false, Variables: []variables.Variable{}},
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
        description: Enter var1
        default: foo

  - name: dep3
    template-folder: /template/folder3
    output-folder: /output/folder3
`

func TestParseBoilerplateConfigMultipleDependencies(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_MULTIPLE_DEPENDENCIES))
	expected := &BoilerplateConfig{
		Variables: []variables.Variable{},
		Dependencies: []variables.Dependency{
			{
				Name: "dep1",
				TemplateFolder: "/template/folder1",
				OutputFolder: "/output/folder1",
				DontInheritVariables: false,
				Variables: []variables.Variable{},
			},
			{
				Name: "dep2",
				TemplateFolder: "/template/folder2",
				OutputFolder: "/output/folder2",
				DontInheritVariables: true,
				Variables: []variables.Variable{
					variables.NewStringVariable("var1").WithDescription("Enter var1").WithDefault("foo"),
				},
			},
			{
				Name: "dep3",
				TemplateFolder: "/template/folder3",
				OutputFolder: "/output/folder3",
				DontInheritVariables: false,
				Variables: []variables.Variable{},
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
	assert.True(t, errors.IsError(err, variables.RequiredFieldMissing("name")), "Expected a RequiredFieldMissing error but got %s", reflect.TypeOf(errors.Unwrap(err)))
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
	assert.True(t, errors.IsError(err, variables.RequiredFieldMissing("template-folder")), "Expected a RequiredFieldMissing error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_VARIABLE_NAME =
`dependencies:
  - name: dep1
    template-folder: /template/folder1
    output-folder: /output/folder1
    variables:
      - description: Enter foo
        default: foo
`

func TestParseBoilerplateConfigDependencyMissingVariableName(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_VARIABLE_NAME))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.RequiredFieldMissing("name")), "Expected a RequiredFieldMissing error but got %s", reflect.TypeOf(errors.Unwrap(err)))
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
	assert.True(t, errors.IsError(err, variables.RequiredFieldMissing("output-folder")), "Expected a RequiredFieldMissing error but got %s", reflect.TypeOf(errors.Unwrap(err)))
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
	assert.True(t, errors.IsError(err, variables.DuplicateDependencyName("dep1")), "Expected a DuplicateDependencyName error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

func TestLoadBoilerplateConfigFullConfig(t *testing.T) {
	t.Parallel()

	actual, err := LoadBoilerplateConfig(&BoilerplateOptions{TemplateFolder: "../test-fixtures/config-test/full-config"})
	expected := &BoilerplateConfig{
		Variables: []variables.Variable{
			variables.NewStringVariable("foo"),
			variables.NewStringVariable("bar").WithDescription("example description"),
			variables.NewStringVariable("baz").WithDescription("example description").WithDefault("default"),
		},
		Dependencies: []variables.Dependency{
			{Name: "dep1", TemplateFolder: "/template/folder1", OutputFolder: "/output/folder1", DontInheritVariables: false, Variables: []variables.Variable{}},
			{Name: "dep2", TemplateFolder: "/template/folder2", OutputFolder: "/output/folder2", DontInheritVariables: true, Variables: []variables.Variable{
				variables.NewStringVariable("baz").WithDescription("example description").WithDefault("other-default"),
				variables.NewStringVariable("abc").WithDescription("example description").WithDefault("default"),
			}},
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestLoadBoilerplateConfigNoConfig(t *testing.T) {
	t.Parallel()

	templateFolder := "../test-fixtures/config-test/no-config"
	_, err := LoadBoilerplateConfig(&BoilerplateOptions{TemplateFolder: templateFolder})
	expectedErr := BoilerplateConfigNotFound(path.Join(templateFolder, "boilerplate.yml"))

	assert.True(t, errors.IsError(err, expectedErr), "Expected error %v but got %v", expectedErr, err)
}

func TestLoadBoilerplateConfigNoConfigIgnore(t *testing.T) {
	t.Parallel()

	templateFolder := "../test-fixtures/config-test/no-config"
	actual, err := LoadBoilerplateConfig(&BoilerplateOptions{TemplateFolder: templateFolder, OnMissingConfig: Ignore})
	expected := &BoilerplateConfig{}

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestLoadBoilerplateConfigInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := LoadBoilerplateConfig(&BoilerplateOptions{TemplateFolder: "../test-fixtures/config-test/invalid-config"})

	assert.NotNil(t, err)

	unwrapped := errors.Unwrap(err)
	_, isYamlTypeError := unwrapped.(*yaml.TypeError)
	assert.True(t, isYamlTypeError, "Expected a YAML type error for an invalid yaml file but got %s", reflect.TypeOf(unwrapped))
}
package config

import (
	"bytes"
	"io"
	"io/ioutil"
	"log/slog"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/variables"

	// NOTE: we use this library to format the yaml to a standard format that lexicographically sorts the dictionary
	// keys to make comparisons more robust. We should not however use it in the CLI itself because it does not make the
	// YAML more readable (it culls all whitespace).
	"github.com/stuart-warren/yamlfmt"
)

func TestParseBoilerplateConfigEmpty(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(""))
	expected := &BoilerplateConfig{}

	assert.Nil(t, err, "Unexpected error: %v", err)
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
const CONFIG_EMPTY_VARIABLES_AND_DEPENDENCIES = `variables:
dependencies:
`

func TestParseBoilerplateConfigEmptyVariablesAndDependencies(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_EMPTY_VARIABLES_AND_DEPENDENCIES))
	expected := &BoilerplateConfig{
		Variables:    []variables.Variable{},
		Dependencies: []variables.Dependency{},
		Hooks:        variables.Hooks{},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_MINIMAL = `variables:
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
		Hooks:        variables.Hooks{},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_FULL = `variables:
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
		Hooks:        variables.Hooks{},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_VARIABLE_MISSING_NAME = `variables:
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
const CONFIG_ONE_VARIABLE_INVALID_TYPE = `variables:
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
const CONFIG_ONE_VARIABLE_INVALID_TYPE_FOR_NAME_FIELD = `variables:
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
const CONFIG_ONE_VARIABLE_ENUM_NO_OPTIONS = `variables:
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
const CONFIG_ONE_VARIABLE_ENUM_OPTIONS_WRONG_TYPE = `variables:
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
const CONFIG_ONE_VARIABLE_OPTIONS_FOR_NON_ENUM = `variables:
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
const CONFIG_MULTIPLE_VARIABLES = `variables:
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
		Hooks:        variables.Hooks{},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ALL_TYPES = `variables:
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
			variables.NewListVariable("var6").WithDefault([]interface{}{"foo", "bar", "baz"}),
			variables.NewMapVariable("var7").WithDefault(map[interface{}]interface{}{"key1": "value1", "key2": "value2", "key3": "value3"}),
			variables.NewEnumVariable("var8", []string{"foo", "bar", "baz"}).WithDefault("bar"),
		},
		Dependencies: []variables.Dependency{},
		Hooks:        variables.Hooks{},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_DEPENDENCY = `dependencies:
  - name: dep1
    template-url: /template/folder1
    output-folder: /output/folder1
`

func TestParseBoilerplateConfigOneDependency(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_DEPENDENCY))
	expected := &BoilerplateConfig{
		Variables: []variables.Variable{},
		Dependencies: []variables.Dependency{
			{Name: "dep1", TemplateUrl: "/template/folder1", OutputFolder: "/output/folder1", DontInheritVariables: false, Variables: []variables.Variable{}},
		},
		Hooks: variables.Hooks{},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_MULTIPLE_DEPENDENCIES = `dependencies:
  - name: dep1
    template-url: /template/folder1
    output-folder: /output/folder1

  - name: dep2
    template-url: /template/folder2
    output-folder: /output/folder2
    dont-inherit-variables: true
    variables:
      - name: var1
        description: Enter var1
        default: foo

  - name: dep3
    template-url: /template/folder3
    output-folder: /output/folder3
    skip: "{{ and .Foo .Bar }}"
`

func TestParseBoilerplateConfigMultipleDependencies(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_MULTIPLE_DEPENDENCIES))
	expected := &BoilerplateConfig{
		Variables: []variables.Variable{},
		Dependencies: []variables.Dependency{
			{
				Name:                 "dep1",
				TemplateUrl:          "/template/folder1",
				OutputFolder:         "/output/folder1",
				DontInheritVariables: false,
				Variables:            []variables.Variable{},
			},
			{
				Name:                 "dep2",
				TemplateUrl:          "/template/folder2",
				OutputFolder:         "/output/folder2",
				DontInheritVariables: true,
				Variables: []variables.Variable{
					variables.NewStringVariable("var1").WithDescription("Enter var1").WithDefault("foo"),
				},
			},
			{
				Name:                 "dep3",
				TemplateUrl:          "/template/folder3",
				OutputFolder:         "/output/folder3",
				DontInheritVariables: false,
				Variables:            []variables.Variable{},
				Skip:                 "{{ and .Foo .Bar }}",
			},
		},
		Hooks: variables.Hooks{},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_NAME = `dependencies:
  - template-url: /template/folder1
    output-folder: /output/folder1
`

func TestParseBoilerplateConfigDependencyMissingName(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_NAME))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.RequiredFieldMissing("name")), "Expected a RequiredFieldMissing error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_TEMPLATE_FOLDER = `dependencies:
  - name: dep1
    output-folder: /output/folder1
`

func TestParseBoilerplateConfigDependencyMissingTemplateUrl(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_TEMPLATE_FOLDER))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.RequiredFieldMissing("template-url")), "Expected a RequiredFieldMissing error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_MISSING_VARIABLE_NAME = `dependencies:
  - name: dep1
    template-url: /template/folder1
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
const CONFIG_DEPENDENCY_MISSING_OUTPUT_FOLDER = `dependencies:
  - name: dep1
    template-url: /template/folder1
    output-folder: /output/folder1

  - name: dep2
    template-url: /template/folder2
`

func TestParseBoilerplateConfigDependencyMissingOutputFolder(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_MISSING_OUTPUT_FOLDER))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.RequiredFieldMissing("output-folder")), "Expected a RequiredFieldMissing error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_DEPENDENCY_DUPLICATE_NAMES = `dependencies:
  - name: dep1
    template-url: /template/folder1
    output-folder: /output/folder1

  - name: dep2
    template-url: /template/folder2
    output-folder: /output/folder2

  - name: dep1
    template-url: /template/folder3
    output-folder: /output/folder3
`

func TestParseBoilerplateConfigDependencyDuplicateNames(t *testing.T) {
	t.Parallel()

	_, err := ParseBoilerplateConfig([]byte(CONFIG_DEPENDENCY_DUPLICATE_NAMES))

	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, variables.DuplicateDependencyName("dep1")), "Expected a DuplicateDependencyName error but got %s", reflect.TypeOf(errors.Unwrap(err)))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_EMPTY_HOOKS = `hooks:
`

func TestParseBoilerplateConfigEmptyHooks(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_EMPTY_HOOKS))
	expected := &BoilerplateConfig{
		Variables:    []variables.Variable{},
		Dependencies: []variables.Dependency{},
		Hooks:        variables.Hooks{},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_EMPTY_BEFORE_AND_AFTER_HOOKS = `hooks:
   before:
   after:
`

func TestParseBoilerplateConfigEmptyBeforeAndAfterHooks(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_EMPTY_BEFORE_AND_AFTER_HOOKS))
	expected := &BoilerplateConfig{
		Variables:    []variables.Variable{},
		Dependencies: []variables.Dependency{},
		Hooks:        variables.Hooks{},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_BEFORE_HOOK_NO_ARGS = `hooks:
   before:
     - command: foo
`

func TestParseBoilerplateConfigOneBeforeHookNoArgs(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_BEFORE_HOOK_NO_ARGS))
	expected := &BoilerplateConfig{
		Variables:    []variables.Variable{},
		Dependencies: []variables.Dependency{},
		Hooks: variables.Hooks{
			BeforeHooks: []variables.Hook{
				{Command: "foo"},
			},
		},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_ONE_AFTER_HOOK_WITH_ARGS = `hooks:
   after:
     - command: foo
       args:
         - bar
         - baz
       env:
         foo: bar
`

func TestParseBoilerplateConfigOneAfterHookWithArgs(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_ONE_AFTER_HOOK_WITH_ARGS))
	expected := &BoilerplateConfig{
		Variables:    []variables.Variable{},
		Dependencies: []variables.Dependency{},
		Hooks: variables.Hooks{
			AfterHooks: []variables.Hook{
				{Command: "foo", Args: []string{"bar", "baz"}, Env: map[string]string{"foo": "bar"}},
			},
		},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const CONFIG_MULTIPLE_HOOKS = `hooks:
   before:
     - command: echo
       args:
         - Hello World
       env:
         foo: bar
         baz: blah
       skip: "true"

     - command: run-some-script.sh
       args:
         - "{{ .foo }}"
         - "{{ .bar }}"

   after:
     - command: foo
       skip: "{{ .baz }}"
     - command: bar
`

func TestParseBoilerplateConfigMultipleHooks(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(CONFIG_MULTIPLE_HOOKS))
	expected := &BoilerplateConfig{
		Variables:    []variables.Variable{},
		Dependencies: []variables.Dependency{},
		Hooks: variables.Hooks{
			BeforeHooks: []variables.Hook{
				{Command: "echo", Args: []string{"Hello World"}, Env: map[string]string{"foo": "bar", "baz": "blah"}, Skip: "true"},
				{Command: "run-some-script.sh", Args: []string{"{{ .foo }}", "{{ .bar }}"}},
			},
			AfterHooks: []variables.Hook{
				{Command: "foo", Skip: "{{ .baz }}"},
				{Command: "bar"},
			},
		},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

func TestLoadBoilerplateConfigFullConfig(t *testing.T) {
	t.Parallel()

	actual, err := LoadBoilerplateConfig(&options.BoilerplateOptions{
		Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
		TemplateFolder: "../test-fixtures/config-test/full-config",
	})
	expected := &BoilerplateConfig{
		Partials: []string{"../templates/foo"},
		Variables: []variables.Variable{
			variables.NewStringVariable("foo"),
			variables.NewStringVariable("bar").WithDescription("example description"),
			variables.NewStringVariable("baz").WithDescription("example description").WithDefault("default"),
		},
		Dependencies: []variables.Dependency{
			{Name: "dep1", TemplateUrl: "/template/folder1", OutputFolder: "/output/folder1", DontInheritVariables: false, Variables: []variables.Variable{}},
			{Name: "dep2", TemplateUrl: "/template/folder2", OutputFolder: "/output/folder2", DontInheritVariables: true, Variables: []variables.Variable{
				variables.NewStringVariable("baz").WithDescription("example description").WithDefault("other-default"),
				variables.NewStringVariable("abc").WithDescription("example description").WithDefault("default"),
			}},
		},
		Hooks: variables.Hooks{
			BeforeHooks: []variables.Hook{
				{Command: "echo", Args: []string{"Hello World"}},
			},
			AfterHooks: []variables.Hook{
				{Command: "foo"},
				{Command: "bar"},
			},
		},
	}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

func TestLoadBoilerplateConfigNoConfig(t *testing.T) {
	t.Parallel()

	templateFolder := "../test-fixtures/config-test/no-config"
	_, err := LoadBoilerplateConfig(&options.BoilerplateOptions{TemplateFolder: templateFolder})
	expectedErr := BoilerplateConfigNotFound(path.Join(templateFolder, "boilerplate.yml"))

	assert.True(t, errors.IsError(err, expectedErr), "Expected error %v but got %v", expectedErr, err)
}

func TestLoadBoilerplateConfigNoConfigIgnore(t *testing.T) {
	t.Parallel()

	templateFolder := "../test-fixtures/config-test/no-config"
	actual, err := LoadBoilerplateConfig(&options.BoilerplateOptions{
		Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
		TemplateFolder: templateFolder, OnMissingConfig: options.Ignore,
	})
	expected := &BoilerplateConfig{}

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

func TestLoadBoilerplateConfigInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := LoadBoilerplateConfig(&options.BoilerplateOptions{
		Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
		TemplateFolder: "../test-fixtures/config-test/invalid-config",
	})

	assert.NotNil(t, err)

	unwrapped := errors.Unwrap(err)
	_, isYamlTypeError := unwrapped.(*yaml.TypeError)
	assert.True(t, isYamlTypeError, "Expected a YAML type error for an invalid yaml file but got %s", reflect.TypeOf(unwrapped))
}

// YAML is whitespace sensitive, so we need to be careful that we don't introduce unnecessary indentation
const configWithSkipFiles = `skip_files:
  - path: "docs/README_ALWAYS_SKIP.md"
  - path: "docs/README_MAYBE_SKIP.md"
    if: "{{ .MaybeSkip }}"
  - not_path: "docs/**/*"
    if: "{{ .DocsOnly }}"
`

func TestParseBoilerplateConfigWithSkipFiles(t *testing.T) {
	t.Parallel()

	actual, err := ParseBoilerplateConfig([]byte(configWithSkipFiles))
	require.NoError(t, err)

	expected := &BoilerplateConfig{
		Variables:    []variables.Variable{},
		Dependencies: []variables.Dependency{},
		Hooks:        variables.Hooks{},
		SkipFiles: []variables.SkipFile{
			{Path: "docs/README_ALWAYS_SKIP.md", If: ""},
			{Path: "docs/README_MAYBE_SKIP.md", If: "{{ .MaybeSkip }}"},
			{NotPath: "docs/**/*", If: "{{ .DocsOnly }}"},
		},
	}
	assert.Equal(t, expected, actual)
}

func TestMarshalBoilerplateConfig(t *testing.T) {
	t.Parallel()

	marshalYamlTestExpectedBase := filepath.Join("..", "test-fixtures", "marshal-yaml-test")
	examplesBase := filepath.Join("..", "examples", "for-learning-and-testing")

	examplesToTest, err := ioutil.ReadDir(marshalYamlTestExpectedBase)
	require.NoError(t, err)

	for _, exampleFolder := range examplesToTest {
		exampleFolderName := exampleFolder.Name()

		t.Run(exampleFolderName, func(t *testing.T) {
			t.Parallel()

			configData, err := ioutil.ReadFile(filepath.Join(examplesBase, exampleFolderName, "boilerplate.yml"))
			require.NoError(t, err)
			config, err := ParseBoilerplateConfig(configData)
			require.NoError(t, err)
			actualYml, err := yaml.Marshal(config)
			require.NoError(t, err)
			expectedYml, err := ioutil.ReadFile(filepath.Join(marshalYamlTestExpectedBase, exampleFolderName, "expected.yml"))
			require.NoError(t, err)

			// Format the two yaml documents
			expectedYmlFormatted := formatYAMLBytes(t, expectedYml)
			actualYmlFormatted := formatYAMLBytes(t, actualYml)
			assert.Equal(t, string(expectedYmlFormatted), string(actualYmlFormatted))
		})
	}
}

func formatYAMLBytes(t *testing.T, ymlData []byte) []byte {
	ymlBuffer := bytes.NewBuffer(ymlData)
	formattedYml, err := yamlfmt.Format(ymlBuffer)
	require.NoError(t, err)
	return formattedYml
}

# Boilerplate

Boilerplate is a tool for generating files and folders ("boilerplate") from a set of templates.

Example use cases:

1. Create the scaffolding for a new project (e.g. like [html5boilerplate](https://html5boilerplate.com/))
1. Fill in boilerplate sections in your code files, such as including a legal disclaimer or license at the top of each
   source file, or updating a version number in a file after each build.
1. Embed code snippets from actual source files in documentation. Most READMEs have code copy/pasted into them and that
   code often has syntax errors or goes out of date. Now you can keep those code examples in normal source files which
   are built & tested, and embed parts of those files dynamically in your docs.

## Example: creating a new template

Create a folder called `website-boilerplate` and put a file called `boilerplate.yml` in it:

```yml
variables:
  - name: Title

  - name: WelcomeText
    description: Enter the welcome text for the website

  - name: ShowLogo
    description: Should the website show the logo?
    type: bool
    default: true

```

This file defines 3 variables: `Title`, `WelcomeText`, and `ShowLogo`. When you run Boilerplate, it will prompt
the user for each one.

Next, create an `index.html` in the `website-boilerplate` folder that uses these variables using [Go
Template](https://golang.org/pkg/text/template) syntax:

```html
<html>
  <head>
    <title>{{.Title}}</title>
  </head>
  <body>
    <h1>{{.WelcomeText}}</h1>
    {{if .ShowLogo}}<img src="logo.png">{{end}}
  </body>
</html>
```

Copy an image into the `website-boilerplate` folder and call it `logo.png`.

Finally, run `boilerplate`, setting the `--template-url` to `website-boilerplate` and `--output-folder` to the path
where you want the generated code to go:

```
boilerplate --template-url /home/ubuntu/website-boilerplate --output-folder /home/ubuntu/website-output

Title

  Enter a value [type: string]: Boilerplate Example

WelcomeText
  Enter the welcome text for the website

  Enter a value [type: string]: Welcome!

ShowLogo
  Should the website show the logo?

  Enter a [type: bool]: true

Generating /home/ubuntu/website-output/index.html
Copying /home/ubuntu/website-output/logo.png
```

Boilerplate copies any files from the `--template-url` into the `--output-folder`, passing them through the
[Go Template](https://golang.org/pkg/text/template) engine along the way. After running the command above, the
`website-output` folder will contain a `logo.png` (unchanged from the original) and an `index.html` with the
following contents:

```html
<html>
  <head>
    <title>Boilerplate</title>
  </head>
  <body>
    <h1>Welcome!</h1>
    <img src="logo.png">
  </body>
</html>
```

You can also run Boilerplate non-interactively, which is great for automation:

```
boilerplate \
  --template-url /home/ubuntu/website-boilerplate \
  --output-folder /home/ubuntu/website-output \
  --non-interactive \
  --var Title="Boilerplate Example" \
  --var WelcomeText="Welcome!" \
  --var ShowLogo="true"

Generating /home/ubuntu/website-output/index.html
Copying /home/ubuntu/website-output/logo.png
```

Of course, Boilerplate can be used to generate any type of project, and not just HTML, so check out the
[examples](/examples) folder for more examples and the [Working with Boilerplate](#working-with-boilerplate) section
for full documentation.

## Install

Download the latest binary for your OS [here](https://github.com/gruntwork-io/boilerplate/releases/latest).

You can find older versions on the [Releases Page](https://github.com/gruntwork-io/usage-patterns/releases).

## Features

1. **Interactive mode**: Boilerplate interactively prompts the user for a set of variables defined in a
   `boilerplate.yml` file and makes those variables available to your project templates during copying.
1. **Non-interactive mode**: Variables can also be set non-interactively, via command-line options, so that
   Boilerplate can be used in automated settings (e.g. during automated tests).
1. **Flexible templating**: Boilerplate uses [Go Template](https://golang.org/pkg/text/template) for templating,
   which gives you the ability to do formatting, conditionals, loops, and call out to Go functions. It also includes
   helpers for common tasks such as loading the contents of another file.
1. **Variable types**: Boilerplate variables support types, so you have first-class support for strings, ints, bools,
1. **Validations**: Boilerplate provides a set of validations for a given variable that user input must satisfy.
1. **Variable presentation order**: Boilerplate allows you to define the relative presentation order of a set of variables.
   lists, maps, and enums.
1. **Scripting**: Need more power than static templates and variables? Boilerplate includes several hooks that allow
   you to run arbitrary scripts.
1. **Cross-platform**: Boilerplate is easy to install (it's a standalone binary) and works on all major platforms (Mac,
   Linux, Windows).

## Working with boilerplate

When you run Boilerplate, it performs the following steps:

1. Read the `boilerplate.yml` file in the folder specified by the `--template-url` option to find all defined
   varaibles.
1. Gather values for the variables from any `--var` and `--var-file` options that were passed in and prompting the user
   for the rest (unless the `--non-interactive` flag is specified).
1. Copy each file from `--template-url` to `--output-folder`, running each non-binary file through the Go
   Template engine with the map of variables as the data structure.

Learn more about boilerplate in the following sections:

1. [Boilerplate command line options](#boilerplate-command-line-options)
1. [The boilerplate.yml file](#the-boilerplate.yml-file)
1. [Variables](#variables)
1. [Dependencies](#dependencies)
1. [Hooks](#hooks)
1. [Partials](#partials)
1. [Skip Files](#skip-files)
1. [Templates](#templates)
1. [Validations](#validations)
1. [Variable order](#variable-ordering)
1. [Alternative Template Engines (EXPERIMENTAL)](#alternative-template-engines-experimental)
1. [Template helpers](#template-helpers)

#### Boilerplate command line options

The `boilerplate` binary supports the following options:

* `--template-url URL` (required): Generate the project from the templates in `URL`. This can be a local path, or a
  [go-getter](https://github.com/hashicorp/go-getter) compatible URL for remote templates (e.g.,
  `git@github.com:gruntwork-io/boilerplate.git//examples/for-learning-and-testing/include?ref=master`).
* `--output-folder` (required): Create the output files and folders in `FOLDER`.
* `--non-interactive` (optional): Do not prompt for input variables. All variables must be set via `--var` and
  `--var-file` options instead.
* `--var NAME=VALUE` (optional): Use `NAME=VALUE` to set variable `NAME` to `VALUE`. May be specified more than once.
* `--var-file FILE` (optional): Load variable values from the YAML file `FILE`. May be specified more than once.
* `--missing-key-action ACTION` (optional): What to do if a template looks up a variable that is not defined. Must
  be one of: `invalid` (render the text "<no value>"), `zero` (render the zero value for the variable), or `error`
  (return an error and exit immediately). Default: `error`.
* `--missing-config-action ACTION` (optional): What to do if a template folder does not have a `boilerplate.yml` file.
  Must be one of: `exit` (return an error and exit immediately) or `ignore` (log a warning and process the template
  folder without any variables). Default: `exit`.
* `--disable-hooks`: If this flag is set, no hooks will execute.
* `--disable-shell`: If this flag is set, no `shell` helpers will execute. They will instead return the text "replace-me".
* `--help`: Show the help text and exit.
* `--version`: Show the version and exit.

Some examples:

Generate a project in ~/output from the templates in ~/templates:

```
boilerplate --template-url ~/templates --output-folder ~/output
```

Generate a project in ~/output from the templates in ~/templates, using variables passed in via the command line:

```
boilerplate --template-url ~/templates --output-folder ~/output --var "Title=Boilerplate" --var "ShowLogo=false"
```

Generate a project in ~/output from the templates in ~/templates, using variables read from a file:

```
boilerplate --template-url ~/templates --output-folder ~/output --var-file vars.yml
```

Generate a project in ~/output from the templates in this repo's `include` example dir, using variables read from a file:

```
boilerplate --template-url "git@github.com:gruntwork-io/boilerplate.git//examples/for-learning-and-testing/include?ref=master" --output-folder ~/output --var-file vars.yml
```


#### The boilerplate.yml file

The `boilerplate.yml` file is used to configure `boilerplate`. The file is optional. If you don't specify it, you can
still use Go templating in your templates so long as you specify the `--missing-config-action ignore` option, but no
variables or dependencies will be available.

`boilerplate.yml` uses the following syntax:

```yaml
required_version: <VERSION_CONSTRAINT>

variables:
  - name: <NAME>
    description: <DESCRIPTION>
    type: <TYPE>
    options:
      - <CHOICE>
      - <CHOICE>
    default: <DEFAULT>
    reference: <NAME>

dependencies:
  - name: <DEPENDENCY_NAME>
    template-url: <FOLDER>
    output-folder: <FOLDER>
    skip: <CONDITION>
    dont-inherit-variables: <BOOLEAN>
    variables:
      - name: <NAME>
        description: <DESCRIPTION>
        type: <TYPE>
        default: <DEFAULT>

hooks:
  before:
    - command: <CMD>
      args:
        - <ARG>
      env:
        <KEY>: <VALUE>
      skip: <CONDITION>
  after:
    - command: <CMD>
      args:
        - <ARG>
      env:
        <KEY>: <VALUE>
      skip: <CONDITION>

partials:
  - <GLOB>
  - <GLOB>
```

Here's an example:

```yaml
variables:
  - name: Description
    description: Enter the description of this template

  - name: Version
    description: Enter the version number that will be used by the docs dependency

  - name: Title
    description: Enter the title for the dependencies example

  - name: WelcomeText
    description: Enter the welcome text used by the website dependency

  - name: ShowLogo
    description: Should the webiste show the logo (true or false)?
    type: bool
    default: true

dependencies:
  - name: docs
    template-url: ../docs
    output-folder: ./docs
    variables:
      - name: Title
        description: Enter the title of the docs page

  - name: website
    template-url: ../website
    output-folder: ./website
    variables:
      - name: Title
        description: Enter the title of the website

skip_files:
  - path: .ignore-me
  - path: subfolder/README.md
    if: {{ not .ShowLogo }}

engines:
  - path: subfolder/foo.json.jsonnet
    template_engine: jsonnet

partials:
  - ../html/*.html
  - ../css/*.css
  - ../other/somefile.html
```

**Required Version**: A version constraint string in the same format [as
Terraform](https://www.terraform.io/language/expressions/version-constraints#version-constraint-syntax) that can be used
to specify what versions of Boilerplate are supported by the config.

**Variables**: A list of objects (i.e. dictionaries) that define variables. Each variable may contain the following
keys:

* `name` (Required): The name of the variable.
* `description` (Optional): The description of the variable. `boilerplate` will show this description to the user when
  prompting them for a value.
* `type` (Optional): The type of the variable. Must be one of: `string`, `int`, `float`, `bool`, `map`, `list`, `enum`.
  If unspecified, the default is `string`.
* `options` (Optional): If the `type` is `enum`, you can specify a list of valid options. Each option must be a string.
* `default` (Optional): A default value for this variable. The user can just hit ENTER at the command line to use the
  default value, if one is provided. If running Boilerplate with the `--non-interactive` flag, the default is
  used for this value if no value is provided via the `--var` or `--var-file` options.
* `reference` (Optional): The name of another variable whose value should be used for this one.

See the [Variables](#variables) section for more info.

**Dependencies**: A list of objects (i.e. dictionaries) that define other `boilerplate` templates to execute before
executing the current one. Each dependency may contain the following keys:

* `name` (Required): A unique name for the dependency.
* `template-url` (Required): Run `boilerplate` on the templates in this folder. This path is relative to the
  current template.
* `output-folder` (Required): Create the output files and folders in this folder. This path is relative to the output
  folder of the current template.
* `skip` (Optional): Skip this dependency if this condition, which can use Go templating syntax and
  boilerplate variables, evaluates to the string `true`. This is useful to conditionally enable or disable
  dependencies.
* `dont-inherit-variables` (Optional): By default, any variables already set as part of the current `boilerplate.yml`
  template will be reused in the dependency, so that the user is not prompted multiple times for the same variable. If
  you set this option to `false`, then the variables from the parent template will not be reused.
* `variables`: If a dependency contains a variable of the same name as a variable in the root `boilerplate.yml` file,
  but you want the dependency to get a different value for the variable, you can specify overrides here. Note that these
  variables will not influence the boilerplate prompts.
* `var_files`: If you want to set the variables for the dependency based on a var file, you can provide a list of var
  file paths to include. The path is assumed to be relative to the current boilerplate.yml, but it is recommended to use
  the `templateFolder` helper function to be explicit. Note that the order of preference for variables are (top most
  have highest precedence - aka override all):
    - Top level variables set on the CLI.
    - Var files set on the dependency.
    - Defaults set on dependency variables (the `variables` field of the dependency).
    - Defaults set on root variables.
    - Defaults set within the dependency boilerplate config.

See the [Dependencies](#dependencies) section for more info.

**Partials**: Use *partials* to include reusable templates. Partials are defined using a list of glob patterns.

* Globs are matched using [the Go `filepath.Glob` function](https://golang.org/pkg/path/filepath/#Glob)
* In the event of a template name collision (e.g. multiple templates are defined with the same name), the last one wins.

See the [Partials](#partials) section for more info.

**Skip Files**: Use *skip_files* to specify files in the template folder that should not be rendered. The `path` field
is the relative path from the template folder root (where the `boilerplate.yml` file is defined) of the file that should
be excluded. You can conditionally skip the file using the `if` field, which should either be a YAML boolean value
(`true` or `false`), or Go templating syntax (which can use the boilerplate variables) that evaluates to the string
values `"true"` or `"false"`.

See the [Skip Files](#skip-files) section for more info.

**Engines**: Use *engines* to specify files in the template folder that should be rendered with an alternative
templating engine. The `path` field is the relative path from the template folder root (where the `boilerplate.yml` file
is defined) of the file that should use the alternative template engine.

See the [Alternative Template Engines (EXPERIMENTAL)](#alternative-template-engines-experimental) section for more info.

**Hooks**: Boilerplate provides hooks to execute arbitrary shell commands. There are two types of hooks:

* `before` (Optional): A list of scripts to execute before any template rendering has started.
* `after` (Optional): A list of scripts to execute after all template rendering has completed.

See the [Hooks](#hooks) section for more info.

#### Variables

You must provide a value for every variable defined in `boilerplate.yml`, or project generation will fail. There are
four ways to provide a value for a variable:

1. `--var` option(s) you pass in when calling boilerplate. Example:
   `boilerplate --var Title=Boilerplate --var ShowLogo=false`. To specify a complex type like a map or a list on the
   command-line, use YAML syntax (preferably the shorthand variety to keep it a one-liner). For example
   `--var foo='{key: "value"}' --var bar='["a", "b", "c"]'`. If you want to specify the value of a
   variable for a specific dependency, use the `<DEPENDENCY_NAME>.<VARIABLE_NAME>` syntax. For example:
   `boilerplate --var Description='Welcome to my home page!' --var about.Description='About Us' --var ShowLogo=false`.
1. `--var-file` option(s) you pass in when calling boilerplate. Example: `boilerplate --var-file vars.yml`. The vars
   file must be a simple YAML file that defines key, value pairs, where the key is the name of a variable (or
   `<DEPENDENCY_NAME>.<VARIABLE_NAME>` for a variable in a dependency) and the value is the value to set for that
   variable. Example:

   ```yaml
   Title: Boilerplate
   ShowLogo: false
   Description: Welcome to my home page!
   about.Description: Welcome to my home page!
   ExampleOfAMap:
     key1: value1
     key2: value2
   ExampleOfAList:
     - value1
     - value2
   ```
1. Manual input. If no value is specified via the `--var` or `--var-file` flags, Boilerplate will interactively prompt
   the user to provide a value. Note that the `--non-interactive` flag disables this functionality.
1. Defaults defined in `boilerplate.yml`. The final fallback is the optional `default` that you can include as part of
   the variable definition in `boilerplate.yml`.

Note that variables can reference other variables using Go templating syntax:

```yaml
variables:
  - name: Foo
    default: foo

  - name: Bar
    default: "{{ .Foo }}-bar"
```

If you rendered `{{ .Bar }}` with the variables above, you would get `foo-bar`. Note that this will always
return a string. If you want to reference another variable of a non-string type (e.g. a list), use the `reference`
keyword:

```yaml
variables:
  - name: Foo
    type: list
    default:
      - 1
      - 2
      - 3

  - name: Bar
    type: list
    reference: Foo
```

In the example above, the `Bar` variable will be set to the same (list) value as `Foo`.

#### Dependencies

Specifying dependencies within your `boilerplate.yml` files allows you to chain multiple `boilerplate` templates
together. This allows you to create more complicated projects from simpler pieces.

Note the following:

* Recursive dependencies: Dependencies can include other dependencies. For example, the `boilerplate.yml` in folder A
  can include folder B in its `dependencies` list, the `boilerplate.yml` in folder B can include folder C in its
  `dependencies` list, and so on.
* Inheriting variables: You can define all your common variables in the root `boilerplate.yml` and any variables with
  the same name in the `boilerplate.yml` files of your `dependencies` list will reuse those variables instead of
  prompting the user for the same value again.
* Variable conflicts: Sometimes, two dependencies use a variable of the same name, but you want them to have different
  values. To handle this use case, you can define custom `variable` blocks for each dependency and `boilerplate` will
  prompt you for each of those variables separately from the root ones. You can also use the
  `<DEPENDENCY_NAME>.<VARIABLE_NAME>` syntax as the name of the variable with the `-var` flag and inside of a var file
  to provide a value for a variable in a dependency.
* Interpolation: You may use interpolation in the `template-url` and `output-folder` parameters of your
  dependencies. This allows you to use specify the paths to your template and output folders dynamically.
* Conditional dependencies: You can enable or disable a dependency using the `skip` parameter, which supports Go
  templating syntax and boilerplate variables. If the `skip` parameter evaluates to the string `true`, the
  dependency will be skipped; otherwise, it will be rendered. Example:

    ```yaml
    variables:
      - name: Foo
        type: bool

      - name: Bar
        type: bool

    dependencies:
      - name: conditional-dependency-example
        template-url: ../foo
        output-folder: foo
        # Skip this dependency if both .Foo and .Bar are set to true
        skip: "{{ and .Foo .Bar }}"
    ```

#### Hooks

You can specify `hooks` in `boilerplate.yml` to tell Boilerplate to execute arbitrary shell commands.

Note the following:

* The `before` hook allows you to run scripts before Boilerplate has started rendering.
* The `after` hook allows you to run scripts after Boilerplate has finished rendering.
* Each hook consists of a `command` to execute (required), a list of `args` to pass to that command (optional), and
  a map of environment variables in `env` to set for the command (optional). Example:

    ```yaml
    before:
      - command: echo
        args:
          - Hello
          - World
        env:
          FOO: BAR
    ```
* You can use Go templating syntax in both `command`, `args`, and `env`. For example, you can pass Boilerplate
  variables to your scripts as follows:

    ```yaml
    before:
      - command: foo.sh
        args:
          - {{ .SomeVariable }}
          - {{ .AnotherVariable }}
    ```
* Boilerplate runs your `command` with the working directory set to the `--template-url` option.
* `skip` (Optional): Skip this hook if this condition, which can use Go templating syntax and
  boilerplate variables, evaluates to the string `true`. This is useful to conditionally enable or disable
  dependencies.
* For an alternative way to execute commands, see the `shell` helper in [template helpers](#template-helpers).

#### Skip Files

You can specify files that should be excluded from the rendered output using the `skip_files` section in
`boilerplate.yml`. This is most useful when you have templates that need to conditionally exclude files from the
rendered folder list.

The `skip_files` section is a list of objects with the fields `path`, `not_path`, and `if`, where one of `path` or
`not_path` is required. When `path` is set, all files that match the `path` attribute will be skipped, while when
`not_path` is set, all files that DO NOT match the `not_path` attribute are skipped (in other words, only paths that
match `not_path` are kept).

`if` can be used to conditionally skip a file from the template folder, and it defaults to `true`. That is, when `if` is
omitted, the file at the path is always excluded from the output. Note that `path` and `not_path` are always the
relative path from the template root.

All three attributes (`path`, `not_path`, and `if`) support Go templating syntax with access to boilerplate
[variables](#variables) and [template helpers](#template-helpers).

Consider the following boilerplate template folder:

```
.
├── boilerplate.yml
├── BOILERPLATE_README.md
└── docs
    ├── README_WITH_ENCRYPTION.md
    └── README_WITHOUT_ENCRYPTION.md
```

Suppose that you wanted to conditionally select which README to render based on some variable. You can use `skip_files`
to implement this logic:

```yaml
variables:
  - name: UseEncryption
    type: bool

skip_files:
  - path: "BOILERPLATE_README.md"
  - path: "docs/README_WITH_ENCRYPTION.md"
    if: "{{ not .UseEncryption }}"
  - path: "docs/README_WITHOUT_ENCRYPTION.md"
    if: "{{ .UseEncryption }}"
  - not_path: "docs/**/*"
    if: "{{ .DocsOnly }}"
```

This will:

- Always skip rendering `BOILERPLATE_README.md`.
- Skip rendering `docs/README_WITHOUT_ENCRYPTION.md` if `UseEncryption` is set to `true`.
- Skip rendering `docs/README_WITH_ENCRYPTION.md` if `UseEncryption` is set to `false`.
- If `DocsOnly` is set to `true`, only render the `docs` folder.

For a more concise specification, you can use glob syntax in the `path` to match multiple paths in one entry:

```yaml
skip_files:
  - path: "docs/**/*"
```


#### Templates

Boilerplate puts all the variables into a Go map where the key is the name of the variable and the value is what
the user provided. It then starts copying files from the `--template-url` into the `--output-folder`, passing each
non-binary file through the [Go Template](https://golang.org/pkg/text/template) engine, with the variable map as a data structure.

For example, if you had a variable called `Title` in your `boilerplate.yml` file, then you could access that variable
in any of your templates using the syntax `{{.Title}}`. You can also use Go template syntax to do
if-statements, for loops, and use the provided [template helpers](#template-helpers).

You can even use Go template syntax and boilerplate variables in the names of your files and folders. For example, if
you were using `boilerplate` to generate a Java project, your template folder could contain the path
`com/{{.PackageName}}/MyFactory.java`. If you run `boilerplate` against this template folder and enter
"gruntwork" as the `PackageName`, you'd end up with the file `com/gruntwork/MyFactory.java`.

#### Validations

Boilerplate allows you to specify a set of validations when defining a variable. When a user is prompted for a variable that has 
validations defined, their input must pass all defined validations. If the user's input does not pass all validations, they'll be 
presented with real-time feedback on exactly which rules their submission is failing. Once a user's submission passes all defined
validations, Boilerplate will accept their submitted value.

Here's an example prompt for a variable with validations that shows how invalid submissions are handled: 

![Example Boilerplate real-time validation](./docs/bp-validation.png)

Here's an example demonstating how to specify validations when defining your variables: 

```yaml
variables:
  - name: CompanyName
    description: Enter the name of your organization. 
    default: ""
    type: string
    validations:
      - required
      - length-5-22
      - alphanumeric
```
This `boilerplate.yml` snippet defines a variable, `CompanyName` which: 
* Must be supplied by the user. No empty or nil values will be accepted.
* Must have a length between 5 and 22 characters
* Must contain only alphanumeric characters (no special characters)

**Currently supported validations**

Boilerplate uses the [`go-ozzo/ozzo-validation` library](https://github.com/go-ozzo/ozzo-validation). The following validations are currently supported: 

 - "required" - field cannot be empty
 - "length-{min-int}-{max-int}" - field must be between ${min-int} and ${max-int} characters in length
 - "url" - field must be a valid URL
 - "email" - field must be a valid email address
 - "alpha" - field must contain English letters only
 - "digit" - field must contain digits only
 - "countrycode2" - field must be an ISO3166 Alpha 2 Country code
 - "semver" - field must be a valid semantic version

#### Variable Ordering

Boilerplate allows you to define the relative order in which a set of variables should be presented to the user when prompting
for human input. 

Here's an example demonstrating how to define the relative order of a set of variables: 

```yaml
variables: 
  - name: WebsiteURL
    order: 0
    description: Enter the URL to your homepage
 - name: ImagePath: 
   order: 1
   description: Enter the full filepath to your logo image 
 - name: ProfileName
   order: 2
   description: Enter the display name for your user

```

#### Alternative Template Engines (EXPERIMENTAL)

Boilerplate has experimental support for the following alternative template engines:

- [jsonnet](#jsonnet)

To specify an alternative template engine, you can use the `engines` directive to provide a path glob that matches the
files that should be fed through the alternative engine. For example, the following boilerplate configuration makes it
so that any file with the `.jsonnet` extension will be fed through the jsonnet template engine:

```yaml
engines:
  - path: "**/*.jsonnet"
    template_engine: jsonnet
```

Note that alternative template engines are currently only supported for processing individual files, and can not be used
for parsing boilerplate directives in the config file or directory names.

See below for more information on each of the template engines supported:

**IMPORTANT**: Support for template helpers are limited to the go templating engine at this time. Some limited functions
may be available depending on the template engine. See the information for the templating engine to know which functions
are supported.

##### Jsonnet

[Jsonnet](https://jsonnet.org/) is a data template engine optimized for generating json data. Unlike go templating,
jsonnet has many features that make it more friendly to write such as:

- [Imports](https://jsonnet.org/learning/tutorial.html#imports)
- [Functions](https://jsonnet.org/learning/tutorial.html#functions)
- [Better error handling](https://jsonnet.org/learning/tutorial.html#errors)
- Editor support

When boilerplate processes jsonnet templates, the variables are passed through as a [Top Level
Argument](https://jsonnet.org/learning/tutorial.html#parameterize-entire-config) under the name `boilerplateVars`. This
means that every jsonnet template must be defined in a way to handle the TLA argument. For example:

```jsonnet
function(boilerplateVars) {
  person: {
    name: boilerplateVars.Name,
  },
}
```

Boilerplate will also make available the following helpers as [external
variables](https://jsonnet.org/learning/tutorial.html#parameterize-entire-config):

- `templateFolder`: Set to the value of the template working dir. This is the same as what the `templateFolder`
  boilerplate helper function returns.
- `outputFolder`: Set to the value of the output folder where the templates are rendered. This is the same as what the
  `outputFolder` boilerplate helper function returns.

While the `jsonnet` template engine does not support the boilerplate helper functions, it does have access to the
[Jsonnet standard library](https://jsonnet.org/ref/stdlib.html). You can also import any `libsonnet` library in your
jsonnet template, including those installed with [jsonnet-bundlery](https://github.com/jsonnet-bundler/jsonnet-bundler).

Note that to ensure you can have editor assistance while modifying jsonnet files, the jsonnet template engine in
boilerplate will strip the extension suffix `.jsonnet` from the output file path. E.g., if your template folder
contained:

```
.
├── boilerplate.yml
└── data.json.jsonnet
```

The output folder will be:

```
.
└── data.json
```


#### Template helpers

Your templates have access to all the standard functionality in [Go Template](https://golang.org/pkg/text/template/),
including conditionals, loops, and functions.

Additionally, boilerplate ships with [sprig](https://github.com/Masterminds/sprig) (version `3.2.1`), the standard
library of template functions. You can view all the functions available in sprig
[here](http://masterminds.github.io/sprig/). Note that there are some differences for some functions due to backwards
compatibility. Take a look at [Deprecated helpers](#deprecated-helpers).

Boilerplate also includes several custom helpers that you can access that enhance the functionality of sprig:

* `snippet <PATH> [NAME]`: Returns the contents of the file at `PATH` as a string. If you specify the second argument,
  `NAME`, only the contents of the snippet with that name will be returned. A snippet is any text in the file
  surrounded by a line on each side of the format "boilerplate-snippet: NAME" (typically using the comment syntax for
  the language). For example, here is how you could define a snippet named "foo" in a Java file:

   ```java
   String str = "this is not part of the snippet";

   // boilerplate-snippet: foo
   String str2 = "this is part of the snippet";
   return str2;
   // boilerplate-snippet: foo
   ```
* `include <PATH> <VARIABLES>`: Returns the contents of the file at `PATH` after rendering it through the templating
  engine with the provided variables, as a string (unlike `snippet`, which returns the contents of the file verbatim).
  Use `.` to pass the current variables to the included template. E.g:
  ```
  {{ include "../source-template.snippet" . }}
  ```
* `replaceOne OLD NEW`: Replace the first occurrence of `OLD` with `NEW`. This is a literal replace, not regex.
* `replaceAll OLD NEW`: Replace all occurrences of `OLD` with `NEW`. This is a literal replace, not regex.
* `roundInt FLOAT`: Round `FLOAT` to the nearest integer. E.g. 1.5 becomes 2.
* `ceilInt FLOAT`: Round up `FLOAT` to the nearest integer. E.g. 1.5 becomes 2.
* `floorInt FLOAT`: Round down `FLOAT` to the nearest integer. E.g. 1.5 becomes 1.
* `dasherize STRING`: Convert `STRING` to a lower case string separated by dashes. E.g. "foo Bar baz" becomes
   "foo-bar-baz".
* `camelCaseLower STRING`: Convert `STRING` to a camel case string where the first letter is lower case. E.g.
   "foo Bar baz" becomes "fooBarBaz".
* `plus NUM NUM`: Add the two numbers. Unlike [add in sprig](http://masterminds.github.io/sprig/math.html#add), this
  supports float.
* `minus NUM NUM`: Subtract the two numbers. Unlike [sub in sprig](http://masterminds.github.io/sprig/math.html#sub),
  this supports float.
* `times NUM NUM`: Multiply the two numbers. Unlike [mul in sprig](http://masterminds.github.io/sprig/math.html#mul),
  this supports float.
* `divide NUM NUM`: Divide the two numbers. Unlike [div in sprig](http://masterminds.github.io/sprig/math.html#div),
  this supports float.
* `numRange START END INCREMENT`: Generate a slice from START to END, incrementing by INCREMENT. This provides a simple
  way to do a for-loop over a range of numbers.
* `keysSorted MAP`: Return a slice that contains all the keys in the given MAP in alphanumeric sorted order. Use the
  built-in Go template helper `.index` to look up these keys in the map.
* `shell CMD ARGS...`: Execute the given shell command, passing it the given args, and render whatever that command
  prints to stdout. The working directory for the command will be set to the directory of the template being rendered,
  so you can use paths relative to the file from which you are calling the `shell` helper. Any argument you pass of the
  form `ENV:KEY=VALUE` will be set as an environment variable for the command rather than an argument. For another way
  to execute commands, see [hooks](#hooks).
* `templateFolder`: Return the value of the template working dir. This is the value of the `--template-url` command-line
  option if local template, or the download dir if remote template. Useful for building relative paths.
* `outputFolder`: Return the value of the `--output-folder` command-line option. Useful for building relative paths.
* `envWithDefault NAME DEFAULT`: Render the value of environment variable `NAME`. If that environment variable is empty or not
  defined, render `DEFAULT` instead.
* `pathExists PATH`: Returns true if the given path exists on the file system.
* `templateIsDefined NAME`: Returns a boolean indicating if template called `NAME` is known. Use this to conditionally
  include one boilerplate template with another. Most often useful along with [partials](#partials).
* `toYaml`: Encodes an input variable as a YAML string. Similar to the `toJson` function in sprig.

#### Deprecated helpers

These helpers are deprecated. They are currently available for backwards compatibility, but may be removed in future
versions. Please use the alternative supported forms listed in the description.

* `downcase STRING`: Same functionality as [lower in sprig](http://masterminds.github.io/sprig/strings.html#lower).
* `upcase STRING`: Same functionality as [upper in sprig](http://masterminds.github.io/sprig/strings.html#upper).
* `capitalize STRING`: Same functionality as [title in sprig](http://masterminds.github.io/sprig/strings.html#title).
* `snakeCase STRING`: Same functionality as [snakecase in sprig](http://masterminds.github.io/sprig/strings.html#snakecase).
* `camelCase STRING`: Same functionality as [camelcase in sprig](http://masterminds.github.io/sprig/strings.html#camelcase).

The following functions overlap with sprig, but have different functionality. There is an equivalent function listed
above under a different name. These point to the boilerplate implementations for backwards compatibility. Please migrate
to using the new naming scheme, as they will be updated to use the sprig versions in future versions of boilerplate.

* `round`: In boilerplate, `round` returns the integer form as opposed to float. E.g `{{ round 123.5555 }}` will return
  `124`. The following supported alternative functions are available:
    - `roundFloat`: The sprig version of [round](http://masterminds.github.io/sprig/math.html#round), which supports
      arbitrary decimal rounding. E.g `{{ round 123.5555 3 }}` returns `123.556`. Note that `{{ round 123.5555 0 }}`
      returns `124.0`.
    - `roundInt`: Another name for the boilerplate version of `round`. Use this if you would like to keep old behavior.
* `ceil` and `floor`: In boilerplate, `ceil` and `floor` return integer forms as opposed to floats. E.g `{{ ceil
  1.1 }}` returns `2`, as opposed to `2.0` in the sprig version. The following supported alternative functions are
  available:
    - `ceilFloat` and `floorFloat`: The sprig version of [ceil](http://masterminds.github.io/sprig/math.html#ceil) and
      [floor](http://masterminds.github.io/sprig/math.html#floor).
    - `ceilInt` and `floorInt`: Another name for the boilerplate version `ceil` and `floor`. Use this if you would like to keep old behavior.
* `env`: In boilerplate, `env` supports returning a default value if the environment variable is not defined. The
  following supported alternative functions are available:
    - `readEnv`: The sprig version of [env](http://masterminds.github.io/sprig/os.html). This always returns empty
      string if the environment variable is undefined.
    - `envWithDefault`: Another name for the boilerplate version of `env`. Use this if you would like to keep old
      behavior.
* `keys`: In boilerplate, `keys` returns the keys of the map in sorted order. The following supported alternative
  functions are available:
    - `keysUnordered`: The sprig version of [keys](http://masterminds.github.io/sprig/dicts.html#keys). This returns the
      list of keys in no particular order, and there is no guarantee that the order of the returned list is consistent.
    - `keysSorted`: Another name for the boilerplate version of `keys`. Use this if you would like to keep old
      behavior.
* `replace`: In boilerplate, `replace` only replaces the first occurrence in the string, as opposed to all occurrences
  as in sprig. The following supported alternative functions are available:
    - `replaceAll`: The sprig version of [replace](http://masterminds.github.io/sprig/strings.html#replace).
    - `replaceOne`: Another name for the boilerplate version of `replace`. Use this if you would like to keep old
      behavior.
* `slice`: In boilerplate, `slice` returns a list of numbers in the provided range. E.g `{{ slice 1 5 1 }}`
  returns the list `[1, 2, 3, 4]`. The following supported alternative functions are available:
    - `sliceList`: The sprig version of [slice](http://masterminds.github.io/sprig/lists.html#slice), which returns the
      slice of the given list. E.g `{{ slice list n m }}` returns `list[n:m]`.
    - `numRange`: Another name for the boilerplate version of `slice`. Use this if you would like to keep old
      behavior.
* `trimPrefix` and `trimSuffix`: In boilerplate, `trimPrefix` and `trimSuffix` takes the base string first. E.g
  `{{ trimPrefix hello-world hello }}` returns `-world`. The following supported alternative functions are
  available:
    - `trimPrefixSprig` and `trimSuffixSprig`: The sprig version of
      [trimPrefix](http://masterminds.github.io/sprig/strings.html#trimPrefix) and
      [trimSuffix](http://masterminds.github.io/sprig/strings.html#trimSuffix). Unlike the boilerplate version, this
      takes the trim text first so that you can pipeline the trimming. E.g `{{ "hello-world" | trimPrefix "hello"
      }}` returns `{{ -world }}`.
    - `trimPrefixBoilerplate` and `trimSuffixBoilerplate`: Another name for the boilerplate versions of `trimPrefix`
      and `trimSuffix`. Use this if you would like to keep old behavior.

#### Partials

Partials help to keep templates DRY. Using partials, you can define templates in external files, and then use those templates over
and over again in other templates. Partials are common among templating engines, such as in [Hugo](https://gohugo.io/templates/partials/).

Let's start with a simple example. In an HTML document, we might want to have a common set of `meta` tags to reuse throughout our site:

```html
<html>
  <head>
    <meta charset="UTF-8">
    <meta name="author" content="Gruntwork">
  </head>
  <body>
    <h1>Welcome to this page!</h1>
    <img src="logo.png">
  </body>
</html>
```

Rather than add these tags in a `<head>` section within each and every file, we could define a partial, then reuse it throughout the site.

We define the header in `partials/header.html`:
```html
{{ define "header" }}
  <head>
    <meta charset="UTF-8">
    <meta name="author" content="Gruntwork">
  </head>
{{ end }}
```

Then we set up the structure in `templates/boilerplate.yml`:
```yaml
partials:
  - ../partials/*.html
```

In `templates/page.html`:
```html
<html>
{{ template "header" }}
  <body>
    <h1>Welcome to this page!</h1>
    <img src="logo.png">
  </body>
</html>
```

The contents of the `header` template will be rendered within `page.html` and any other page in which we call the header partial.

Let's see a slightly more involved example.

```html
<html>
  <head>
    <title>Welcome!</title>
  </head>
  <body>
    <h1>Welcome to this page!</h1>
    <img src="logo.png">
  </body>
</html>
```

The example above shows the HTML for a web page, with a title, a welcome message, and a logo. Now, if we wanted to have
another page showing a different title and body, we'd have to duplicate all of that content.

In the example below, we'll create a partial that represents the basic layout of the site, then reuse that layout for each page.
First, we create a directory structure to keep everything organized:

```
.
├── partials
│   └── layout.html
└── template
    ├── about
    │   ├── about.html
    │   └── boilerplate.yml
    └── root
        ├── boilerplate.yml
        └── index.html
```

In `partials/layout.html`, we create the basic page layout:

```
{{ define "basic_layout" }}
<html>
  <head>
    <title>{{ .Title }}</title>
  </head>
  <body>
    {{ template "body" . }}
  </body>
</html>
{{ end }}
```

Now, in each of the pages on the site, we can reuse this layout. For example, from the site's root, we want the welcome
page. We create the `boilerplate.yml` first:

```yaml
partials:
  - ../../partials/*.html

variables:
  - name: Title
    description: A title for the page.
    default: "Welcome!"
```

Now we can use the layout within our `index.html`:

```
{{- define "body" -}}
    <h1>This is index.html.</h1>
    <img src="logo.png">
{{- end -}}
{{- template "basic_layout" . -}}
```

When we run `boilerplate`, `basic_layout` template will be rendered with the contents of the `index.html`. Then we can
use the same layout for the about page, with its corresponding `boilerplate.yml`.

Contents of `about/boilerplate.yml`:

```yaml
partials:
  - ../../partials/*.html

variables:
  - name: Title
    description: A title for the page.
    default: "About"

```

`about/about.html`:

```
{{- define "body" -}}
    <h1>This is about.html.</h1>
{{- end -}}
{{- template "basic_layout" . -}}
```

Partials do not need to be located in a magic `partials` directory. Partials can be located anywhere and referred to using relative
paths.

The list of partials is a glob that can match multiple files. The content of all of the files that match the globs will be parsed
when rendering the final template. For example, you could match many HTML files at once with:

```
partials:
  - ../../html/*.html
  - ../../css/*.css
```

You can use the template definitions from any of the included partials throughout your templates.

You can use Go templating syntax in partial paths. For example, you can define a convenenience variable for a relative path to
make the paths slightly easier to read:

```
variables:
  - name: TemplatesRoot
    description: A convenience variable identify the relative path to the root of the templates directory.
    default: ../../../../
partials:
  - "{{ .TemplatesRoot }}/html/*.html"
  - "{{ .TemplatesRoot }}/css/*.css"
```

## Alternative project generators

Before creating Boilerplate, we tried a number of other project generators, but none of them met all of our
[requirements](#features). We list these alternatives below as a thank you to the creators of those projects for
inspiring many of the ideas in Boilerplate and so you can try out other projects if Boilerplate doesn't work for you:

* [yeoman](http://yeoman.io/): Project generator written in JavaScript. Good UI and huge community. However, very
  focused on generating web projects, and creating new generators is complicated and built around NPM. Not clear if
  it supports non-interactive mode.
* [plop](https://github.com/amwmedia/plop): Project generator written in JavaScript. Good UI and templating features.
  Does not support non-interactive mode.
* [giter8](https://github.com/foundweekends/giter8): Project generator written in Scala. A good option if you're
  already using the JVM (e.g. you're generating a Scala project), but too long of a startup time (due to all the jars
  it needs to download) if you're not.
* [plate](https://github.com/pilu/plate): Project generator written in Go. Many of the ideas for boilerplate came
  from this tool. Does not support non-interactive mode and has not been updated in 2+ years.
* [generator-yoga](https://github.com/raineorshine/generator-yoga): Project generator written in JavaScript. Supports
  templating for file copy and file contents. Interop with Yeoman. Does not support non-interactive mode.
* [hugo](https://gohugo.io/): Static website generator written in Go. Uses Go templates. Very focused on generating
  websites, HTML, themes, etc, and doesn't support interactive prompts, so it's not a great fit for other types of
  project generation.
* [jekyll](https://jekyllrb.com/): Static website generator written in Ruby. Huge community. Very focused on generating
  websites, HTML, themes, etc, and doesn't support interactive prompts, so it's not a great fit for other types of
  project generation.
* [play-doc](https://github.com/playframework/play-doc): Documentation generator used by the Play Framework that allows
  code snippets to be loaded from external files. Great for ensuring the code snippets in your docs are from files that
  are compiled and tested, but does not work as a general-purpose project generator.

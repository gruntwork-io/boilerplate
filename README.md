# Boilerplate

Boilerplate is a tool for generating files and folders ("boilerplate") from a set of templates.

Example use cases:

1. Create the scaffolding for a new project (e.g. like [html5boilerplate](https://html5boilerplate.com/))
1. Fill in boilerplate sections in your code files, such as including a legal disclaimer or license at the top of each
   source file, or updating a version number in a file after each build.
1. Embed code snippets from actual source files in documentation. Most READMEs have code copy/pasted into them and that
   code often has syntax errors or goes out of date. Now you can keep those code examples in normal source files which
   are built & tested, and embed parts of those files dynamically in your docs.

Note: the README for this project is generated using boilerplate! Check out the templates for it in the
[_docs folder](/_docs) and the build job configuration in [circle.yml](/circle.yml).

## Example: creating a new template

Create a folder called `website-boilerplate` and put a file called `boilerplate.yml` in it:

```yml
variables:
  - name: Title

  - name: WelcomeText
    prompt: Enter the welcome text for the website

  - name: ShowLogo
    prompt: Should the webiste show the logo (true or false)?
    default: true

```

This file defines 3 variables: `Title`, `WelcomeText`, and `ShowLogo`. When you run Boilerplate, it will prompt
the user (with the specified `prompt`, if specified) for each one.

Next, create an `index.html` in the `website-boilerplate` folder that uses these variables using [Go
Template](https://golang.org/pkg/text/template) syntax:

```html
<html>
  <head>
    <title>{{.Title}}</title>
  </head>
  <body>
    <h1>{{.WelcomeText}}</h1>
    {{if eq .ShowLogo "true"}}<img src="logo.png">{{end}}
  </body>
</html>
```

Copy an image into the `website-boilerplate` folder and call it `logo.png`.

Finally, run `boilerplate`, setting the `--template-folder` to `website-boilerplate` and `--output-folder` to the path
where you want the generated code to go:

```
boilerplate --template-folder /home/ubuntu/website-boilerplate --output-folder /home/ubuntu/website-output

Enter the value for 'Title': Boilerplate Example
Enter the welcome text for the website: Welcome!
Should the website show the logo? (y/n) (default: "y"): y

Generating /home/ubuntu/website-output/index.html
Copying /home/ubuntu/website-output/logo.png
```

Boilerplate copies any files from the `--template-folder` into the `--output-folder`, passing them through the
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
  --template-folder /home/ubuntu/website-boilerplate \
  --output-folder /home/ubuntu/website-output \
  --non-interactive \
  --var Title="Boilerplate Example" \
  --var WelcomeText="Welcome!" \
  --var ShowLogo="y"

Generating /home/ubuntu/website-output/index.html
Copying /home/ubuntu/website-output/logo.png
```

Of course, Boilerplate can be used to generate any type of project, and not just HTML, so check out the
[examples](/examples) folder for more examples and the [Working with Boilerplate](#working-with-boilerplate) section
for full documentation.

## Install

Download the latest binary for your OS here: [boilerplate v0.0.10](https://github.com/gruntwork-io/boilerplate/releases/tag/v0.0.10).

You can find older versions on the [Releases Page](https://github.com/gruntwork-io/usage-patterns/releases).

## Features

1. **Interactive mode**: Boilerplate interactively prompts the user for a set of variables defined in a
   `boilerplate.yml` file and makes those variables available to your project templates during copying.
1. **Non-interactive mode**: Variables can also be set non-interactively, via command-line options, so that
   Boilerplate can be used in automated settings (e.g. during automated tests).
1. **Flexible templating**: Boilerplate uses [Go Template](https://golang.org/pkg/text/template) for templating,
   which gives you the ability to do formatting, conditionals, loops, and call out to Go functions. It also includes
   helpers for common tasks such as loading the contents of another file.
1. **Cross-platform**: Boilerplate is easy to install (it's a standalone binary) and works on all major platforms (Mac,
   Linux, Windows).

## Working with boilerplate

When you run Boilerplate, it performs the following steps:

1. Read the `boilerplate.yml` file in the folder specified by the `--template-folder` option to find all defined
   varaibles.
1. Gather values for the variables from any `--var` and `--var-file` options that were passed in and prompting the user
   for the rest (unless the `--non-interactive` flag is specified).
1. Copy each file from `--template-folder` to `--output-folder`, running each non-binary file through the Go
   Template engine with the map of variables as the data structure.

Learn more about boilerplate in the following sections:

1. [Boilerplate command line options](#boilerplate-command-line-options)
1. [The boilerplate.yml file](#the-boilerplate.yml-file)
1. [Variables](#variables)
1. [Dependencies](#dependencies)
1. [Templates](#templates)
1. [Template helpers](#template-helpers)

#### Boilerplate command line options

The `boilerplate` binary supports the following options:

* `--template-folder FOLDER` (required): Generate the project from the templates in `FOLDER`.
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
* `--help`: Show the help text and exit.
* `--version`: Show the version and exit.

Some examples:

Generate a project in ~/output from the templates in ~/templates:

```
boilerplate --template-folder ~/templates --output-folder ~/output
```

Generate a project in ~/output from the templates in ~/templates, using variables passed in via the command line:

```
boilerplate --template-folder ~/templates --output-folder ~/output --var "Title=Boilerplate" --var "ShowLogo=false"
```

Generate a project in ~/output from the templates in ~/templates, using variables read from a file:

```
boilerplate --template-folder ~/templates --output-folder ~/output --var-file vars.yml
```

#### The boilerplate.yml file

The `boilerplate.yml` file is used to configure `boilerplate`. The file is optional. If you don't specify it, you can
still use Go templating in your templates so long as you specify the `--missing-config-action ignore` option, but no
variables or dependencies will be available.

`boilerplate.yml` uses the following syntax:

```yaml
variables:
  - name: <NAME>
    prompt: <PROMPT>
    default: <DEFAULT>

dependencies:
  - name: <DEPENDENCY_NAME>
    template-folder: <FOLDER>
    output-folder: <FOLDER>
    dont-inherit-variables: <BOOLEAN>
    variables:
      - name: <NAME>
        prompt: <PROMPT>
        default: <DEFAULT>
```

Here's an example:

```yaml
variables:
  - name: Title
    prompt: Enter a title for the home page

  - name: IncludeLogo
    prompt: Should we include a logo on the website?
    default: true

  - name: Description
    prompt: Enter a description for the home page
    default: Welcome to my home page!

dependencies:
  - name: about
    template-folder: ../about-us-page
    output-folder: ../about-us-page
    variables:
      - name: Description
        prompt: Enter a description for the about page
        default: About Us
```

**Variables**: A list of objects (i.e. dictionaries) that define variables. Each variable may contain the following
keys:

* `name` (Required): The name of the variable.
* `prompt` (Optional): The prompt to display to the user when asking them for a value. Default:
  "Enter a value for <VARIABLE_NAME>".
* `default` (Optional): A default value for this variable. The user can just hit ENTER at the command line to use the
  default value, if one is provided. If running Boilerplate with the `--non-interactive` flag, the default is
  used for this value if no value is provided via the `--var` or `--var-file` options.

See the [Variables](#variables) section for more info.

**Dependencies**: A list of objects (i.e. dictionaries) that define other `boilerplate` templates to execute before
executing the current one. Each dependency may contain the following keys:

* `name` (Required): A unique name for the dependency.
* `template-folder` (Required): Run `boilerplate` on the templates in this folder. This path is relative to the
  current template.
* `output-folder` (Required): Create the output files and folders in this folder. This path is relative to the output
  folder of the current template.
* `dont-inherit-variables` (Optional): By default, any variables already set as part of the current `boilerplate.yml`
  template will be reused in the dependency, so that the user is not prompted multiple times for the same variable. If
  you set this option to `false`, then the variables from the parent template will not be reused.
* `variables`: If a dependency contains a variable of the same name as a variable in the root `boilerplate.yml` file,
  but you want the dependency to get a different value for the variable, you can specify overrides here. `boilerplate`
  will include a separate prompt for variables defined under a `dependency`. You can also override the dependency's
  prompt and default values here.

See the [Dependencies](#dependencies) section for more info.

#### Variables

You must provide a value for every variable defined in `boilerplate.yml`, or project generation will fail. There are
four ways to provide a value for a variable:

1. `--var` option(s) you pass in when calling boilerplate. Example:
   `boilerplate --var Title=Boilerplate --var ShowLogo=false`. If you want to specify the value of a variable for a
   specific dependency, use the `<DEPENDENCY_NAME>.<VARIABLE_NAME>` syntax. For example:
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
   ```
1. Manual input. If no value is specified via the `--var` or `--var-file` flags, Boilerplate will interactively prompt
   the user to provide a value. Note that the `--non-interactive` flag disables this functionality.
1. Defaults defined in `boilerplate.yml`. The final fallback is the optional `default` that you can include as part of
   the variable definition in `boilerplate.yml`.

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

#### Templates

Boilerplate puts all the variables into a Go map where the key is the name of the variable and the value is what
the user provided. It then starts copying files from the `--template-folder` into the `--output-folder`, passing each
non-binary file through the [Go Template](https://golang.org/pkg/text/template) engine, with the variable map as a
data structure.

For example, if you had a variable called `Title` in your `boilerplate.yml` file, then you could access that variable
in any of your templates using the syntax `{{.Title}}`.

You can even use Go template syntax and boilerplate variables in the names of your files and folders. For example, if
you were using `boilerplate` to generate a Java project, your template folder could contain the path
`com/{{.PackageName}}/MyFactory.java`. If you run `boilerplate` against this template folder and enter
"gruntwork" as the `PackageName`, you'd end up with the file `com/gruntwork/MyFactory.java`.

#### Template helpers

Your templates have access to all the standard functionality in [Go Template](https://golang.org/pkg/text/template/),
including conditionals, loops, and functions. Boilerplate also includes several custom helpers that you can access:

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
* `downcase STRING`: Convert `STRING` to lower case. E.g. "FOO" becomes "foo".
* `upcase STRING`: Convert `STRING` to upper case. E.g. "foo" becomes "FOO".
* `capitalize STRING`: Capitalize the first letter of each word in `STRING`. E.g. "foo bar baz" becomes "Foo Bar Baz".
* `replace OLD NEW`: Replace the first occurrence of `OLD` with `NEW`. This is a literal replace, not regex.
* `replaceAll OLD NEW`: Replace all occurrences of `OLD` with `NEW`. This is a literal replace, not regex.
* `trim STRING`: Remove leading and trailing whitespace from `STRING`.
* `round FLOAT`: Round `FLOAT` to the nearest integer. E.g. 1.5 becomes 2.
* `ceil FLOAT`: Round up `FLOAT` to the nearest integer. E.g. 1.5 becomes 2.
* `floor FLOAT`: Round down `FLOAT` to the nearest integer. E.g. 1.5 becomes 1.
* `dasherize STRING`: Convert `STRING` to a lower case string separated by dashes. E.g. "foo Bar baz" becomes
   "foo-bar-baz".
* `snakeCase STRING`: Convert `STRING` to a lower case string separated by underscores. E.g. "foo Bar baz" becomes
   "foo_bar_baz".
* `camelCase STRING`: Convert `STRING` to a camel case string. E.g. "foo Bar baz" becomes "FooBarBaz".
* `camelCaseLower STRING`: Convert `STRING` to a camel case string where the first letter is lower case. E.g.
   "foo Bar baz" becomes "fooBarBaz".
* `plus NUM NUM`: Add the two numbers.
* `minus NUM NUM`: Subtract the two numbers.
* `times NUM NUM`: Multiply the two numbers.
* `divide NUM NUM`: Divide the two numbers.
* `mod INT INT`: Return the remainder of dividing the two numbers.
* `slice START END INCREMENT`: Generate a slice from START to END, incrementing by INCREMENT. This provides a simple
  way to do a for-loop over a range of numbers.

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

## TODO

1. Add support for using Go template syntax in the paths of files so they can be copied dynamically.
1. Add support for callbacks. For example, a `start` callback called just after Boilerplate gathers all variables but
   before it starts generating; an `file` callback called for each file and folder Boilerplate processes; and an
   `end` callback called after project generation is finished. The callbacks could be defined as simple shell
   commands in `boilerplate.yml`:

   ```yaml
   variables:
     - name: foo
     - name: bar

   callbacks:
     start: echo "Boilerplate is generating files to $2 from templates in $1"
     file: echo "Boilerplate is processing file $1"
     end: echo "Boilerplate finished generating files in $2 from templates in $1"
   ```
1. Support a list of `ignore` files in `boilerplate.yml` or even a `.boilerplate-ignore` file for files that should be
   skipped over while generating code.
1. Consider supporting different types for variables. Currently, all variables are strings, but there may be value
   in specifying a `type` in `boilerplate.yml`. Useful types: string, int, bool, float, list, map.
# Boilerplate

Boilerplate is a tool for generating files and folders ("boilerplate") from a set of templates. It interactively
prompts the user for a set of input variables which, along with lots of handy helpers, can be used by the templates
using [Go Template](https://golang.org/pkg/text/template) syntax.

Example use cases:

1. Create the scaffolding for a new project (e.g. like [html5boilerplate](https://html5boilerplate.com/))
1. Automate code generation for an existing project (e.g. like generating a new controller, view, and model in a Rails
   app).
1. Fill in boilerplate sections in your code files, such as including a legal disclaimer or license at the top of each
   source file, or updating a version number in a file after each build.
1. Embed code snippets from actual source files in documentation. Most READMEs have code copy/pasted into them and that
   code often has syntax errors or goes out of date. Now you can keep those code examples in normal source files which
   are built & tested, and embed parts of those files dynamically in your docs.

Note: the README for this project is generated using boilerplate! Check out the templates for it in the [_docs](/_docs)
folder and the build job configuration in [circle.yml](/circle.yml).

## Example: creating a new template

Create a folder called `website-boilerplate` and put a file called `boilerplate.yml` in it:

```json
{{snippet "../examples/website/boilerplate.yml" "all"}}
```

This file defines 3 variables: `Title`, `WelcomeText`, and `ShowLogo`. When you run Boilerplate, it will prompt
the user (with the specified `prompt`, if specified) for each one.

Next, create an `index.html` in the `website-boilerplate` folder that uses these variables using [Go
Template](https://golang.org/pkg/text/template) syntax:

```html
{{snippet "../examples/website/index.html" "all"}}
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
{{snippet "../test-fixtures/examples-expected-output/website/index.html" "all"}}
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

To install Boilerplate, just download the binary for your OS from the
[Releases Page](https://github.com/gruntwork-io/usage-patterns/releases).

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

1. Read the `boilerplate.yml` file in the folder specified by the `--template-folder` option and prompt the user
   for any variables defined in the `variables` section (unless the `--non-interactive` flag is specified).
1. Copy each file from `--template-folder` to `--output-folder`, passing each non-binary file through the Go
   Template engine, passing a map of user-specified variables as the data structure.

Learn more about each step below.

#### The boilerplate.yml file

The `boilerplate.yml` file uses the following syntax:

```yaml
variables:
  - name: <NAME>
    prompt: <PROMPT>
    default: <DEFAULT>

  - name: <NAME>
    prompt: <PROMPT>
    default: <DEFAULT>
```

The `variables` map can contain one or more variables, where the key is the variable name and the value is an object
with the following fields:

* `name` (Required): The name of the variable.
* `prompt` (Optional): The prompt to display to the user when asking them for a value. Default:
  "Enter a value for <VARIABLE_NAME>".
* `default` (Optional): A default value for this variable. The user can just hit ENTER at the command line to use the
  default value, if one is provided. If running Boilerplate with the `--non-interactive` flag, the default is
  used for this value if no value is provided via the `--var` option.

Note: the `boilerplate.yml` file is optional. If you don't specify it, you can still use Go templating in your
templates, but no variables will be available.

#### Templates

Boilerplate puts all the variables into a Go map where the key is the name of the variable and the value is what
the user provided. It then starts copying files from the `--template-folder` into the `--output-folder`, passing each
non-binary file through the [Go Template](https://golang.org/pkg/text/template) engine, with the variable map as a
data structure.

* If you had a variable called `Title` in your `boilerplate.yml` file, then you could access that variable in any
  of your templates using the syntax `&#123;&#123;.Title&#124;&#124;`.
* You can also use the Go template syntax in the names of the files and folders. For example, if you had a file
  called `website-&#123;&#123;.title&#124;&#124;.html` and the user set the variable `title` to "example", the
  resulting file would be called `website-example.html`.

#### Template helpers

Your templates have access to all the standard functionality in [Go Template](https://golang.org/pkg/text/template/),
including conditionals, loops, and functions. Boilerplate also includes several custom helpers that you can access:

1. `snippet <PATH> [NAME]`: Returns the contents of the file at `PATH` as a string. If you specify the second argument,
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
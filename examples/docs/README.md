# {{ .Title }}

This shows an example of how to use boilerplate to fill in parts of your documentation.

## Variables

Here is how you can use a variable:

The latest version of my app is {{.Version}}.

You could create a CI job that, for each release, regenerates your docs with the latest value of the `Version` variable
passed in using the `--var` option.

## Snippets

Here is how to use the `snippet` helper to embed files or parts of files from source code:

```html
{{snippet "../website/index.html"}}
```
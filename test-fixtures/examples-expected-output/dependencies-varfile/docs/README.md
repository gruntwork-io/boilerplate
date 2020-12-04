# I am vars for docs

This shows an example of how to use boilerplate to fill in parts of your documentation.

## Variables

Here is how you can use a variable:

The latest version of my app is 0.0.3.

You could create a CI job that, for each release, regenerates your docs with the latest value of the `Version` variable
passed in using the `--var` option.

## Snippets

Here is how to use the `snippet` helper to embed files or parts of files from source code:

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

## Arithmetic

Here is how you can use the arithmetic helpers to create a numbered list:

1. Item
2. Item
3. Item

And here is another way to do it using the slice helper:

1. Item
2. Item
3. Item

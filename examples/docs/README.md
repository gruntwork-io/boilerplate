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

## Arithmetic

Here is how you can use the arithmetic helpers to create a numbered list:

{{ with $index := "0" -}}
{{plus $index 1}}. Item
{{plus $index 2}}. Item
{{plus $index 3}}. Item
{{- end }}

And here is another way to do it using the slice helper:

{{ range $i := (slice 1 4 1) -}}
{{$i}}. Item
{{ end -}}


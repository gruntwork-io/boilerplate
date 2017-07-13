# Recursive variables

This shows an example of variables that reference other variables.

Foo = {{ .Foo }}
Bar = {{ .Bar }}
Baz = {{ .Baz }}
FooList = {{ range $index, $element := .FooList }}{{ if gt $index 0 }}, {{ end }}{{ $element }}{{ end }}
BarList = {{ range $index, $element := .BarList }}{{ if gt $index 0 }}, {{ end }}{{ $element }}{{ end }}
FooMap = {{ range $index, $key := (.FooMap | keys) }}{{ if gt $index 0 }}, {{ end }}{{ $key }}: {{ index $.FooMap $key }}{{ end }}
BarMap = {{ range $index, $key := (.BarMap | keys) }}{{ if gt $index 0 }}, {{ end }}{{ $key }}: {{ index $.BarMap $key }}{{ end }}
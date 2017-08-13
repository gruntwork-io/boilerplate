# Recursive variables

This shows an example of variables that reference other variables.

Foo = {{ .Foo }}
Bar = {{ .Bar }}
Baz = {{ .Baz }}
FooList = {{ range $index, $element := .FooList }}{{ if gt $index 0 }}, {{ end }}{{ $element }}{{ end }}
BarList = {{ range $index, $element := .BarList }}{{ if gt $index 0 }}, {{ end }}{{ $element }}{{ end }}
FooMap = {{ range $index, $key := (.FooMap | keys) }}{{ if gt $index 0 }}, {{ end }}{{ $key }}: {{ index $.FooMap $key }}{{ end }}
BarMap = {{ range $index, $key := (.BarMap | keys) }}{{ if gt $index 0 }}, {{ end }}{{ $key }}: {{ index $.BarMap $key }}{{ end }}
ListWithTemplates = {{ range $index, $element := .ListWithTemplates }}{{ if gt $index 0 }}, {{ end }}{{ $element }}{{ end }}
MapWithTemplates = {{ range $index, $key := (.MapWithTemplates | keys) }}{{ if gt $index 0 }}, {{ end }}{{ $key }}: {{ index $.MapWithTemplates $key }}{{ end }}
ListWithNestedMap = {{ range $index, $item := .ListWithNestedMap }}{{ if gt $index 0 }}, {{ end }}(name: {{ $item.name }}, value: {{ $item.value }}){{ end }}
MapWithNestedList = {{ range $index, $key := (.MapWithNestedList | keys) }}{{ if gt $index 0 }}, {{ end }}(key: {{ $key }}, value: {{ range $index2, $value := (index $.MapWithNestedList $key) }}{{ if gt $index2 0 }}, {{ end }}{{ $value }}{{ end }}){{ end }}
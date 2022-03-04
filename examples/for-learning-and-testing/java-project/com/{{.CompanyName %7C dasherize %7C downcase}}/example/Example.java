package com.{{.CompanyName | dasherize | downcase}}.example

import com.google.common.collect.ImmutableMap;

public class Example {
  public static final int EXAMPLE_INT_CONSTANT = {{.ExampleIntConstant}}
  public static final Map<> EXAMPLE_MAP = ImmutableMap.of({{range $index, $key := .ExampleMap | keys}}{{if gt $index 0}}, {{end}}"{{$key}}", "{{index $.ExampleMap $key}}"{{end}});

{{- if .IncludeEnum}}

  public enum ExampleEnum {
    {{range $index, $enumName := .EnumNames -}}
      {{if gt $index 0}}, {{end}}{{$enumName}}
    {{- end }}
  }
{{- end }}
}
package inputs

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractRefs_Simple(t *testing.T) {
	t.Parallel()

	refs, err := extractRefs("t", "Hello, {{ .Name }}!")
	require.NoError(t, err)
	assert.Equal(t, []string{"Name"}, sortedSetKeys(refs.vars))
	assert.Empty(t, refs.invocations)
}

func TestExtractRefs_MultipleRefsAndPipes(t *testing.T) {
	t.Parallel()

	refs, err := extractRefs("t", `Title: {{ .Title | upper }} / {{ .Subtitle }}{{ .Extra }}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"Extra", "Subtitle", "Title"}, sortedSetKeys(refs.vars))
}

func TestExtractRefs_Conditional(t *testing.T) {
	t.Parallel()

	refs, err := extractRefs("t", `{{ if .Show }}Shown {{ .Body }}{{ else }}Hidden {{ .Fallback }}{{ end }}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"Body", "Fallback", "Show"}, sortedSetKeys(refs.vars))
}

func TestExtractRefs_Range(t *testing.T) {
	t.Parallel()

	// `range .Items` references Items. The dot inside the range binds to each
	// item, not a top-level var, so it's correctly ignored.
	refs, err := extractRefs("t", `{{ range .Items }}- {{ . }}{{ end }} ({{ .Count }} total)`)
	require.NoError(t, err)
	assert.Equal(t, []string{"Count", "Items"}, sortedSetKeys(refs.vars))
}

func TestExtractRefs_With(t *testing.T) {
	t.Parallel()

	refs, err := extractRefs("t", `{{ with .Block }}{{ . }}{{ end }} after={{ .After }}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"After", "Block"}, sortedSetKeys(refs.vars))
}

func TestExtractRefs_TemplateInvocation(t *testing.T) {
	t.Parallel()

	refs, err := extractRefs("t", `{{ template "header" . }}body {{ .Body }}{{ template "footer" . }}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"Body"}, sortedSetKeys(refs.vars))
	assert.Equal(t, []string{"footer", "header"}, sortedSetKeys(refs.invocations))
}

func TestExtractRefs_DefineBlock(t *testing.T) {
	t.Parallel()

	// {{ define "x" }} introduces a named template; vars referenced inside it
	// should still be collected (we walk every parsed tree).
	refs, err := extractRefs("t", `{{- define "x" -}}{{ .Inside }}{{- end -}}{{ .Outside }}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"Inside", "Outside"}, sortedSetKeys(refs.vars))
}

func TestExtractRefs_ShellHelperIgnored(t *testing.T) {
	t.Parallel()

	// {{ shell "echo hi" }} should NOT add `shell` as a var; it's a function
	// call. The argument is a string literal, also no var.
	refs, err := extractRefs("t", `{{ shell "echo hi" }} and {{ .Real }}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"Real"}, sortedSetKeys(refs.vars))
}

func TestExtractRefs_BuiltinsExcluded(t *testing.T) {
	t.Parallel()

	refs, err := extractRefs("t", `{{ .__each__ }} {{ .This }} {{ .BoilerplateConfigVars }} {{ .Real }}`)
	require.NoError(t, err)
	// All the builtins should be filtered out.
	assert.Equal(t, []string{"Real"}, sortedSetKeys(refs.vars))
}

func TestExtractRefs_EmptyAndNoVars(t *testing.T) {
	t.Parallel()

	refs, err := extractRefs("t", `Just plain text, no template syntax.`)
	require.NoError(t, err)
	assert.Empty(t, refs.vars)
}

func TestExtractRefs_ParseError(t *testing.T) {
	t.Parallel()

	// Mismatched action delimiter: {{ never closes.
	_, err := extractRefs("t", `{{ .Broken `)
	require.Error(t, err)
}

func TestExpandPartialRefs_Transitive(t *testing.T) {
	t.Parallel()

	// A invokes B; B invokes C; C references X. After expansion, A should also
	// reference X.
	a := newTemplateRefs()
	a.invocations["B"] = struct{}{}

	b := newTemplateRefs()
	b.invocations["C"] = struct{}{}

	c := newTemplateRefs()
	c.vars["X"] = struct{}{}

	m := map[string]*templateRefs{"A": a, "B": b, "C": c}
	expandPartialRefs(m)

	assert.Equal(t, []string{"X"}, sortedSetKeys(m["A"].vars))
	assert.Equal(t, []string{"X"}, sortedSetKeys(m["B"].vars))
	assert.Equal(t, []string{"X"}, sortedSetKeys(m["C"].vars))
}

// sortedSetKeys returns the keys of m as a sorted slice. Used to make set
// comparisons deterministic in assertions.
func sortedSetKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

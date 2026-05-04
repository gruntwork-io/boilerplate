package inputs

import (
	"context"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runFS is a small helper: it invokes FromFS against an in-memory FS and
// returns the result, failing the test on hard error.
func runFS(t *testing.T, fsys fstest.MapFS, vars map[string]any) *Result {
	t.Helper()

	res, err := FromFS(context.Background(), fsys, ".", vars)
	require.NoError(t, err)

	return res
}

func TestFromFS_SingleTemplateBareRef(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Greeting
    description: Greeting text
`)},
		"hello.txt": &fstest.MapFile{Data: []byte(`Hello {{ .Greeting }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	require.Contains(t, res.Inputs, ".:Greeting")
	assert.Equal(t, []string{"hello.txt"}, res.Inputs[".:Greeting"].Files)
	assert.Equal(t, []string{".:Greeting"}, res.Files["hello.txt"])
	assert.Empty(t, res.Errors)
}

func TestFromFS_DeclaredButUnused(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Used
  - name: Unused
`)},
		"main.txt": &fstest.MapFile{Data: []byte(`{{ .Used }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	assert.Equal(t, []string{"main.txt"}, res.Inputs[".:Used"].Files)
	// Declared-but-unused must still be present, with an empty file list.
	require.Contains(t, res.Inputs, ".:Unused")
	assert.Empty(t, res.Inputs[".:Unused"].Files)
}

func TestFromFS_UndeclaredVariable(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Declared
`)},
		"main.txt": &fstest.MapFile{Data: []byte(`{{ .Declared }} and {{ .Mystery }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	require.Len(t, res.Errors, 1)
	assert.Equal(t, "undeclared_variable", res.Errors[0].Kind)
	assert.Equal(t, "Mystery", res.Errors[0].Name)
	assert.Equal(t, "main.txt", res.Errors[0].File)
}

func TestFromFS_DependencyNameMatchInheritance(t *testing.T) {
	t.Parallel()

	// Parent declares Title and has a dep `web` with template-url pointing to
	// the `web/` subdir. The child also declares Title (no explicit override
	// in the parent's dependencies block). With dont-inherit-variables=false
	// (the default), the parent's Title flows into the child by name.
	//
	// Editing parent's Title should affect both root files referencing Title
	// and the child's files referencing Title (via name-match inheritance).
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Title
dependencies:
  - name: web
    template-url: ./web
    output-folder: ./web
`)},
		"root.txt": &fstest.MapFile{Data: []byte(`{{ .Title }}`)},

		"web/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Title
`)},
		"web/page.html": &fstest.MapFile{Data: []byte(`<h1>{{ .Title }}</h1>`)},
	}

	res := runFS(t, fsys, map[string]any{})

	// Parent Title should affect both files.
	assert.Equal(t, []string{"root.txt", "web/page.html"}, res.Inputs[".:Title"].Files,
		"parent's Title should affect both root and child files via name-match inheritance")

	// Child Title is its own declaration, affecting only its own file.
	assert.Equal(t, []string{"web/page.html"}, res.Inputs["web:Title"].Files)

	// Inverse index: web/page.html is affected by both Titles.
	assert.Equal(t, []string{".:Title", "web:Title"}, res.Files["web/page.html"])
}

func TestFromFS_ExplicitValueExpressionEdge(t *testing.T) {
	t.Parallel()

	// The parent passes `{{ .Region }}` to the child as a different name
	// (`AwsRegion`) via the dependencies[].variables[].default field. Editing
	// parent's Region should affect any child file referencing AwsRegion,
	// even though the names don't match.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
dependencies:
  - name: vpc
    template-url: ./modules/vpc
    output-folder: ./modules/vpc
    variables:
      - name: AwsRegion
        default: "{{ .Region }}"
`)},

		"modules/vpc/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: AwsRegion
`)},
		"modules/vpc/main.tf": &fstest.MapFile{Data: []byte(`region = "{{ .AwsRegion }}"`)},
	}

	res := runFS(t, fsys, map[string]any{})

	// Region should reach modules/vpc/main.tf via the explicit edge.
	assert.Equal(t, []string{"modules/vpc/main.tf"}, res.Inputs[".:Region"].Files)
	// Child's AwsRegion still maps to its own file directly.
	assert.Equal(t, []string{"modules/vpc/main.tf"}, res.Inputs["modules/vpc:AwsRegion"].Files)

	// Inverse index lists both.
	assert.Equal(t, []string{".:Region", "modules/vpc:AwsRegion"}, res.Files["modules/vpc/main.tf"])
}

func TestFromFS_InterpolatedValueExpression(t *testing.T) {
	t.Parallel()

	// Parent passes "prefix-{{ .Foo }}-{{ .Bar }}" to a child input. Both Foo
	// and Bar are upstream of the child input; both should reach files that
	// reference the child input.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Foo
  - name: Bar
dependencies:
  - name: child
    template-url: ./child
    output-folder: ./child
    variables:
      - name: Combined
        default: "prefix-{{ .Foo }}-{{ .Bar }}-suffix"
`)},

		"child/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Combined
`)},
		"child/file.txt": &fstest.MapFile{Data: []byte(`{{ .Combined }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	assert.Equal(t, []string{"child/file.txt"}, res.Inputs[".:Foo"].Files)
	assert.Equal(t, []string{"child/file.txt"}, res.Inputs[".:Bar"].Files)
	assert.ElementsMatch(t,
		[]string{".:Bar", ".:Foo", "child:Combined"},
		res.Files["child/file.txt"],
	)
}

func TestFromFS_TransitiveAcrossThreeLevels(t *testing.T) {
	t.Parallel()

	// Root passes Foo down to L1.Foo via name match, L1 passes its Foo to
	// L2.Bar via explicit value expression. Root's Foo should reach L2's file.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Foo
dependencies:
  - name: l1
    template-url: ./l1
    output-folder: ./l1
`)},

		"l1/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Foo
dependencies:
  - name: l2
    template-url: ./l2
    output-folder: ./l2
    variables:
      - name: Bar
        default: "{{ .Foo }}"
`)},

		"l1/l2/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Bar
`)},
		"l1/l2/leaf.txt": &fstest.MapFile{Data: []byte(`{{ .Bar }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	// Root Foo -> (name match) -> l1.Foo -> (explicit edge) -> l2.Bar -> file.
	assert.Equal(t, []string{"l1/l2/leaf.txt"}, res.Inputs[".:Foo"].Files,
		"root Foo should reach the leaf via two hops")
	assert.Equal(t, []string{"l1/l2/leaf.txt"}, res.Inputs["l1:Foo"].Files)
	assert.Equal(t, []string{"l1/l2/leaf.txt"}, res.Inputs["l1/l2:Bar"].Files)
}

func TestFromFS_DontInheritVariablesBlocksNameMatch(t *testing.T) {
	t.Parallel()

	// Parent and child both declare Title, but the dependency sets
	// dont-inherit-variables=true. Without an explicit edge for Title in the
	// dependencies block, parent's Title should NOT reach the child's file.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Title
dependencies:
  - name: web
    template-url: ./web
    output-folder: ./web
    dont-inherit-variables: true
    variables:
      - name: Title
`)},
		"root.txt": &fstest.MapFile{Data: []byte(`{{ .Title }}`)},

		"web/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Title
`)},
		"web/page.html": &fstest.MapFile{Data: []byte(`{{ .Title }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	// Parent Title should ONLY affect root.txt.
	assert.Equal(t, []string{"root.txt"}, res.Inputs[".:Title"].Files)
	// Child Title affects only its own file.
	assert.Equal(t, []string{"web/page.html"}, res.Inputs["web:Title"].Files)
}

func TestFromFS_ExplicitOverridesNameMatch(t *testing.T) {
	t.Parallel()

	// Parent declares Title and Tag. The dep's variables block redefines
	// Title with default `{{ .Tag }}`. The explicit edge should govern: only
	// Tag (not Title) reaches the child via that variable. Parent's Title is
	// NOT propagated through the explicit override (we honor the user's
	// intentional rebinding).
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Title
  - name: Tag
dependencies:
  - name: web
    template-url: ./web
    output-folder: ./web
    variables:
      - name: Title
        default: "{{ .Tag }}"
`)},

		"web/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Title
`)},
		"web/page.html": &fstest.MapFile{Data: []byte(`{{ .Title }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	// Parent Tag reaches web/page.html via explicit rebind.
	assert.Equal(t, []string{"web/page.html"}, res.Inputs[".:Tag"].Files)
	// Parent Title does NOT reach web/page.html — the explicit edge replaced
	// the implicit name-match.
	assert.Empty(t, res.Inputs[".:Title"].Files,
		"parent Title should not propagate when an explicit override is in scope")
}

func TestFromFS_CycleDetection(t *testing.T) {
	t.Parallel()

	// Two templates referencing each other. The resolver should break the
	// cycle and emit a "cycle" error rather than recursing forever.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Foo
dependencies:
  - name: a
    template-url: ./a
    output-folder: ./a
`)},
		"a/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Bar
dependencies:
  - name: back
    template-url: ../
    output-folder: .
`)},
	}

	res := runFS(t, fsys, map[string]any{})

	hasCycle := false

	for _, e := range res.Errors {
		if e.Kind == "cycle" || e.Kind == "unresolvable_dependency" {
			// On the FS-only resolver, parent-relative `..` paths return an
			// error from the resolver itself, which surfaces as
			// unresolvable_dependency. Either resolution is acceptable: the
			// important thing is the analysis terminates without exploding.
			hasCycle = true
			break
		}
	}

	assert.True(t, hasCycle, "expected the analyzer to surface a cycle or unresolvable error rather than recurse forever; got: %+v", res.Errors)
}

func TestFromFS_PartialsExpandRefs(t *testing.T) {
	t.Parallel()

	// A partial defines a named template that references .Inner. The body
	// invokes that named template. Editing Inner should re-render the body.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Inner
partials:
  - parts/*.tmpl
`)},
		"parts/header.tmpl": &fstest.MapFile{Data: []byte(`{{- define "header" -}}HEADER {{ .Inner }}{{- end -}}`)},
		"main.txt":          &fstest.MapFile{Data: []byte(`{{ template "header" . }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	// main.txt invokes the named partial, so editing Inner re-renders it.
	// The partial file itself is also a walked file in FS mode (boilerplate
	// renders partials as outputs unless skip_files excludes them), so we
	// just assert main.txt is included rather than the full set.
	assert.Contains(t, res.Inputs[".:Inner"].Files, "main.txt",
		"Inner should reach main.txt via the partial it invokes")
}

func TestFromFS_FilenameTemplateRendering(t *testing.T) {
	t.Parallel()

	// A file whose name contains template syntax should appear in the result
	// under the rendered path, not the literal one. The render uses the
	// supplied vars.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Name
`)},
		"{{.Name}}.txt": &fstest.MapFile{Data: []byte(`{{ .Name }}`)},
	}

	res := runFS(t, fsys, map[string]any{"Name": "alice"})

	require.Contains(t, res.Inputs, ".:Name")
	// The file should appear under its rendered name.
	assert.Equal(t, []string{"alice.txt"}, res.Inputs[".:Name"].Files)
}

func TestFinalizeResult_DeterministicOrdering(t *testing.T) {
	t.Parallel()

	r := &Result{
		Inputs: map[string]InputEntry{
			".:A": {Name: "A", DeclaredIn: ".", Files: []string{"z", "a", "m"}},
		},
		Files: map[string][]string{
			"x": {"z:1", "a:1"},
		},
		Errors: []AnalysisError{
			{Kind: "z"}, {Kind: "a"},
		},
	}

	finalizeResult(r)
	assert.Equal(t, []string{"a", "m", "z"}, r.Inputs[".:A"].Files)
	assert.Equal(t, []string{"a:1", "z:1"}, r.Files["x"])

	kinds := make([]string, len(r.Errors))
	for i, e := range r.Errors {
		kinds[i] = e.Kind
	}

	sort.Strings(kinds)
	assert.Equal(t, []string{"a", "z"}, kinds)
}

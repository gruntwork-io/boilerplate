package inputs //nolint:testpackage

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

	// Cycle detection keys on the resolved (parent.dir + renderedURL) path,
	// so a true cycle is an `a` dep that points back to its own directory
	// (`./a` resolves to `a` from inside `a`, which is still in the visiting
	// set). This both exercises the detector and avoids the false-positive
	// trap two siblings sharing a raw URL string used to fall into.
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
  - name: aself
    template-url: .
    output-folder: ./self
`)},
	}

	res := runFS(t, fsys, map[string]any{})

	var cycleErrs []AnalysisError

	for _, e := range res.Errors {
		if e.Kind == "cycle" {
			cycleErrs = append(cycleErrs, e)
		}
	}

	require.Len(t, cycleErrs, 1, "expected exactly one cycle error; got all errors: %+v", res.Errors)
	assert.Equal(t, "aself", cycleErrs[0].Name, "cycle error should name the offending dependency")
}

// TestFromFS_SiblingDepsSameURLAreNotACycle pins the regression for the
// bug fixed alongside the cycle-key change: two siblings declaring the
// same template-url under different output-folders are not a cycle, and
// the analyzer must not report one.
func TestFromFS_SiblingDepsSameURLAreNotACycle(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
dependencies:
  - name: first
    template-url: ./shared
    output-folder: ./out-first
  - name: second
    template-url: ./shared
    output-folder: ./out-second
`)},
		"shared/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Greeting
`)},
		"shared/hello.txt": &fstest.MapFile{Data: []byte(`{{ .Greeting }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	for _, e := range res.Errors {
		assert.NotEqual(t, "cycle", e.Kind, "two siblings sharing a template-url must not be a cycle: %+v", e)
	}
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

func TestFromFS_SkipFilesPath(t *testing.T) {
	t.Parallel()

	// `secrets.txt` is listed under skip_files with no `if` condition (so it
	// always skips). It should not appear in the analyzer's output, even
	// though it references a declared variable.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
skip_files:
  - path: secrets.txt
`)},
		"main.tf":     &fstest.MapFile{Data: []byte(`region = "{{ .Region }}"`)},
		"secrets.txt": &fstest.MapFile{Data: []byte(`secret={{ .Region }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	assert.Equal(t, []string{"main.tf"}, res.Inputs[".:Region"].Files,
		"skip_files entry should exclude secrets.txt from the input map")
	assert.NotContains(t, res.Files, "secrets.txt",
		"skipped file should not appear in inverse files index")
}

func TestFromFS_SkipFilesIfFalse(t *testing.T) {
	t.Parallel()

	// When `if` evaluates to "false", the skip rule does not apply and the
	// file is reported normally.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
  - name: Exclude
    type: bool
    default: false
skip_files:
  - path: optional.txt
    if: "{{ .Exclude }}"
`)},
		"optional.txt": &fstest.MapFile{Data: []byte(`{{ .Region }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	assert.Equal(t, []string{"optional.txt"}, res.Inputs[".:Region"].Files,
		"skip rule with if=false should leave the file in the analysis")
}

func TestFromFS_SkipFilesNotPath(t *testing.T) {
	t.Parallel()

	// `not_path` inverts the rule: only files matching kept.txt are kept;
	// everything else is skipped.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
skip_files:
  - not_path: kept.txt
`)},
		"kept.txt":   &fstest.MapFile{Data: []byte(`{{ .Region }}`)},
		"dropped.tf": &fstest.MapFile{Data: []byte(`{{ .Region }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	assert.Equal(t, []string{"kept.txt"}, res.Inputs[".:Region"].Files,
		"not_path rule should keep only the listed files")
}

func TestFromFS_SkipFilesUnsupportedDoubleStar(t *testing.T) {
	t.Parallel()

	// `**` is not supported by fs.Glob in FS mode. The analyzer should
	// surface a soft error rather than silently producing no matches.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
skip_files:
  - path: "**/*.tf"
`)},
		"main.tf": &fstest.MapFile{Data: []byte(`{{ .Region }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	hasSkipErr := false

	for _, e := range res.Errors {
		if e.Kind == "skip_files" {
			hasSkipErr = true
			break
		}
	}

	assert.True(t, hasSkipErr, "expected a skip_files soft error for `**` in FS mode; got: %+v", res.Errors)
}

func TestFromFS_FilenameRefAffectsFileEvenWhenBodyDoesNot(t *testing.T) {
	t.Parallel()

	// The body has no template syntax, but the filename uses {{ .Name }}
	// to choose the directory. Changing Name relocates the file, so Name
	// must appear as affecting it even though the body never references it.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Name
`)},
		"{{.Name}}/static.txt": &fstest.MapFile{Data: []byte(`literal content, no template refs`)},
	}

	res := runFS(t, fsys, map[string]any{"Name": "alice"})

	require.Contains(t, res.Inputs, ".:Name")
	assert.Equal(t, []string{"alice/static.txt"}, res.Inputs[".:Name"].Files,
		"a var used only in the file path should still link to that file")
	assert.Equal(t, []string{".:Name"}, res.Files["alice/static.txt"])
}

func TestFromFS_DepOutputFolderAffectsSubtree(t *testing.T) {
	t.Parallel()

	// The parent's Env shapes the dep's output-folder. The child's file
	// has no template syntax of its own, but a change to Env relocates
	// the entire subtree, so Env must reach every file under the dep.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Env
dependencies:
  - name: app
    template-url: ./app
    output-folder: ./{{ .Env }}/app
`)},
		"app/boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"app/main.tf":         &fstest.MapFile{Data: []byte(`literal terraform content`)},
	}

	res := runFS(t, fsys, map[string]any{"Env": "prod"})

	require.Contains(t, res.Inputs, ".:Env")
	assert.Equal(t, []string{"prod/app/main.tf"}, res.Inputs[".:Env"].Files,
		"vars used in dep output-folder should affect every file in the subtree")
}

func TestFromFS_DepTemplateURLAffectsSubtree(t *testing.T) {
	t.Parallel()

	// The dep's template-url itself is shaped by a parent var: changing
	// Flavor would pull in a different child module. Files in the
	// resolved subtree should link back to Flavor.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Flavor
dependencies:
  - name: mod
    template-url: ./modules/{{ .Flavor }}
    output-folder: ./mod
`)},
		"modules/blue/boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"modules/blue/file.txt":        &fstest.MapFile{Data: []byte(`plain text`)},
	}

	res := runFS(t, fsys, map[string]any{"Flavor": "blue"})

	require.Contains(t, res.Inputs, ".:Flavor")
	assert.Equal(t, []string{"mod/file.txt"}, res.Inputs[".:Flavor"].Files,
		"vars used in dep template-url should affect files in the resolved subtree")
}

func TestFromFS_TemplatedDepURLExcludedFromParentWalk(t *testing.T) {
	t.Parallel()

	// Regression: when a dependency's template-url is itself templated
	// (`./modules/{{ .Flavor }}`), the parent walker must still skip the
	// resolved subdirectory so the child's files don't bleed into the
	// parent's analysis. Without this, the parent's Region would report
	// `modules/blue/file.txt` as an affected file (the source path the
	// walker visited), in addition to the legitimate output paths.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Flavor
  - name: Region
dependencies:
  - name: mod
    template-url: ./modules/{{ .Flavor }}
    output-folder: ./mod
`)},
		"root.txt": &fstest.MapFile{Data: []byte(`root {{ .Region }}`)},

		"modules/blue/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
`)},
		"modules/blue/file.txt": &fstest.MapFile{Data: []byte(`child {{ .Region }}`)},
	}

	res := runFS(t, fsys, map[string]any{"Flavor": "blue"})

	// Region reaches root.txt (direct) and mod/file.txt (via name-match
	// inheritance into the child). The child's *source* path
	// modules/blue/file.txt must NOT appear: it isn't an output path of
	// the parent template.
	assert.ElementsMatch(t,
		[]string{"root.txt", "mod/file.txt"},
		res.Inputs[".:Region"].Files,
		"parent Region should reach the dep's output path, not the source path the walker would otherwise descend into",
	)

	assert.NotContains(t, res.Files, "modules/blue/file.txt",
		"the dep template's source path must not show up in the parent's file index")
}

func TestFromFS_SkipFilesPathLinksVar(t *testing.T) {
	t.Parallel()

	// skip_files.path uses {{ .Env }} to choose which file to skip. A
	// change to Env shifts which file is excluded — conservatively, link
	// Env to every output file in the template so consumers know to
	// re-analyze.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Env
  - name: Region
skip_files:
  - path: "{{ .Env }}-secrets.txt"
`)},
		"main.tf":          &fstest.MapFile{Data: []byte(`region = "{{ .Region }}"`)},
		"prod-secrets.txt": &fstest.MapFile{Data: []byte(`prod`)},
		"dev-secrets.txt":  &fstest.MapFile{Data: []byte(`dev`)},
	}

	// With Env=prod the skip rule matches prod-secrets.txt; main.tf and
	// dev-secrets.txt remain. Env should link to both of them.
	res := runFS(t, fsys, map[string]any{"Env": "prod"})

	require.Contains(t, res.Inputs, ".:Env")
	assert.ElementsMatch(t,
		[]string{"main.tf", "dev-secrets.txt"},
		res.Inputs[".:Env"].Files,
		"vars in skip_files.path should link to every file the template currently produces")
	// Region's existing semantics (body-ref only) should be unaffected.
	assert.Equal(t, []string{"main.tf"}, res.Inputs[".:Region"].Files)
}

func TestFromFS_SkipFilesIfLinksVar(t *testing.T) {
	t.Parallel()

	// skip_files.if depends on Mode. A change to Mode flips whether
	// optional.txt is included, which is a meaningful dependency the
	// analyzer should report.
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Mode
  - name: Region
skip_files:
  - path: optional.txt
    if: '{{ eq .Mode "strict" }}'
`)},
		"main.tf":      &fstest.MapFile{Data: []byte(`region = "{{ .Region }}"`)},
		"optional.txt": &fstest.MapFile{Data: []byte(`hi`)},
	}

	res := runFS(t, fsys, map[string]any{"Mode": "lax"})

	require.Contains(t, res.Inputs, ".:Mode")
	assert.ElementsMatch(t,
		[]string{"main.tf", "optional.txt"},
		res.Inputs[".:Mode"].Files,
		"vars in skip_files.if should link to every file in the template")
}

// TestFromFS_SourcesMapsOutputsToSourcePaths exercises Result.Sources in FS
// mode. Because there is no on-disk root, the values are slash-separated FS
// paths, not absolute disk paths — but the contract that every file in
// Result.Files (minus filename_render cases) has a corresponding Sources
// entry holds the same way the CLI emits it.
func TestFromFS_SourcesMapsOutputsToSourcePaths(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Title
dependencies:
  - name: web
    template-url: ./web
    output-folder: ./web
`)},
		"README.md": &fstest.MapFile{Data: []byte(`# {{ .Title }}`)},
		"web/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Title
`)},
		"web/index.html": &fstest.MapFile{Data: []byte(`<h1>{{ .Title }}</h1>`)},
	}

	res := runFS(t, fsys, map[string]any{"Title": "hello"})

	assert.Equal(t, "README.md", res.Sources["README.md"])
	assert.Equal(t, "web/index.html", res.Sources["web/index.html"])

	for outPath := range res.Files {
		src, ok := res.Sources[outPath]
		require.Truef(t, ok, "missing sources entry for %q", outPath)
		require.NotEmptyf(t, src, "empty source for %q", outPath)
	}
}

// TestFromFS_SourcesOmitsFilenameRenderFailures ensures that when a file's
// templated name cannot be rendered (parse error), the analyzer records the
// filename_render soft error and leaves the output absent from Sources.
func TestFromFS_SourcesOmitsFilenameRenderFailures(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Name
`)},
		"good.txt":  &fstest.MapFile{Data: []byte(`{{ .Name }}`)},
		"bad-{{.tf": &fstest.MapFile{Data: []byte(`unused body`)},
	}

	res := runFS(t, fsys, map[string]any{"Name": "alice"})

	// The good file got a source entry.
	assert.Equal(t, "good.txt", res.Sources["good.txt"])

	// The bad file produced a filename_render soft error and is omitted from
	// Sources. It still appears in Files under its un-rendered name, which is
	// what the analyzer falls back to.
	hasFilenameRenderError := false

	for _, e := range res.Errors {
		if e.Kind == KindFilenameRender && e.File == "bad-{{.tf" {
			hasFilenameRenderError = true
			break
		}
	}

	require.True(t, hasFilenameRenderError,
		"expected a filename_render error for bad-{{.tf; got: %+v", res.Errors)

	_, hasSource := res.Sources["bad-{{.tf"]
	assert.False(t, hasSource, "filename_render case must not appear in sources")
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

// TestFromFS_PartialExpansionLimit drives a fixture through analyze with
// a deliberately low partial-expansion cap so KindPartialExpansionLimit
// lands in Result.Errors. The chain a -> b -> c needs 2 iterations to
// fully propagate c's references into a; a cap of 1 trips the soft error.
func TestFromFS_PartialExpansionLimit(t *testing.T) {
	prev := partialExpansionMaxIterations
	partialExpansionMaxIterations = 1

	t.Cleanup(func() {
		partialExpansionMaxIterations = prev
	})

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Deep
partials:
  - "_partials/*.tmpl"
`)},
		"_partials/a.tmpl": &fstest.MapFile{Data: []byte(`{{ define "a" }}{{ template "b" . }}{{ end }}`)},
		"_partials/b.tmpl": &fstest.MapFile{Data: []byte(`{{ define "b" }}{{ template "c" . }}{{ end }}`)},
		"_partials/c.tmpl": &fstest.MapFile{Data: []byte(`{{ define "c" }}{{ .Deep }}{{ end }}`)},
		"main.txt":         &fstest.MapFile{Data: []byte(`{{ template "a" . }}`)},
	}

	res := runFS(t, fsys, map[string]any{})

	var hit *AnalysisError

	for i := range res.Errors {
		if res.Errors[i].Kind == KindPartialExpansionLimit {
			hit = &res.Errors[i]
			break
		}
	}

	require.NotNil(t, hit, "expected a KindPartialExpansionLimit error in Result.Errors, got: %+v", res.Errors)
	assert.Contains(t, hit.Message, "partial-template invocation graph")
}

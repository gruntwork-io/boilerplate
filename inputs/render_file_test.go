package inputs //nolint:testpackage

import (
	"context"
	"io/fs"
	"maps"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/config"
)

// renderFile invokes RenderFileFromFS with a background context and ".".
// It is the render counterpart to runFS (analyzer test helper).
//
// The deps index is synthesised from the in-memory fixture: every test
// fsys places dep files at the same path the parent's `template-url`
// names (relative join), so the index just reflects that layout. Real
// bundles get this index from the producer side at bundle time; tests
// that bypass the producer have to reproduce the layout themselves, and
// this helper does that bookkeeping so individual test cases stay terse.
func renderFile(t *testing.T, fsys fstest.MapFS, outputPath string, vars map[string]any) (string, error) {
	t.Helper()

	idx := buildDepsIndexFromFS(t, fsys, ".")

	return RenderFileFromFS(context.Background(), fsys, ".", outputPath, vars, idx)
}

// buildDepsIndexFromFS walks fsys and parses every boilerplate.yml's
// `dependencies:` block, returning a depsIndex whose BundlePath entries
// match the natural fsys layout (parent dir + raw template-url). It is
// intentionally simple: it does not attempt to render templated
// template-urls — the existing render_file_test fixtures use literal
// relative paths, so the raw URL is the right key.
//
// Remote URLs are skipped (matching the producer's behavior), so tests
// that exercise the ErrDependencyNotInBundle path against a remote URL
// see an empty entry under the parent — which lookupDep treats as
// "not in bundle" — exactly as a real CLI bundle would behave.
func buildDepsIndexFromFS(t *testing.T, fsys fstest.MapFS, rootPath string) map[string][]ResolvedDep {
	t.Helper()

	idx := map[string][]ResolvedDep{}
	collectDepsIndex(t, fsys, rootPath, map[string]any{}, idx)

	return idx
}

// collectDepsIndex walks the dep tree starting at parentDir, threading
// the scope of variable defaults already applied so a child's
// for_each_reference can resolve against list variables declared in
// the parent's boilerplate.yml. Mirrors collectBundle's recursion shape
// (and the runtime's variable inheritance), but stays minimal: it just
// builds the depsIndex that RenderFileFromFS consumes — no Files map,
// no notes, no cycle tracking (test fixtures don't cycle).
func collectDepsIndex(t *testing.T, fsys fstest.MapFS, parentDir string, parentScope map[string]any, idx map[string][]ResolvedDep) {
	t.Helper()

	cfgPath := path.Join(parentDir, config.BoilerplateConfigFile)

	data, readErr := fs.ReadFile(fsys, cfgPath)
	if readErr != nil {
		// Fixtures may omit a boilerplate.yml at this level; that's
		// fine — there are no deps to collect, the renderer will get
		// the same shape via its own loadConfig.
		return
	}

	cfg, parseErr := config.ParseBoilerplateConfig(data)
	if parseErr != nil {
		// Test fixtures with deliberately bad config still need to
		// reach the renderer; treat parse failure here as "no deps".
		return
	}

	// Apply this template's variable defaults so deeper for_each_reference
	// lookups can see list variables declared in *this* boilerplate.yml.
	scope, _ := applyConfigDefaults(context.Background(), templateLocation{fsys: fsys, dir: parentDir}, cfg, parentScope, true)

	parentKey := parentDir
	if parentKey == "" || parentKey == "." {
		parentKey = "."
	}

	for i := range cfg.Dependencies {
		dep := &cfg.Dependencies[i]
		url := strings.TrimSpace(dep.TemplateURL)

		if url == "" || strings.Contains(url, "://") {
			continue
		}

		bundlePath := path.Clean(path.Join(parentDir, url))
		if bundlePath == "" || bundlePath == "." {
			continue
		}

		// Match producer semantics: deps that didn't resolve at
		// bundle time aren't in the index. The renderer treats
		// missing-from-index siblings as "skip and continue" and
		// surfaces ErrOutputNotProduced when no sibling owns the
		// path.
		if _, statErr := fs.Stat(fsys, bundlePath); statErr != nil {
			continue
		}

		forEachItems, _ := resolveForEach(context.Background(), templateLocation{fsys: fsys, dir: parentDir}, dep, scope)

		iterations := []string{""}
		isForEach := len(forEachItems) > 0

		if isForEach {
			iterations = forEachItems
		}

		for _, item := range iterations {
			renderVars := scope
			if isForEach {
				renderVars = make(map[string]any, len(scope)+1)
				maps.Copy(renderVars, scope)
				renderVars[eachVarName] = item
			}

			renderedOutputFolder, ofErr := renderForAnalysis(context.Background(), "", dep.OutputFolder, renderVars)
			if ofErr != nil {
				renderedOutputFolder = dep.OutputFolder
			}

			entry := ResolvedDep{
				Name:         dep.Name,
				BundlePath:   bundlePath,
				OutputFolder: strings.TrimSpace(renderedOutputFolder),
			}

			if isForEach {
				entry.Each = item
			}

			idx[parentKey] = append(idx[parentKey], entry)
		}

		collectDepsIndex(t, fsys, bundlePath, scope, idx)
	}
}

// TestRenderFileFromFS_RootTemplateNoDeps is the simplest case: a single
// boilerplate.yml at the root produces one file whose template references
// only user-supplied vars. No defaults, no deps.
func TestRenderFileFromFS_RootTemplateNoDeps(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Greeting
`)},
		"hello.txt": &fstest.MapFile{Data: []byte(`Hello, {{ .Greeting }}!`)},
	}

	got, err := renderFile(t, fsys, "hello.txt", map[string]any{"Greeting": "world"})
	require.NoError(t, err)
	assert.Equal(t, "Hello, world!", got)
}

// TestRenderFileFromFS_DepScopedDefaultApplied is the load-bearing case
// from the spec: a child dep declares a variable with a `default:` value
// that the user never supplies. A file in the dep references that
// variable. Warm dispatch must pick up the dep's default — otherwise the
// runbooks "AddAdditionalCommonVariables" failure mode reappears.
func TestRenderFileFromFS_DepScopedDefaultApplied(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: OrgNamePrefix
dependencies:
  - name: common
    template-url: ./common
    output-folder: ./common
`)},

		"common/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: OrgNamePrefix
  - name: AddAdditionalCommonVariables
    type: bool
    default: false
`)},
		"common/common.hcl": &fstest.MapFile{Data: []byte(
			`org = "{{ .OrgNamePrefix }}"
extra = {{ .AddAdditionalCommonVariables }}`)},
	}

	got, err := renderFile(t, fsys, "common/common.hcl", map[string]any{
		"OrgNamePrefix": "acme",
	})
	require.NoError(t, err)
	assert.Equal(t, "org = \"acme\"\nextra = false", got)
}

// TestRenderFileFromFS_UserVarsOverrideDepDefault verifies precedence:
// when both the dep declares a default and the user supplies a value for
// the same name, the user's value wins.
func TestRenderFileFromFS_UserVarsOverrideDepDefault(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
dependencies:
  - name: common
    template-url: ./common
    output-folder: ./common
`)},

		"common/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Mode
    default: lax
`)},
		"common/m.txt": &fstest.MapFile{Data: []byte(`mode={{ .Mode }}`)},
	}

	// User overrides Mode at the root scope; the dep inherits Mode by
	// name-match, so the user's value should win over the dep's default.
	got, err := renderFile(t, fsys, "common/m.txt", map[string]any{"Mode": "strict"})
	require.NoError(t, err)
	assert.Equal(t, "mode=strict", got)
}

// TestRenderFileFromFS_TemplatedDefaultsRenderAgainstScope verifies that
// defaults containing template expressions (the
// "{{ .OrgNamePrefix }}:Team" case from the spec) render against the
// merged scope when the dep is entered.
func TestRenderFileFromFS_TemplatedDefaultsRenderAgainstScope(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: OrgNamePrefix
dependencies:
  - name: dep
    template-url: ./dep
    output-folder: ./dep
`)},

		"dep/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: OrgNamePrefix
  - name: TeamName
    default: "{{ .OrgNamePrefix }}:Team"
`)},
		"dep/out.txt": &fstest.MapFile{Data: []byte(`team={{ .TeamName }}`)},
	}

	got, err := renderFile(t, fsys, "dep/out.txt", map[string]any{
		"OrgNamePrefix": "acme",
	})
	require.NoError(t, err)
	assert.Equal(t, "team=acme:Team", got)
}

// TestRenderFileFromFS_ExplicitEdgeFromParent verifies the runtime
// behavior where a parent's `dependencies[].variables[].default` value
// rebinds a child variable. Here the parent maps its Region to the child's
// AwsRegion via `default: "{{ .Region }}"`.
func TestRenderFileFromFS_ExplicitEdgeFromParent(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
dependencies:
  - name: vpc
    template-url: ./vpc
    output-folder: ./vpc
    variables:
      - name: AwsRegion
        default: "{{ .Region }}"
`)},

		"vpc/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: AwsRegion
`)},
		"vpc/main.tf": &fstest.MapFile{Data: []byte(`region = "{{ .AwsRegion }}"`)},
	}

	got, err := renderFile(t, fsys, "vpc/main.tf", map[string]any{"Region": "us-west-2"})
	require.NoError(t, err)
	assert.Equal(t, `region = "us-west-2"`, got)
}

// TestRenderFileFromFS_UnknownOutputPath verifies the ErrOutputNotProduced
// sentinel for outputs that no file in the dep tree generates.
func TestRenderFileFromFS_UnknownOutputPath(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"only.txt":        &fstest.MapFile{Data: []byte(`x`)},
	}

	_, err := renderFile(t, fsys, "does-not-exist.txt", map[string]any{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputNotProduced, "expected ErrOutputNotProduced, got: %v", err)
}

// TestRenderFileFromFS_MissingLocalDep verifies the renderer's behavior
// when a dep is declared but the producer didn't include it in the
// bundle (e.g., local-path resolution failed at bundle time). The
// renderer's contract is to skip the missing sibling and surface
// ErrOutputNotProduced once no remaining sibling owns the requested
// output. Dispatchers route to cold render off either signal — the
// bundle's notes[] array carries the human-readable reason.
func TestRenderFileFromFS_MissingLocalDep(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
dependencies:
  - name: missing
    template-url: ./missing
    output-folder: ./missing
`)},
		// Nothing under ./missing.
	}

	_, err := renderFile(t, fsys, "missing/anything.txt", map[string]any{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputNotProduced, "expected ErrOutputNotProduced for a dep the producer didn't bundle, got: %v", err)
}

// TestRenderFileFromFS_RemoteDepURL verifies remote URLs are omitted
// from the bundle's depsIndex (the producer surfaces them as a separate
// bundle note instead). The renderer then has no sibling to descend
// into and returns ErrOutputNotProduced.
func TestRenderFileFromFS_RemoteDepURL(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
dependencies:
  - name: remote
    template-url: git::https://example.com/repo.git
    output-folder: ./remote
`)},
	}

	_, err := renderFile(t, fsys, "remote/file.txt", map[string]any{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputNotProduced, "expected ErrOutputNotProduced for a remote dep absent from the bundle, got: %v", err)
}

// TestRenderFileFromFS_DynamicFilename verifies that a file whose name
// itself contains unrenderable template syntax surfaces as
// ErrDynamicFilename when its un-rendered path could plausibly match
// outputPath. The consumer is expected to fall back to cold render.
func TestRenderFileFromFS_DynamicFilename(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Name
`)},
		// Unclosed {{ — won't parse at filename-render time.
		"bad-{{.tf": &fstest.MapFile{Data: []byte(`unused`)},
	}

	// Query for the literal un-rendered name. With Name supplied, the
	// filename's `{{` is still malformed, so the analyzer / renderer can't
	// resolve which output path it should produce.
	_, err := renderFile(t, fsys, "bad-{{.tf", map[string]any{"Name": "alice"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDynamicFilename, "expected ErrDynamicFilename, got: %v", err)
}

// TestRenderFileFromFS_SkipFilesExcluded verifies that requesting a file
// the dep's skip_files rule would exclude returns ErrSkipFilesExcluded
// rather than rendering or returning ErrOutputNotProduced.
func TestRenderFileFromFS_SkipFilesExcluded(t *testing.T) {
	t.Parallel()

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

	_, err := renderFile(t, fsys, "secrets.txt", map[string]any{"Region": "us-east-1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSkipFilesExcluded, "expected ErrSkipFilesExcluded, got: %v", err)
}

// TestRenderFileFromFS_SkippedDepNotEntered verifies the runtime semantic
// that a dep marked `skip: "true"` is treated as nonexistent — its files
// must not be discoverable via warm dispatch.
func TestRenderFileFromFS_SkippedDepNotEntered(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
dependencies:
  - name: optional
    template-url: ./optional
    output-folder: ./optional
    skip: "true"
`)},
		"optional/boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"optional/x.txt":           &fstest.MapFile{Data: []byte(`x`)},
	}

	_, err := renderFile(t, fsys, "optional/x.txt", map[string]any{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputNotProduced, "expected ErrOutputNotProduced (because skipped), got: %v", err)
}

// TestRenderFileFromFS_HooksAreNotExecuted verifies the spec contract that
// hooks must never execute under warm dispatch. We can't directly observe
// a missing hook execution, but we can configure a hook whose command
// would fail catastrophically (rm /) and assert the render succeeds anyway
// — proving the hook code path is never entered. Using a noop "echo" would
// pass for both behaviors; the catastrophic command is the test.
func TestRenderFileFromFS_HooksAreNotExecuted(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Greeting
hooks:
  before:
    - command: /bin/false
    - command: rm
      args: ["-rf", "/"]
`)},
		"out.txt": &fstest.MapFile{Data: []byte(`{{ .Greeting }}`)},
	}

	got, err := renderFile(t, fsys, "out.txt", map[string]any{"Greeting": "hi"})
	require.NoError(t, err)
	assert.Equal(t, "hi", got)
}

// TestRenderFileFromFS_MatchesFullRenderForTransitiveFixture is the
// load-bearing equivalence test promised by the spec: re-rendering one
// file via warm dispatch must produce byte-identical output to a full
// boilerplate template run for that file.
//
// We don't have access to `boilerplate template` from this package
// directly, so we recreate the expected output by composing the runtime
// substitutions inline. The point is to verify the dep-scoped variable
// resolution lands the same bytes a full render would.
func TestRenderFileFromFS_MatchesFullRenderForTransitiveFixture(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
  - name: ProjectName
dependencies:
  - name: vpc
    template-url: ./modules/vpc
    output-folder: ./modules/vpc
    variables:
      - name: AwsRegion
        default: "{{ .Region }}"
`)},
		"README.md": &fstest.MapFile{Data: []byte(
			`# {{ .ProjectName }}

Stack deployed in region {{ .Region }}.
`)},

		"modules/vpc/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: AwsRegion
`)},
		"modules/vpc/main.tf": &fstest.MapFile{Data: []byte(`provider "aws" {
  region = "{{ .AwsRegion }}"
}
`)},
	}

	// Warm render of the dep file.
	gotVPC, err := renderFile(t, fsys, "modules/vpc/main.tf", map[string]any{
		"Region":      "us-west-2",
		"ProjectName": "demo",
	})
	require.NoError(t, err)
	assert.Equal(t, "provider \"aws\" {\n  region = \"us-west-2\"\n}\n", gotVPC)

	// Warm render of a root file.
	gotREADME, err := renderFile(t, fsys, "README.md", map[string]any{
		"Region":      "us-west-2",
		"ProjectName": "demo",
	})
	require.NoError(t, err)
	assert.Equal(t, "# demo\n\nStack deployed in region us-west-2.\n", gotREADME)
}

// TestRenderFileFromFS_PartialsResolveInWarmRender verifies that a
// template body invoking a named partial via `{{ template "name" }}`
// resolves correctly when the partial is in the bundle. This is the
// fs.FS-based equivalent of the runtime's ParseGlob behavior.
func TestRenderFileFromFS_PartialsResolveInWarmRender(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Title
partials:
  - parts/*.tmpl
`)},
		"parts/header.tmpl": &fstest.MapFile{Data: []byte(`{{- define "header" -}}HEADER:{{ .Title }}{{- end -}}`)},
		"main.txt":          &fstest.MapFile{Data: []byte(`{{ template "header" . }}`)},
	}

	got, err := renderFile(t, fsys, "main.txt", map[string]any{"Title": "hello"})
	require.NoError(t, err)
	assert.Equal(t, "HEADER:hello", got)
}

// TestRenderFileFromFS_NilDepsIndexRejectsOldBundles asserts the
// hard-error contract: a bundle that arrives without the Dependencies
// field at all (depsIndex == nil) is treated as too old. The runbooks
// dispatcher catches the sentinel and routes to cold render rather than
// silently falling back to the old (broken) re-render path.
func TestRenderFileFromFS_NilDepsIndexRejectsOldBundles(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"hello.txt":       &fstest.MapFile{Data: []byte(`hi`)},
	}

	_, err := RenderFileFromFS(context.Background(), fsys, ".", "hello.txt", map[string]any{}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDependencyNotInBundle,
		"a nil depsIndex must produce ErrDependencyNotInBundle so consumers route to cold; got: %v", err)
}

// TestRenderFileFromFS_EmptyDepsIndexAllowsDeplessTemplates pins the
// other side of the rule: an empty (non-nil) map is the valid encoding
// for a bundle with no deps. Warm dispatch must still succeed.
func TestRenderFileFromFS_EmptyDepsIndexAllowsDeplessTemplates(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`variables: [{name: Name}]`)},
		"hello.txt":       &fstest.MapFile{Data: []byte(`hi, {{ .Name }}`)},
	}

	got, err := RenderFileFromFS(context.Background(), fsys, ".", "hello.txt", map[string]any{"Name": "alice"}, map[string][]ResolvedDep{})
	require.NoError(t, err)
	assert.Equal(t, "hi, alice", got)
}

// TestLookupDeps covers the lookup helper's observable cases: an empty
// list, a name that doesn't match any sibling, a name that matches once
// (a plain dep), and a name that matches multiple times (a for_each dep,
// where the producer emits one entry per iteration). Together they pin
// the "skip and continue" contract the dispatcher relies on for deps the
// producer didn't bundle and the new "iterate every match" contract the
// renderer relies on to feed __each__ into each iteration.
func TestLookupDeps(t *testing.T) {
	t.Parallel()

	t.Run("nil_map_returns_empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, lookupDeps(nil, ".", "anything"))
	})

	t.Run("empty_parent_bucket_returns_empty", func(t *testing.T) {
		t.Parallel()

		idx := map[string][]ResolvedDep{".": {}}
		assert.Empty(t, lookupDeps(idx, ".", "anything"))
	})

	t.Run("name_mismatch_returns_empty", func(t *testing.T) {
		t.Parallel()

		idx := map[string][]ResolvedDep{".": {{Name: "vpc", BundlePath: "_deps/vpc"}}}
		assert.Empty(t, lookupDeps(idx, ".", "common"))
	})

	t.Run("single_match_returns_one_entry", func(t *testing.T) {
		t.Parallel()

		idx := map[string][]ResolvedDep{".": {
			{Name: "vpc", BundlePath: "_deps/vpc", OutputFolder: "./vpc"},
			{Name: "other", BundlePath: "_deps/other"},
		}}
		got := lookupDeps(idx, ".", "vpc")
		require.Len(t, got, 1)
		assert.Equal(t, "_deps/vpc", got[0].BundlePath)
	})

	t.Run("multiple_matches_preserve_producer_order", func(t *testing.T) {
		t.Parallel()

		// Two entries with the same Name + BundlePath model a for_each
		// dep's per-iteration entries. Producer ordering is the source
		// of truth — the consumer iterates in declared order.
		idx := map[string][]ResolvedDep{".": {
			{Name: "envs", BundlePath: "_deps/envs", OutputFolder: "out/dev", Each: "dev"},
			{Name: "envs", BundlePath: "_deps/envs", OutputFolder: "out/prod", Each: "prod"},
		}}
		got := lookupDeps(idx, ".", "envs")
		require.Len(t, got, 2)
		assert.Equal(t, "dev", got[0].Each)
		assert.Equal(t, "prod", got[1].Each)
	})
}

// TestRenderFileFromFS_DontInheritVariablesBlocksParentScope verifies the
// dont-inherit-variables flag: the child shouldn't see parent vars by
// name. A reference to an inherited-only var must fail.
func TestRenderFileFromFS_DontInheritVariablesBlocksParentScope(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Shared
dependencies:
  - name: dep
    template-url: ./dep
    output-folder: ./dep
    dont-inherit-variables: true
`)},
		"dep/boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"dep/out.txt":         &fstest.MapFile{Data: []byte(`{{ .Shared }}`)},
	}

	// Parent's Shared is set, but dep doesn't inherit. Render must fail
	// with a missing-key error from the template engine — NOT one of our
	// sentinel errors.
	_, err := renderFile(t, fsys, "dep/out.txt", map[string]any{"Shared": "value"})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrOutputNotProduced)
	require.NotErrorIs(t, err, ErrSkipFilesExcluded)
}

// TestRenderFileFromFS_ForEachSeedsEachInDepVariableDefaults is the
// reproduction for the gruntwork-landing-zone bug: a parent declares a
// for_each dep whose own variables block uses `default: '{{ .__each__ }}'`.
// Pre-fix, warm dispatch failed every iteration with "map has no entry for
// key __each__" because the per-iteration value was never seeded into the
// parent scope before scopeForDep evaluated the default. The fix makes
// the renderer carry the producer-recorded Each value into the parent
// scope before building the child's scope — the same shape the CLI
// constructs in templates.processDependency.
func TestRenderFileFromFS_ForEachSeedsEachInDepVariableDefaults(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
dependencies:
  - name: envs
    template-url: ./envs
    output-folder: "envs/{{ .__each__ }}"
    for_each:
      - dev
      - prod
    variables:
      - name: FormattedAccountName
        default: "acct-{{ .__each__ }}"
`)},

		"envs/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: FormattedAccountName
`)},
		"envs/account.hcl": &fstest.MapFile{Data: []byte(`account = "{{ .FormattedAccountName }}"`)},
	}

	t.Run("first_iteration_renders", func(t *testing.T) {
		t.Parallel()
		got, err := renderFile(t, fsys, "envs/dev/account.hcl", map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, `account = "acct-dev"`, got)
	})

	t.Run("second_iteration_renders", func(t *testing.T) {
		t.Parallel()
		got, err := renderFile(t, fsys, "envs/prod/account.hcl", map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, `account = "acct-prod"`, got)
	})
}

// TestRenderFileFromFS_ForEachOutputFolderUsesEach pins the producer-side
// half of the same fix: a for_each dep with a templated output-folder
// (the gruntwork-landing-zone shape, `pipelines-config/{{ .__each__ }}`)
// must render to per-iteration folders rather than a single empty one.
// Without this the renderer would see one ResolvedDep with an
// empty/malformed OutputFolder and never find the requested file.
func TestRenderFileFromFS_ForEachOutputFolderUsesEach(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
dependencies:
  - name: envs
    template-url: ./envs
    output-folder: "envs/{{ .__each__ }}"
    for_each:
      - alpha
      - beta
`)},
		"envs/boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"envs/marker.txt":      &fstest.MapFile{Data: []byte(`hello from {{ .__each__ }}`)},
	}

	got, err := renderFile(t, fsys, "envs/alpha/marker.txt", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "hello from alpha", got)

	got, err = renderFile(t, fsys, "envs/beta/marker.txt", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "hello from beta", got)
}

// TestRenderFileFromFS_NoForEachDoesNotLeakEachIntoScope guards the
// non-iterating path: a plain dep must NOT see a `__each__` value in its
// scope, even if a sibling dep is for_each-driven, because __each__ is
// scoped to a single iteration. We assert this by referencing __each__
// from a plain dep's template body and expecting a missing-key error.
func TestRenderFileFromFS_NoForEachDoesNotLeakEachIntoScope(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
dependencies:
  - name: plain
    template-url: ./plain
    output-folder: ./plain
`)},
		"plain/boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"plain/out.txt":         &fstest.MapFile{Data: []byte(`{{ .__each__ }}`)},
	}

	_, err := renderFile(t, fsys, "plain/out.txt", map[string]any{})
	require.Error(t, err, "plain dep must not see __each__ in scope")
	assert.NotErrorIs(t, err, ErrOutputNotProduced,
		"missing __each__ should surface as a render error, not a sentinel; got: %v", err)
}

// TestRenderFileFromFS_ForEachReferenceUsesParentDefault mirrors the
// gruntwork-landing-zone shape: the parent template declares a list
// variable (AWSCoreAccountsList) with a default. A child dep iterates
// via for_each_reference pointing at that variable name, and the dep's
// own variables block has a default referencing `.__each__`. Pre-fix
// the bundle producer couldn't resolve the for_each_reference because
// it only knew about user-supplied vars (not the parent template's
// declared defaults), so it emitted a single non-iterated entry and
// the renderer's scopeForDep blew up evaluating the `__each__`-using
// default. The fix makes the producer apply config defaults so
// for_each_reference can find list variables defined in the
// boilerplate.yml itself.
func TestRenderFileFromFS_ForEachReferenceUsesParentDefault(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: AccountsList
    type: list
    default:
      - logs
      - security
      - shared
dependencies:
  - name: envs
    template-url: ./envs
    output-folder: "envs/{{ .__each__ }}"
    for_each_reference: AccountsList
    variables:
      - name: FormattedAccountName
        default: "{{ .__each__ }}"
`)},
		"envs/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: FormattedAccountName
`)},
		"envs/account.hcl": &fstest.MapFile{Data: []byte(`account = "{{ .FormattedAccountName }}"`)},
	}

	for _, env := range []string{"logs", "security", "shared"} {
		t.Run(env, func(t *testing.T) {
			t.Parallel()

			got, err := renderFile(t, fsys, "envs/"+env+"/account.hcl", map[string]any{})
			require.NoError(t, err)
			assert.Equal(t, `account = "`+env+`"`, got)
		})
	}
}

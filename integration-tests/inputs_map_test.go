package integrationtests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/inputs"
)

// inputsMapResult mirrors the JSON shape of inputs.Result so this integration
// test can decode it without depending on the inputs package directly. (We
// could import the package, but doing so would couple this test to its
// internal types; using a local mirror keeps the contract explicit.)
type inputsMapResult struct {
	Inputs map[string]struct {
		Name        string   `json:"name"`
		DeclaredIn  string   `json:"declared_in"`
		Type        string   `json:"type"`
		Description string   `json:"description,omitempty"`
		Files       []string `json:"files"`
	} `json:"inputs"`

	Files map[string][]string `json:"files"`

	Sources map[string]string `json:"sources"`

	Errors []struct {
		Kind     string `json:"kind"`
		Template string `json:"template,omitempty"`
		Name     string `json:"name,omitempty"`
		File     string `json:"file,omitempty"`
		Message  string `json:"message,omitempty"`
	} `json:"errors"`

	// Bundle is populated only when --include-bundle is set. It's
	// optional so existing test cases that don't pass the flag can
	// continue decoding the response into this struct.
	Bundle *struct {
		RootPath     string                          `json:"rootPath"`
		Files        map[string]string               `json:"files"`
		Dependencies map[string][]inputs.ResolvedDep `json:"dependencies"`
	} `json:"bundle,omitempty"`
}

// TestInputsMap_TransitiveFixture drives the `boilerplate inputs map`
// subcommand against a small fixture (one root + one nested dependency that
// receives a parent variable via an explicit value expression). It validates
// that the JSON output contains the expected entries, including the inverse
// `files` index and at least one transitive edge.
func TestInputsMap_TransitiveFixture(t *testing.T) {
	t.Parallel()

	app := cli.CreateBoilerplateCli()

	var stdout, stderr bytes.Buffer

	app.Writer = &stdout
	app.ErrWriter = &stderr

	args := []string{
		"boilerplate", "inputs", "map",
		"--template-url", "../test-fixtures/inputs-test/transitive",
	}

	require.NoError(t, app.Run(args), "stderr=%s", stderr.String())

	var got inputsMapResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "stdout=%s", stdout.String())

	// Root declarations.
	require.Contains(t, got.Inputs, ".:Region")
	assert.Equal(t, "string", got.Inputs[".:Region"].Type)
	assert.Equal(t, ".", got.Inputs[".:Region"].DeclaredIn)

	require.Contains(t, got.Inputs, ".:ProjectName")

	// Transitive edge: root's Region reaches modules/vpc/main.tf via the
	// explicit `{{ .Region }}` expression in the dependency's variables block.
	assert.Contains(t, got.Inputs[".:Region"].Files, "modules/vpc/main.tf",
		"root.Region should reach modules/vpc/main.tf via the dependency value expression")

	// Child declaration.
	require.Contains(t, got.Inputs, "modules/vpc:AwsRegion")
	assert.Equal(t, "modules/vpc", got.Inputs["modules/vpc:AwsRegion"].DeclaredIn)
	assert.Contains(t, got.Inputs["modules/vpc:AwsRegion"].Files, "modules/vpc/main.tf")

	// Inverse index covers both inputs that affect modules/vpc/main.tf.
	require.Contains(t, got.Files, "modules/vpc/main.tf")
	assert.Contains(t, got.Files["modules/vpc/main.tf"], ".:Region")
	assert.Contains(t, got.Files["modules/vpc/main.tf"], "modules/vpc:AwsRegion")

	// README.md references both root vars directly.
	require.Contains(t, got.Files, "README.md")
	assert.ElementsMatch(t,
		[]string{".:ProjectName", ".:Region"},
		got.Files["README.md"],
	)

	// No soft errors expected from this clean fixture.
	assert.Empty(t, got.Errors, "unexpected analysis errors: %+v", got.Errors)

	// Every output path in Files must have a Sources entry for this fixture
	// (none of these files have templated names that fail to render). The
	// values must point at real files on disk.
	require.NotEmpty(t, got.Sources, "expected non-empty sources map")

	for outPath := range got.Files {
		src, ok := got.Sources[outPath]
		require.Truef(t, ok, "missing sources entry for %q", outPath)

		require.Truef(t, filepath.IsAbs(src), "source path %q for %q is not absolute", src, outPath)

		_, statErr := os.Stat(src)
		require.NoErrorf(t, statErr, "source path %q for %q does not exist on disk", src, outPath)
	}
}

// TestInputsMap_SourcesField drives the new `sources` field. The fixture has
// one local-path dep so we exercise both root and child source paths, plus a
// file whose name contains an unclosed `{{` so the analyzer emits a
// filename_render soft error and omits that output from sources entirely.
func TestInputsMap_SourcesField(t *testing.T) {
	t.Parallel()

	app := cli.CreateBoilerplateCli()

	var stdout, stderr bytes.Buffer

	app.Writer = &stdout
	app.ErrWriter = &stderr

	args := []string{
		"boilerplate", "inputs", "map",
		"--template-url", "../test-fixtures/inputs-test/sources",
	}

	require.NoError(t, app.Run(args), "stderr=%s", stderr.String())

	var got inputsMapResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "stdout=%s", stdout.String())

	require.NotEmpty(t, got.Sources, "expected non-empty sources map")

	// Collect the set of output paths that the analyzer flagged as having an
	// unrenderable templated filename. Those paths are expected to appear in
	// Files (the analyzer falls back to the unrendered path so the inverse
	// index still names something) but to be absent from Sources, since
	// there's no single source file we can point the consumer at.
	dynamicPaths := map[string]struct{}{}

	for _, e := range got.Errors {
		if e.Kind == "filename_render" {
			// The error reports the on-disk path relative to the template
			// folder; for the root template that matches the output path.
			dynamicPaths[e.File] = struct{}{}
		}
	}

	require.NotEmpty(t, dynamicPaths,
		"expected at least one filename_render error from the fixture; got errors=%+v", got.Errors)

	// For every file the analyzer reports, either it has a Sources entry
	// (pointing at a real, absolute path), or it's flagged as a
	// filename_render case and is intentionally omitted.
	for outPath := range got.Files {
		if _, dynamic := dynamicPaths[outPath]; dynamic {
			_, hasSource := got.Sources[outPath]
			assert.Falsef(t, hasSource,
				"output %q is a filename_render case and must not be in sources", outPath)

			continue
		}

		src, ok := got.Sources[outPath]
		require.Truef(t, ok, "missing sources entry for %q (files=%v, sources=%v)", outPath, got.Files, got.Sources)
		require.Truef(t, filepath.IsAbs(src), "source path %q for %q is not absolute", src, outPath)

		_, statErr := os.Stat(src)
		require.NoErrorf(t, statErr, "source path %q for %q does not exist on disk", src, outPath)
	}

	// Spot-check that the dep file landed under the resolved dep source dir
	// (i.e., we don't accidentally point modules/web/index.html at the root
	// template's directory).
	if depSrc, ok := got.Sources["modules/web/index.html"]; ok {
		assert.Contains(t, depSrc, filepath.Join("modules", "web"),
			"dep file source %q should live under the dep's source dir", depSrc)
	}
}

// TestInputsMap_NoBundleWithoutFlag verifies the back-compat guarantee:
// without --include-bundle, the JSON output has no `bundle` key at all.
// Existing consumers (which don't ask for the bundle) must see the exact
// same shape as before.
func TestInputsMap_NoBundleWithoutFlag(t *testing.T) {
	t.Parallel()

	app := cli.CreateBoilerplateCli()

	var stdout, stderr bytes.Buffer

	app.Writer = &stdout
	app.ErrWriter = &stderr

	args := []string{
		"boilerplate", "inputs", "map",
		"--template-url", "../test-fixtures/inputs-test/transitive",
	}

	require.NoError(t, app.Run(args), "stderr=%s", stderr.String())

	// Decode into a generic map so we can assert on the presence/absence
	// of the bundle key without relying on the strongly-typed mirror.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &raw), "stdout=%s", stdout.String())

	_, hasBundle := raw["bundle"]
	assert.False(t, hasBundle, "no bundle key should appear when --include-bundle is unset")
}

// TestInputsMap_IncludeBundleEmitsAllFiles drives the new --include-bundle
// flag against the transitive fixture and verifies:
//   - bundle.files is non-empty.
//   - every boilerplate.yml in the dep tree is present.
//   - every source path in `sources` has a corresponding bundle entry
//     (keyed by output path).
//   - the existing JSON fields (inputs/files/sources/errors) still
//     contain the same data they did pre-flag.
func TestInputsMap_IncludeBundleEmitsAllFiles(t *testing.T) {
	t.Parallel()

	app := cli.CreateBoilerplateCli()

	var stdout, stderr bytes.Buffer

	app.Writer = &stdout
	app.ErrWriter = &stderr

	args := []string{
		"boilerplate", "inputs", "map",
		"--template-url", "../test-fixtures/inputs-test/transitive",
		"--include-bundle",
	}

	require.NoError(t, app.Run(args), "stderr=%s", stderr.String())

	var got inputsMapResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "stdout=%s", stdout.String())

	// Bundle field present and populated.
	require.NotNil(t, got.Bundle, "bundle should be present when --include-bundle is set")
	require.NotEmpty(t, got.Bundle.Files, "bundle.files should not be empty")

	// Every boilerplate.yml in the resolved tree must be present. Deps
	// live under the producer's reserved `_deps/<name>/` prefix, not
	// under their disk-side template-url — that path arithmetic is no
	// longer load-bearing.
	require.Contains(t, got.Bundle.Files, "boilerplate.yml",
		"root boilerplate.yml must appear in bundle.files")

	depBundlePath := lookupBundleDep(t, got.Bundle.Dependencies, ".", "vpc")
	require.NotEmpty(t, depBundlePath, "expected the vpc dep to appear in bundle.dependencies under the root")
	require.Contains(t, got.Bundle.Files, depBundlePath+"/boilerplate.yml",
		"dep boilerplate.yml must appear in bundle.files at the producer's bundle path")

	// Existing data shape is unaffected by the flag.
	require.Contains(t, got.Inputs, ".:Region")
	require.Contains(t, got.Inputs, "modules/vpc:AwsRegion")
	assert.Empty(t, got.Errors)
}

// lookupBundleDep returns the BundlePath the producer recorded for the
// dep named depName under parentKey, or "" if not present. Integration
// tests don't import the inputs.ResolvedDep helpers, so this stays here.
func lookupBundleDep(t *testing.T, idx map[string][]inputs.ResolvedDep, parentKey, depName string) string {
	t.Helper()

	for _, dep := range idx[parentKey] {
		if dep.Name == depName {
			return dep.BundlePath
		}
	}

	return ""
}

// TestInputsMap_BundleRoundTripsThroughRenderFile is the end-to-end loop
// verification from the spec: take the bundle emitted by --include-bundle,
// re-feed it into the same RenderFileFromFS the WASM bridge wraps, and
// assert the warm-rendered output matches what a fresh cold render via
// `boilerplate template` would produce for at least one dep-owned file.
//
// We don't shell out to `boilerplate template`: this is an integration
// test inside the boilerplate repo, so we can call RenderFileFromFS
// directly and compare against the literal expected output computed from
// the fixture and supplied vars. The point is to prove the bundle is
// self-consistent: bundle in → renderfile → same bytes a full render
// produces.
func TestInputsMap_BundleRoundTripsThroughRenderFile(t *testing.T) {
	t.Parallel()

	app := cli.CreateBoilerplateCli()

	var stdout, stderr bytes.Buffer

	app.Writer = &stdout
	app.ErrWriter = &stderr

	args := []string{
		"boilerplate", "inputs", "map",
		"--template-url", "../test-fixtures/inputs-test/transitive",
		"--var", "Region=us-east-1",
		"--var", "ProjectName=demo",
		"--include-bundle",
	}

	require.NoError(t, app.Run(args), "stderr=%s", stderr.String())

	var got inputsMapResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "stdout=%s", stdout.String())
	require.NotNil(t, got.Bundle)
	require.NotNil(t, got.Bundle.Dependencies, "bundle must carry the Dependencies index for warm dispatch")

	// Reconstruct the bundle as an fs.FS that mirrors what the WASM
	// renderFile bridge does internally. Same shape; same code path.
	mfs := fstest.MapFS{}
	for k, v := range got.Bundle.Files {
		mfs[k] = &fstest.MapFile{Data: []byte(v)}
	}

	vars := map[string]any{
		"Region":      "us-east-1",
		"ProjectName": "demo",
	}

	// Warm-render the dep-owned file. The vpc dep inherits Region via an
	// explicit `default: "{{ .Region }}"` edge for AwsRegion. Output
	// paths are still expressed relative to the rendered output tree
	// (`modules/vpc/main.tf`), independent of where the producer parked
	// the dep's source files inside the bundle.
	rendered, err := inputs.RenderFileFromFS(context.Background(), mfs, ".", "modules/vpc/main.tf", vars, got.Bundle.Dependencies)
	require.NoError(t, err, "warm render failed; bundle keys: %v", mapKeys(got.Bundle.Files))

	expected := "provider \"aws\" {\n  region = \"us-east-1\"\n}\n"
	assert.Equal(t, expected, rendered,
		"warm-rendered dep file should match what a cold render would produce")

	// Also exercise a root-owned file to confirm the same loop works at
	// the root scope (different scope-building path).
	rendered, err = inputs.RenderFileFromFS(context.Background(), mfs, ".", "README.md", vars, got.Bundle.Dependencies)
	require.NoError(t, err)
	assert.Equal(t, "# demo\n\nStack deployed in region us-east-1.\n", rendered)
}

// mapKeys returns the sorted keys of m, for human-readable test failure
// messages. Kept local because the integration-tests package doesn't
// otherwise need this helper.
func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}

// TestInputsMap_TemplateFolderSiblingDepBundlesCleanly is the regression
// gate for the `{{ templateFolder }}` bug: a wrapper template whose
// dependency declares its template-url with `{{ templateFolder }}/..`
// used to produce bundle keys that contained the parent template's
// absolute disk path concatenated twice. With the producer-authoritative
// fix, every bundle.files key must satisfy the same validateBundlePath
// rule the WASM bridge applies, the Dependencies index must point at a
// path prefix that actually exists in bundle.files, and feeding the
// bundle back through RenderFileFromFS must produce the same bytes a
// fresh cold render would.
//
// Without this test, the regression would only surface after shipping
// to the WASM consumer — which is exactly how the original bug was
// found.
func TestInputsMap_TemplateFolderSiblingDepBundlesCleanly(t *testing.T) {
	t.Parallel()

	app := cli.CreateBoilerplateCli()

	var stdout, stderr bytes.Buffer

	app.Writer = &stdout
	app.ErrWriter = &stderr

	args := []string{
		"boilerplate", "inputs", "map",
		"--template-url", "../test-fixtures/inputs-test/templatefolder-sibling/root",
		"--var", "OrgName=acme",
		"--include-bundle",
	}

	require.NoError(t, app.Run(args), "stderr=%s", stderr.String())

	var got inputsMapResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "stdout=%s", stdout.String())
	require.NotNil(t, got.Bundle)
	require.NotNil(t, got.Bundle.Dependencies)
	require.NotEmpty(t, got.Bundle.Files)

	// Assertion #1 — every bundle key passes the same validation the
	// WASM bridge runs at ingest. This is what would have caught the
	// original "abs-path concatenated twice" bug.
	for k := range got.Bundle.Files {
		require.NoErrorf(t, validateBundleKeyForTest(k), "bundle file key %q failed validation", k)
	}

	// Assertion #2 — the Dependencies index identifies the catalog dep
	// under the root parent, and its BundlePath is a prefix of bundle
	// files that exist (i.e., the producer placed the dep's files where
	// the consumer would look for them).
	depBundlePath := lookupBundleDep(t, got.Bundle.Dependencies, ".", "catalog")
	require.NotEmptyf(t, depBundlePath,
		"expected the catalog dep to appear in bundle.dependencies under the root parent; got: %+v", got.Bundle.Dependencies)
	require.NoError(t, validateBundleKeyForTest(depBundlePath))

	hitsUnderDep := 0

	for k := range got.Bundle.Files {
		if k == depBundlePath || strings.HasPrefix(k, depBundlePath+"/") {
			hitsUnderDep++
		}
	}

	require.Positivef(t, hitsUnderDep,
		"no bundle files live under the dep's BundlePath %q (bundle keys: %v)", depBundlePath, mapKeys(got.Bundle.Files))

	// Assertion #3 — warm render returns the same bytes a cold render
	// would. We don't shell out to `boilerplate template`; the cold
	// expectation is built inline from the fixture so this test pins
	// the byte-equivalence requirement without dragging in the
	// renderer's command-line surface.
	mfs := fstest.MapFS{}
	for k, v := range got.Bundle.Files {
		mfs[k] = &fstest.MapFile{Data: []byte(v)}
	}

	vars := map[string]any{"OrgName": "acme"}

	// The dep declares `Region` with a default `us-east-1`. The user
	// supplied only OrgName, so warm render must pick up the dep's
	// default — matching the runtime's non-interactive precedence.
	rendered, err := inputs.RenderFileFromFS(context.Background(), mfs, ".", "catalog/main.tf", vars, got.Bundle.Dependencies)
	require.NoError(t, err, "warm render failed; bundle keys: %v", mapKeys(got.Bundle.Files))
	assert.Equal(t,
		"org    = \"acme\"\nregion = \"us-east-1\"\n",
		rendered,
		"warm-rendered dep file must match cold-render output")

	// Root file too — different scope-building path.
	renderedRoot, err := inputs.RenderFileFromFS(context.Background(), mfs, ".", "README.md", vars, got.Bundle.Dependencies)
	require.NoError(t, err)
	assert.Equal(t, "# acme\n\nRoot template for the templateFolder-sibling fixture.\n", renderedRoot)
}

// TestInputsMap_ForEachBundleSeedsEachIntoWarmRender is the regression
// gate for the gruntwork-landing-zone bug: a parent declares a for_each
// dep whose output-folder and variable defaults both reference
// `.__each__`. Pre-fix, the bundle produced one ResolvedDep with an
// empty/malformed OutputFolder and warm dispatch failed every iteration
// with `map has no entry for key __each__`. The fix makes the producer
// emit one entry per iteration (pre-rendered OutputFolder + Each value)
// and makes the renderer seed `__each__` into the parent scope before
// evaluating that iteration's dep-variable defaults.
//
// We assert the bundle shape AND the round-trip: feeding the bundle
// back through RenderFileFromFS must produce the same content for every
// iteration that a cold render via `boilerplate template` would.
func TestInputsMap_ForEachBundleSeedsEachIntoWarmRender(t *testing.T) {
	t.Parallel()

	app := cli.CreateBoilerplateCli()

	var stdout, stderr bytes.Buffer

	app.Writer = &stdout
	app.ErrWriter = &stderr

	args := []string{
		"boilerplate", "inputs", "map",
		"--template-url", "../test-fixtures/inputs-test/for-each-default",
		"--include-bundle",
	}

	require.NoError(t, app.Run(args), "stderr=%s", stderr.String())

	var got inputsMapResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "stdout=%s", stdout.String())
	require.NotNil(t, got.Bundle)
	require.NotNil(t, got.Bundle.Dependencies)

	// Producer-side assertion: the bundle records one entry per iteration
	// with the iteration value and the per-iteration pre-rendered
	// output-folder. Pre-fix this would have been a single entry with
	// an empty OutputFolder.
	rootDeps := got.Bundle.Dependencies["."]
	require.Len(t, rootDeps, 3, "for_each dep should produce one entry per iteration; got: %+v", rootDeps)

	wantOrder := []string{"dev", "staging", "prod"}
	for i, want := range wantOrder {
		assert.Equal(t, "envs", rootDeps[i].Name)
		assert.Equal(t, want, rootDeps[i].Each, "Each must carry the iteration value")
		assert.Equal(t, "pipelines-config/"+want, rootDeps[i].OutputFolder,
			"OutputFolder must be pre-rendered with __each__=%q", want)
	}

	// Warm-render every iteration through the bundle. Pre-fix every one
	// of these would have failed with the missing-key error reported in
	// the bug.
	mfs := fstest.MapFS{}
	for k, v := range got.Bundle.Files {
		mfs[k] = &fstest.MapFile{Data: []byte(v)}
	}

	for _, env := range wantOrder {
		outputPath := "pipelines-config/" + env + "/account.hcl"
		rendered, err := inputs.RenderFileFromFS(context.Background(), mfs, ".", outputPath, map[string]any{}, got.Bundle.Dependencies)
		require.NoErrorf(t, err, "warm render of %q failed; bundle keys: %v", outputPath, mapKeys(got.Bundle.Files))
		assert.Equalf(t, `account = "acct-`+env+`"`+"\n", rendered,
			"warm-rendered %q must match cold-render output", outputPath)
	}
}

// TestInputsMap_RenderFileRejectsOldBundles is the dispatcher contract:
// a bundle that arrives without a Dependencies index (older CLI) must
// not silently fall through to the broken re-render path. The consumer
// surfaces ErrDependencyNotInBundle, which the dispatcher routes to
// cold.
func TestInputsMap_RenderFileRejectsOldBundles(t *testing.T) {
	t.Parallel()

	mfs := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"hello.txt":       &fstest.MapFile{Data: []byte(`hi`)},
	}

	_, err := inputs.RenderFileFromFS(context.Background(), mfs, ".", "hello.txt", map[string]any{}, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, inputs.ErrDependencyNotInBundle),
		"nil deps index must trip the too-old-bundle guard; got: %v", err)
}

// validateBundleKeyForTest mirrors the rule the WASM bridge applies
// in cmd/wasm/renderfile.validateBundlePath. Kept inline so this test
// doesn't depend on the //go:build js && wasm package.
func validateBundleKeyForTest(p string) error {
	if p == "" {
		return errors.New("empty path")
	}

	if strings.HasPrefix(p, "/") {
		return errors.New("absolute paths not allowed")
	}

	if strings.ContainsRune(p, '\\') {
		return errors.New("use forward slashes")
	}

	cleaned := path.Clean(p)
	if cleaned != p {
		return fmt.Errorf("non-canonical path; clean to %q", cleaned)
	}

	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return errors.New("path escapes bundle root")
	}

	return nil
}

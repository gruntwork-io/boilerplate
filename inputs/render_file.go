package inputs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"path"
	"strings"
	"text/template"

	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/variables"
)

// Sentinel errors returned by RenderFileFromFS. Callers (notably the WASM
// bridge) discriminate on these so they can route specific failure modes to
// the cold-render fallback rather than surfacing them as generic render
// errors.
var (
	// ErrOutputNotProduced means outputPath is not produced by any file in
	// the resolved template tree.
	ErrOutputNotProduced = errors.New("output path not produced by template tree")

	// ErrDependencyNotInBundle means a local dependency's template-url did
	// not resolve to a directory inside rootFS. Remote dependencies always
	// produce this error (no go-getter is available in WASM).
	ErrDependencyNotInBundle = errors.New("dependency not present in bundle")

	// ErrDynamicFilename means outputPath corresponds to a source file
	// whose filename contains template syntax that did not render to a
	// static path. The consumer should fall back to cold render rather
	// than attempt warm dispatch.
	ErrDynamicFilename = errors.New("output path has a dynamic (templated) filename")

	// ErrSkipFilesExcluded means a skip_files rule excludes outputPath
	// from the dep's output under the current vars. The consumer should
	// treat this as cold-only.
	ErrSkipFilesExcluded = errors.New("output path excluded by skip_files rule")
)

// RenderFileFromFS walks the dep tree rooted at rootPath in rootFS, locates
// the dep that produces outputPath, builds the dep-scoped variable map by
// applying each ancestor's variable defaults (overridable by userVars), and
// renders the source template that produces outputPath against that scope.
//
// The function is side-effect-free: hooks never execute, no files are
// written, and no I/O occurs outside rootFS. It is the WASM warm-dispatch
// counterpart to running `boilerplate template` against a single file.
//
// depsIndex is the bundle's pre-computed dependency layout (see
// Bundle.Dependencies). It MUST be supplied. Without it, the consumer
// would have to re-render each dep's `template-url` against an in-memory
// fs where helpers like `{{ templateFolder }}` are meaningless — which is
// the precise bug this signature change exists to prevent. Older bundles
// (pre-fix CLIs) won't carry a Dependencies field; the consumer rejects
// them with ErrDependencyNotInBundle so callers route to cold render.
// Templates with no deps may pass an empty map; nil is reserved for the
// "old bundle" error path.
//
// userVars is the merged scope supplied by the caller. For nested deps it
// is layered onto each dep's declared defaults (user wins). A var
// referenced by a template but absent from both userVars and any dep
// default produces a render error via missingkey=error.
//
// On success it returns the rendered file contents. Failure modes are
// signaled via the sentinel errors declared above and may be checked with
// errors.Is.
func RenderFileFromFS(ctx context.Context, rootFS fs.FS, rootPath, outputPath string, userVars map[string]any, depsIndex map[string][]ResolvedDep) (string, error) {
	if rootPath == "" {
		rootPath = "."
	}

	outputPath = path.Clean(strings.TrimLeft(strings.TrimSpace(outputPath), "/"))
	if outputPath == "" || outputPath == "." {
		return "", fmt.Errorf("outputPath must be a non-empty relative path")
	}

	if depsIndex == nil {
		return "", fmt.Errorf("%w: bundle has no Dependencies index; re-produce the bundle with `boilerplate inputs map --include-bundle` from a newer CLI", ErrDependencyNotInBundle)
	}

	rootLoc := templateLocation{fsys: rootFS, dir: rootPath}

	visiting := map[string]struct{}{}

	rendered, found, err := renderFileWalk(ctx, rootLoc, ".", ".", outputPath, userVars, depsIndex, visiting)
	if err != nil {
		return "", err
	}

	if !found {
		return "", fmt.Errorf("%w: %q", ErrOutputNotProduced, outputPath)
	}

	return rendered, nil
}

// renderFileWalk recursively descends from loc, building scope and looking
// for the source file that produces outputPath. Returns the rendered
// content if outputPath is owned by this template or any of its
// descendants. The found flag distinguishes "this subtree produced
// outputPath and here's the bytes" from "this subtree did not produce
// outputPath; keep searching".
//
// outputPath is the canonical, slash-separated path relative to the root
// output root. currentOutputPath tracks where this template's output lands
// inside that root ("." for the root template, the dep's resolved
// output-folder for deps). currentBundlePath tracks the parent key into
// depsIndex — "." at the root, otherwise the previous dep's BundlePath.
//
// vars is the scope for THIS template — i.e., the parent's scope with this
// template's variable defaults already applied. The first call (root) just
// gets userVars; recursive calls layer on each dep's resolution.
func renderFileWalk(
	ctx context.Context,
	loc templateLocation,
	currentOutputPath string,
	currentBundlePath string,
	outputPath string,
	vars map[string]any,
	depsIndex map[string][]ResolvedDep,
	visiting map[string]struct{},
) (string, bool, error) {
	cfg, err := loadConfig(loc)
	if err != nil {
		return "", false, err
	}

	// Apply this template's own boilerplate.yml defaults to the scope.
	// Non-interactive semantics: user-supplied wins, otherwise the default
	// is used. Variables with neither produce a missing-key render error
	// downstream — same as `boilerplate template --non-interactive`.
	scope, err := applyConfigDefaults(ctx, loc, cfg, vars)
	if err != nil {
		return "", false, err
	}

	partials, err := renderPartialGlobs(ctx, loc, cfg.Partials, scope)
	if err != nil {
		return "", false, err
	}

	// Process this template's skip_files. We need it both to verify
	// outputPath isn't excluded and to skip source files that would not be
	// produced.
	skipFilter, skipErrs := processSkipFiles(ctx, loc, cfg.SkipFiles, scope)
	if len(skipErrs) > 0 {
		// Surface the first skip_files error rather than silently
		// proceeding; an unrenderable skip rule could otherwise change
		// which file (if any) we treat as the owner of outputPath.
		return "", false, fmt.Errorf("skip_files processing failed: %s", skipErrs[0].Message)
	}

	// Skip-dirs are computed from the deps index's BundlePath entries.
	// Each dep's files live under that path inside loc.fsys; the walker
	// must avoid descending into them so they don't bleed into the
	// parent's owns-this-path determination.
	skipDirs := bundleDepSkipDirs(depsIndex[currentBundlePath])

	// First, see if this template owns outputPath. Walk its files (skipping
	// dep subtrees) and check each one's computed output path against
	// outputPath. If we find a match, render and return.
	rendered, ownedHere, err := findAndRenderInThisTemplate(ctx, loc, scope, partials, skipFilter, skipDirs, currentOutputPath, outputPath)
	if err != nil {
		return "", false, err
	}

	if ownedHere {
		return rendered, true, nil
	}

	// Not owned here. Descend into each dependency that could plausibly
	// contain outputPath. The producer is authoritative for layout:
	// instead of re-rendering each dep's template-url (which can't be
	// done safely against an in-memory bundle), we look up the dep's
	// pre-resolved BundlePath / OutputFolder from depsIndex.
	for i := range cfg.Dependencies {
		dep := &cfg.Dependencies[i]

		// Honor skip: a skipped dep contributes nothing to the output tree.
		shouldSkip, skipErr := shouldSkipDependency(ctx, loc, dep, scope)
		if skipErr != nil {
			return "", false, skipErr
		}

		if shouldSkip {
			continue
		}

		// A for_each dep produces N ResolvedDep entries (same Name +
		// BundlePath, different OutputFolder + Each); a normal dep
		// produces one. Iterate every matching entry so each iteration's
		// scope is built with __each__ seeded into the parent — the
		// runtime semantic the WASM path was previously missing.
		matched := lookupDeps(depsIndex, currentBundlePath, dep.Name)
		if len(matched) == 0 {
			// Producer didn't bundle this dep. Two reasons it could be
			// missing: (a) it was a remote URL or otherwise unresolvable
			// at bundle time, (b) the bundle predates this fix. Either
			// way the consumer's contract is to surface
			// ErrDependencyNotInBundle so the dispatcher routes to cold.
			// Skip to the next sibling — a later dep may still own
			// outputPath; if none does we return ErrOutputNotProduced
			// from the top of RenderFileFromFS, which the dispatcher
			// also treats as a fall-through signal.
			continue
		}

		for j := range matched {
			resolved := &matched[j]

			result, found, recurErr := descendIntoDep(ctx, loc, dep, resolved, scope, currentOutputPath, outputPath, depsIndex, visiting)
			if recurErr != nil {
				return "", false, recurErr
			}

			if found {
				return result, true, nil
			}
		}
	}

	return "", false, nil
}

// descendIntoDep applies one ResolvedDep entry (either the only entry for a
// plain dep, or one iteration of a for_each dep): it seeds __each__ into
// the parent scope when the entry carries an Each value, builds the child
// scope via scopeForDep, and recurses. Splitting this out keeps
// renderFileWalk's per-dep loop linear and lets the for_each case share
// every other step with the non-iterating case.
func descendIntoDep(
	ctx context.Context,
	parentLoc templateLocation,
	dep *variables.Dependency,
	resolved *ResolvedDep,
	parentScope map[string]any,
	currentOutputPath string,
	outputPath string,
	depsIndex map[string][]ResolvedDep,
	visiting map[string]struct{},
) (string, bool, error) {
	// Cycle detection — keyed on the bundle path the producer recorded.
	// Two distinct deps with the same name nested recursively land at
	// distinct BundlePaths, so this is safe. for_each iterations share
	// a BundlePath but the loop in renderFileWalk processes them
	// sequentially and we delete the entry after each recursion returns,
	// so the second iteration is not mistaken for a cycle.
	cycleKey := resolved.BundlePath
	if _, busy := visiting[cycleKey]; busy {
		return "", false, nil
	}

	visiting[cycleKey] = struct{}{}
	defer delete(visiting, cycleKey)

	// Seed __each__ into the parent scope for this iteration so the
	// dep's variables block — whose defaults can legitimately reference
	// `.__each__` — has the value the runtime would have provided.
	// Non-for_each entries have Each=="" and skip this step entirely.
	scopeForChild := parentScope
	if resolved.Each != "" {
		scopeForChild = make(map[string]any, len(parentScope)+1)
		maps.Copy(scopeForChild, parentScope)
		scopeForChild[eachVarName] = resolved.Each
	}

	childLoc := templateLocation{fsys: parentLoc.fsys, dir: resolved.BundlePath}
	childOutputPath := joinOutputPath(currentOutputPath, resolved.OutputFolder)

	// Build child's scope: start from parent's (possibly __each__-seeded)
	// scope filtered/mapped by the dep's declared variables block
	// (explicit edges + defaults), then user-namespaced overrides. This
	// mirrors cloneVariablesForDependency in the runtime.
	childScope, scopeErr := scopeForDep(ctx, parentLoc, dep, scopeForChild)
	if scopeErr != nil {
		return "", false, fmt.Errorf("could not build scope for dependency %q: %w", dep.Name, scopeErr)
	}

	return renderFileWalk(ctx, childLoc, childOutputPath, resolved.BundlePath, outputPath, childScope, depsIndex, visiting)
}

// lookupDeps returns every ResolvedDep entry under (parentBundlePath, depName)
// from the bundle's pre-computed index. A plain dep matches at most one
// entry; a for_each dep matches once per iteration (same Name + BundlePath,
// distinct Each + OutputFolder). The slice is empty when the producer
// didn't bundle this dep — the renderer treats that as "skip and continue".
//
// Order is preserved from the producer's index, which itself preserves the
// order of for_each iterations declared in the parent's boilerplate.yml.
// Linear search is fine: deps lists are short and the renderer walks them
// at most once per recursion frame.
func lookupDeps(idx map[string][]ResolvedDep, parentBundlePath, depName string) []ResolvedDep {
	siblings := idx[parentBundlePath]

	var out []ResolvedDep

	for i := range siblings {
		if siblings[i].Name == depName {
			out = append(out, siblings[i])
		}
	}

	return out
}

// bundleDepSkipDirs returns the set of directory paths inside the bundle
// that hold a child dep's files. The walker uses this set to avoid
// descending into nested templates when deciding which file owns
// outputPath.
func bundleDepSkipDirs(siblings []ResolvedDep) map[string]struct{} {
	out := make(map[string]struct{}, len(siblings))

	for i := range siblings {
		p := path.Clean(siblings[i].BundlePath)
		if p == "" || p == "." {
			continue
		}

		out[p] = struct{}{}
	}

	return out
}

// findAndRenderInThisTemplate walks the template files under loc (excluding
// dep subtrees, the boilerplate.yml itself, and any skip_files-excluded
// files) and renders the one whose computed output path equals outputPath.
//
// Returns (rendered, true, nil) if found, (_, false, nil) if not. Returns
// ErrDynamicFilename if a file with a templated name would have matched
// outputPath but doesn't render to a static target — the consumer is
// expected to fall back to cold render. Returns ErrSkipFilesExcluded if a
// file under this template renders to outputPath but is excluded by
// skip_files (so the file would never be produced).
func findAndRenderInThisTemplate(
	ctx context.Context,
	loc templateLocation,
	scope map[string]any,
	partials []string,
	skipFilter *skipFileFilter,
	skipDirs map[string]struct{},
	currentOutputPath string,
	outputPath string,
) (string, bool, error) {
	walkRoot := loc.dir
	if walkRoot == "" {
		walkRoot = "."
	}

	cfgPath := path.Join(loc.dir, config.BoilerplateConfigFile)

	var (
		matchedSource string
		matchedSkip   bool
		matchedDyn    bool
	)

	walkErr := fs.WalkDir(loc.fsys, walkRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if _, skip := skipDirs[p]; skip && p != walkRoot {
				return fs.SkipDir
			}

			return nil
		}

		if p == cfgPath || path.Base(p) == config.BoilerplateConfigFile {
			return nil
		}

		rel := slashRel(loc.dir, p)
		if decoded, decErr := url.QueryUnescape(rel); decErr == nil {
			rel = decoded
		}

		// Render the filename portion against the current scope.
		renderedRel, renderErr := renderForRender(ctx, loc, rel, scope)
		if renderErr != nil {
			// If this file's name would have rendered to outputPath had it
			// been static, signal the dynamic-filename case. Otherwise
			// silently skip — a sibling file may still be the owner.
			if strings.Contains(rel, "{{") && filenameLikelyMatches(rel, currentOutputPath, outputPath) {
				matchedDyn = true
			}

			return nil
		}

		candidate := joinOutputPath(currentOutputPath, renderedRel)
		if candidate != outputPath {
			return nil
		}

		// This file's output path matches. Honor skip_files: a matched
		// file that would be excluded means consumer must fall back.
		if skipFilter.shouldSkip(slashRel(loc.dir, p)) {
			matchedSkip = true
			return fs.SkipAll
		}

		matchedSource = p

		return fs.SkipAll
	})
	if walkErr != nil {
		return "", false, walkErr
	}

	switch {
	case matchedSkip:
		return "", false, fmt.Errorf("%w for %q", ErrSkipFilesExcluded, outputPath)
	case matchedSource == "":
		if matchedDyn {
			return "", false, fmt.Errorf("%w: %q", ErrDynamicFilename, outputPath)
		}

		return "", false, nil
	}

	data, readErr := fs.ReadFile(loc.fsys, matchedSource)
	if readErr != nil {
		return "", false, readErr
	}

	rendered, err := renderTemplateBody(ctx, loc, matchedSource, string(data), scope, partials)
	if err != nil {
		return "", false, err
	}

	return rendered, true, nil
}

// filenameLikelyMatches is a heuristic for the dynamic-filename error path:
// when we encounter a source file whose filename failed to render, we want
// to know whether — had it rendered — it could have been the owner of
// outputPath. Treat any source file whose un-rendered name shares the
// expected directory prefix as a candidate. This intentionally errs on the
// side of returning the dynamic-filename error rather than the
// "not produced" error, because the consumer's contract is "if you can't
// produce it warm, fall back to cold" and ErrDynamicFilename triggers that.
func filenameLikelyMatches(unrenderedRel, currentOutputPath, outputPath string) bool {
	target := outputPath
	if currentOutputPath != "" && currentOutputPath != "." {
		prefix := strings.TrimSuffix(currentOutputPath, "/") + "/"
		if !strings.HasPrefix(outputPath, prefix) {
			return false
		}

		target = strings.TrimPrefix(outputPath, prefix)
	}

	// Compare directory portions: if the un-rendered file has the same
	// number of path segments as the target, treat it as a candidate.
	return strings.Count(unrenderedRel, "/") == strings.Count(target, "/")
}

// applyConfigDefaults applies a boilerplate.yml's declared variable
// defaults to the parent scope, mirroring the non-interactive branch of
// config.GetVariablesWithContext. The result is a fresh map; the input is
// not mutated.
//
// Precedence (highest first):
//  1. Value already in parentVars (user-supplied or inherited).
//  2. The variable's `default` field, rendered against the working scope.
//
// Variables with no value AND no default are left unset; a downstream
// render that references them will fail with the usual missing-key error.
// We intentionally don't error here — a variable may be declared in the
// config but unused by the source file we're about to render, and erroring
// on every such case would make warm dispatch fragile.
func applyConfigDefaults(ctx context.Context, loc templateLocation, cfg *config.BoilerplateConfig, parentVars map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(parentVars)+len(cfg.Variables))

	maps.Copy(out, parentVars)

	for _, v := range cfg.Variables {
		if _, set := out[v.Name()]; set {
			continue
		}

		def := v.Default()
		if def == nil {
			continue
		}

		// Render templated defaults against the scope built so far. Match
		// the runtime semantics where defaults can interpolate previously
		// declared variables.
		resolved, err := renderDefaultValue(ctx, loc, def, out)
		if err != nil {
			return nil, fmt.Errorf("rendering default for variable %q: %w", v.Name(), err)
		}

		typed, typeErr := variables.ConvertType(resolved, v)
		if typeErr != nil {
			return nil, fmt.Errorf("converting default for variable %q: %w", v.Name(), typeErr)
		}

		out[v.Name()] = typed
	}

	return out, nil
}

// renderDefaultValue runs templated leaf strings inside def through the
// template engine. Non-string leaves pass through untouched. Mirrors
// render.RenderVariables but kept inline because the analyzer's pure-render
// helpers all live in this package already.
func renderDefaultValue(ctx context.Context, loc templateLocation, def any, scope map[string]any) (any, error) {
	switch v := def.(type) {
	case string:
		if !strings.Contains(v, "{{") {
			return v, nil
		}

		return renderForRender(ctx, loc, v, scope)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			rendered, err := renderDefaultValue(ctx, loc, item, scope)
			if err != nil {
				return nil, err
			}

			out[i] = rendered
		}

		return out, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			rendered, err := renderDefaultValue(ctx, loc, item, scope)
			if err != nil {
				return nil, err
			}

			out[k] = rendered
		}

		return out, nil
	default:
		return def, nil
	}
}

// scopeForDep builds the child dep's variable scope from the parent's
// scope, applying the dependency's variables block (defaults + references)
// the way cloneVariablesForDependency does at runtime. The child's OWN
// boilerplate.yml defaults are applied separately by renderFileWalk via
// applyConfigDefaults when it descends into the child.
func scopeForDep(ctx context.Context, parentLoc templateLocation, dep *variables.Dependency, parentVars map[string]any) (map[string]any, error) {
	out := map[string]any{}

	if !dep.DontInheritVariables {
		// Inherit everything from parent except dependency-namespaced
		// entries (those are CLI-only and only apply to their target dep,
		// which we handle below for the matching name).
		for k, v := range parentVars {
			depName, _ := variables.SplitIntoDependencyNameAndVariableName(k)
			if depName == "" {
				out[k] = v
			}
		}
	}

	// Apply the dep's variables block. Each entry can be a reference (use
	// parent's value for the referenced name) or a default (rendered
	// against the in-progress scope so later defaults can reference earlier
	// resolved values).
	working := make(map[string]any, len(parentVars))
	maps.Copy(working, parentVars)

	for _, v := range dep.Variables {
		var value any

		switch {
		case v.Reference() != "":
			if refVal, ok := working[v.Reference()]; ok {
				value = refVal
			} else {
				// Referenced var has no value in parent scope. Skip rather
				// than error — the child's own boilerplate.yml will apply
				// its default (if any) when we descend.
				continue
			}
		case v.Default() != nil:
			resolved, err := renderDefaultValue(ctx, parentLoc, v.Default(), working)
			if err != nil {
				return nil, fmt.Errorf("rendering default for dep variable %q: %w", v.Name(), err)
			}

			typed, typeErr := variables.ConvertType(resolved, v)
			if typeErr != nil {
				return nil, fmt.Errorf("converting default for dep variable %q: %w", v.Name(), typeErr)
			}

			value = typed
		default:
			continue
		}

		out[v.Name()] = value
		working[v.Name()] = value
	}

	if dep.DontInheritVariables {
		return out, nil
	}

	// Layer dependency-namespaced parent vars (FOO.Bar) onto the child as
	// unqualified names when FOO matches this dep — same as the runtime.
	for key, value := range parentVars {
		depName, originalName := variables.SplitIntoDependencyNameAndVariableName(key)
		if depName == dep.Name {
			out[originalName] = value
		}
	}

	return out, nil
}

// shouldSkipDependency renders the dep's skip expression against the
// parent scope. Treats the result as `true` for the literal "true" only,
// matching the runtime.
func shouldSkipDependency(ctx context.Context, parentLoc templateLocation, dep *variables.Dependency, parentVars map[string]any) (bool, error) {
	if dep.Skip == "" {
		return false, nil
	}

	rendered, err := renderForRender(ctx, parentLoc, dep.Skip, parentVars)
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(rendered) == "true", nil
}

// renderPartialGlobs resolves each partial glob declared in cfg.Partials
// and returns the list of concrete file paths (as fs.FS-relative slash
// paths). The returned list is what renderTemplateBody parses as the
// template's partial templates.
func renderPartialGlobs(ctx context.Context, loc templateLocation, partialGlobs []string, scope map[string]any) ([]string, error) {
	if len(partialGlobs) == 0 {
		return nil, nil
	}

	var out []string

	for _, glob := range partialGlobs {
		rendered, err := renderForRender(ctx, loc, glob, scope)
		if err != nil {
			rendered = glob
		}

		joined := path.Join(loc.dir, rendered)

		matches, globErr := fs.Glob(loc.fsys, joined)
		if globErr != nil {
			return nil, fmt.Errorf("globbing partials %q: %w", glob, globErr)
		}

		out = append(out, matches...)
	}

	return out, nil
}

// renderTemplateBody parses and executes a single template file in the
// context of any partials declared for its enclosing template. Mirrors
// render.RenderTemplateWithPartialsWithContext but reads partial contents
// from fs.FS rather than the disk-backed ParseGlob: each partial file is
// parsed as a named template (basename) and added to the template
// namespace alongside the main body. This lets `{{ template "name" . }}`
// invocations resolve against partials that define a named template, and
// `{{ template "filename.tmpl" . }}` invocations resolve against the
// partial's free body — same as ParseGlob does at runtime.
//
// Skip-files and dynamic-filename handling are done by the caller; this
// function is responsible solely for parsing + executing.
func renderTemplateBody(ctx context.Context, loc templateLocation, sourcePath, contents string, scope map[string]any, partialPaths []string) (string, error) {
	opts := &options.BoilerplateOptions{
		TemplateFolder:  loc.absDir,
		NonInteractive:  true,
		NoHooks:         true,
		NoShell:         true,
		OnMissingKey:    options.ExitWithError,
		OnMissingConfig: options.Ignore,
	}

	if len(partialPaths) == 0 {
		return render.RenderTemplateFromStringWithContext(ctx, logging.Discard(), sourcePath, contents, scope, opts)
	}

	// Build the template namespace explicitly so we can attach partials
	// without touching the disk. Use the same helper FuncMap and the same
	// missingkey option the runtime uses, so behavior matches `boilerplate
	// template`.
	missingKey := "missingkey=" + string(opts.OnMissingKey)
	tmpl := template.New(path.Base(sourcePath)).Option(missingKey)
	tmpl = tmpl.Funcs(render.CreateTemplateHelpers(ctx, logging.Discard(), sourcePath, opts, tmpl))

	if _, err := tmpl.Parse(contents); err != nil {
		return "", fmt.Errorf("parsing %s: %w", sourcePath, err)
	}

	for _, pp := range partialPaths {
		pdata, readErr := fs.ReadFile(loc.fsys, pp)
		if readErr != nil {
			return "", fmt.Errorf("reading partial %s: %w", pp, readErr)
		}

		// Each partial file becomes a named template (basename) so that
		// `{{ template "file.tmpl" . }}` resolves to its body — same as
		// ParseGlob at runtime. Any `{{ define "name" }}` blocks inside
		// the partial register their own names in the shared namespace.
		if _, err := tmpl.New(path.Base(pp)).Parse(string(pdata)); err != nil {
			return "", fmt.Errorf("parsing partial %s: %w", pp, err)
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, scope); err != nil {
		return "", fmt.Errorf("executing %s: %w", sourcePath, err)
	}

	return buf.String(), nil
}

// renderForRender renders a small string fragment against scope, using
// missingkey=error semantics so the caller sees the missing-variable error
// directly instead of getting a "<no value>" placeholder. Mirrors
// renderForAnalysis from the analyzer but does NOT scrub <no value>
// strings — at the render path we want real errors.
func renderForRender(ctx context.Context, loc templateLocation, contents string, scope map[string]any) (string, error) {
	if !strings.Contains(contents, "{{") {
		return contents, nil
	}

	opts := &options.BoilerplateOptions{
		TemplateFolder:  loc.absDir,
		NonInteractive:  true,
		NoHooks:         true,
		NoShell:         true,
		OnMissingKey:    options.ExitWithError,
		OnMissingConfig: options.Ignore,
	}

	return render.RenderTemplateFromStringWithContext(ctx, logging.Discard(), "render", contents, scope, opts)
}

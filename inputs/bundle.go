package inputs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"strings"

	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/variables"
)

// eachVarName is the parent-scope key that boilerplate sets to the current
// for_each iteration value before rendering a dep's output-folder /
// template-url / variables block. We mirror the constant here (rather than
// importing templates/ which would create an import cycle) so the bundle
// producer can construct an iteration-aware scope the same way the runtime
// does.
const eachVarName = "__each__"

// Bundle is a snapshot of every text file in the resolved boilerplate
// template tree, keyed by a forward-slash path relative to RootPath. The
// shape mirrors the bundle that boilerplateInputsMap and
// boilerplateRenderFile accept, so callers can pipe the output of
// `boilerplate inputs map --include-bundle` directly into the WASM
// functions without rewriting.
//
// Each dep's files live at a deterministic, bundle-relative directory
// computed by the producer (see ResolvedDep.BundlePath). That directory
// does not — and cannot — mirror the dep's on-disk template-url, because
// a template-url that uses {{ templateFolder }} resolves to an absolute
// disk path that has no defensible meaning inside a virtual filesystem.
type Bundle struct {
	RootPath string            `json:"rootPath"`
	Files    map[string]string `json:"files"`

	// Dependencies maps a parent template's bundle directory ("." for
	// the root, or another ResolvedDep.BundlePath for nested deps) to
	// its ordered list of resolved local deps. Remote deps and
	// unresolvable local deps do NOT appear here; consumers must treat
	// their omission as a hint to force cold render.
	//
	// Required for warm rendering of any template with deps. Older
	// bundles (no Dependencies field) are produced by pre-fix CLIs; the
	// consumer (RenderFileFromFS) refuses to render against them.
	Dependencies map[string][]ResolvedDep `json:"dependencies,omitempty"`
}

// ResolvedDep is the bundle-time-resolved location and output-folder for
// a single declared dependency. The CLI computes these once during
// collectBundle using the parent template's real absDir and writes them
// into the bundle so no consumer ever has to re-render the dep's
// `template-url` (which is meaningless inside an in-memory bundle).
//
// A dep declared with `for_each` (or `for_each_reference`) produces one
// ResolvedDep entry per iteration. The entries share Name and BundlePath
// but each carries its own pre-rendered OutputFolder and the Each value
// the consumer must inject as `__each__` before evaluating that iteration's
// dep-variable defaults.
type ResolvedDep struct {
	Name string `json:"name"`

	// BundlePath is the dep's directory inside Bundle.Files.
	// Forward-slash, strictly-relative, validateBundlePath-clean.
	// Stable regardless of what `{{ templateFolder }}` resolved to at
	// bundle time.
	BundlePath string `json:"bundlePath"`

	// OutputFolder is the dep's `output-folder` field, pre-rendered
	// against the parent scope at bundle time. Consumers use this to
	// compute output paths for files emitted by this dep, exactly like
	// the runtime does. For for_each deps, the parent scope used at
	// render time included `__each__` set to Each below — matching the
	// runtime's per-iteration scope.
	OutputFolder string `json:"outputFolder"`

	// Each is the for_each iteration value carried into the child scope
	// as `__each__`. Empty for non-for_each deps. The consumer must seed
	// this into the parent scope under the `__each__` key before
	// evaluating the dep's variables block — otherwise dep-variable
	// defaults that reference `.__each__` will fail with a missing-key
	// error.
	Each string `json:"each,omitempty"`
}

// ErrRemoteDependencyInBundle is returned when BundleFromOptions
// encounters a remote dependency URL during walking. Warm dispatch cannot
// resolve remote deps (no go-getter in WASM), so the bundle is incomplete
// and the caller should treat the affected outputs as cold-only. The
// error is non-fatal at the bundle level — BundleFromOptions still returns
// a Bundle containing whatever local deps it could resolve — but is
// surfaced so the CLI can include it in the JSON output's existing
// errors[] array.
var ErrRemoteDependencyInBundle = errors.New("remote dependency cannot be bundled")

// bundleDepsDir is the reserved directory name under which every
// dependency's files are placed inside a bundle. Using a fixed prefix
// (rather than the dep's rendered template-url) gives the bundle
// path-context-free addressing — a parent's `{{ templateFolder }}/..`
// no longer leaks into bundle keys.
//
// Templates with a real directory literally named `_deps` would have
// been broken pre-fix anyway (their files would have collided with
// nonsense produced by the old joinBundlePath logic); the producer
// reserves the name and the producer is the only writer to the bundle.
const bundleDepsDir = "_deps"

// BundleFromOptions resolves the root template (via go-getter if needed)
// and walks the dep tree, collecting every boilerplate.yml and every text
// template file. Remote dependencies are NOT followed; they appear as
// dangling references in the bundle. The caller can detect this by
// inspecting BundleResult.RemoteDeps.
//
// Cleanup of any go-getter-fetched temp directories is performed before
// this function returns — Files contains the file contents as strings,
// not references to disk, so the bundle remains usable after cleanup.
func BundleFromOptions(ctx context.Context, l logging.Logger, opts *options.BoilerplateOptions) (*Bundle, []BundleNote, error) {
	rootLoc, cleanup, err := resolveRootLocation(l, opts)
	if cleanup != nil {
		defer cleanup()
	}

	if err != nil {
		return nil, nil, err
	}

	bundle := &Bundle{
		RootPath:     ".",
		Files:        map[string]string{},
		Dependencies: map[string][]ResolvedDep{},
	}

	var (
		notes    []BundleNote
		cleanups []func()
	)

	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()

	resolver := &osResolver{logger: l}
	visiting := map[string]struct{}{}

	if walkErr := collectBundle(ctx, rootLoc, ".", opts.Vars, resolver, bundle, &notes, visiting, &cleanups); walkErr != nil {
		return nil, notes, walkErr
	}

	return bundle, notes, nil
}

// BundleNote is a soft diagnostic surfaced during bundle collection.
// Distinct from the analyzer's AnalysisError because the bundle's failure
// modes are narrower: a dep was remote (skipped), a dep didn't resolve
// locally, or a config failed to parse. The CLI maps these into the
// existing inputs.AnalysisError shape so the JSON contract stays uniform.
type BundleNote struct {
	Kind    string
	Name    string
	Message string
}

// collectBundle reads every file under loc and stores it in bundle.Files
// keyed by bundleDir + rel(loc.dir, file). Recurses into each dep's
// resolved location with bundleDir advanced by a deterministic slug
// (_deps/<name>) that is bundle-relative and independent of what the
// dep's template-url rendered to at bundle time. Remote deps are recorded
// as notes and not followed.
//
// We use a separate walker rather than reusing analyzer's analyzeFiles
// because the bundle needs:
//   - Both text and config files (analyzer skips boilerplate.yml).
//   - No knowledge of variable refs.
//   - Different path keys (bundle paths, not output paths).
//
// vars is best-effort: it's passed to renderForAnalysis when computing
// template-urls so templated deps like `./modules/{{ .Flavor }}` resolve
// when the user has supplied the right vars. Failures fall through to
// using the raw template-url, matching the analyzer's behavior.
func collectBundle(
	ctx context.Context,
	loc templateLocation,
	bundleDir string,
	vars map[string]any,
	resolver dependencyResolver,
	bundle *Bundle,
	notes *[]BundleNote,
	visiting map[string]struct{},
	cleanups *[]func(),
) error {
	walkRoot := loc.dir
	if walkRoot == "" {
		walkRoot = "."
	}

	cfg, cfgErr := loadConfig(loc)
	if cfgErr != nil {
		*notes = append(*notes, BundleNote{
			Kind:    KindParse,
			Message: fmt.Sprintf("could not parse boilerplate.yml at %s: %v", loc.dir, cfgErr),
		})
		// Fall through: still emit whatever files we can find under loc.
		cfg = &config.BoilerplateConfig{}
	}

	// Apply this template's own variable defaults so deeper logic
	// (for_each_reference, dep URL/output-folder rendering, recursive
	// collectBundle calls) sees the same scope the runtime would.
	// Without this, a dep that iterates via `for_each_reference: SomeList`
	// where SomeList is declared in *this* boilerplate.yml's variables
	// block — not in opts.Vars — silently degrades to a single
	// un-iterated entry, and the renderer later blows up evaluating any
	// dep-block default that references `.__each__`.
	//
	// This is intentionally a leniency point: defaults that depend on
	// runtime-only context (helpers like `{{ shell }}`, runtime-resolved
	// `.outputs`, or user inputs that weren't supplied) are skipped so
	// the bundle still gets built for everything else. Strict
	// evaluation continues to happen at render time via the renderer's
	// applyConfigDefaults.
	scope := applyDefaultsForBundle(ctx, loc, cfg, vars)

	// Pre-render dep URLs so we can call resolver.Resolve below and so
	// dependencySkipDirs sees the same string the analyzer does. The
	// rendered URL is used ONLY for resolution / skip-dir computation;
	// it is NOT used to derive bundle keys.
	renderedDepURLs := make([]string, len(cfg.Dependencies))

	for i := range cfg.Dependencies {
		dep := &cfg.Dependencies[i]
		if dep.TemplateURL == "" {
			continue
		}

		rendered, err := renderForAnalysis(ctx, loc.absDir, dep.TemplateURL, scope)
		if err != nil {
			rendered = dep.TemplateURL
		}

		renderedDepURLs[i] = strings.TrimSpace(rendered)
	}

	skipDirs := dependencySkipDirs(loc, cfg, renderedDepURLs)

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

		// Skip any nested boilerplate.yml that belongs to a dep we'll
		// recurse into separately. The parent's own boilerplate.yml IS
		// included (handled below by walking root-level files).
		base := path.Base(p)
		if base == config.BoilerplateConfigFile && p != path.Join(loc.dir, config.BoilerplateConfigFile) {
			return nil
		}

		data, readErr := fs.ReadFile(loc.fsys, p)
		if readErr != nil {
			return readErr
		}

		// Include text files only. The bundle is consumed by warm render,
		// which only ever touches text content; binaries would inflate the
		// transfer size with no upside. Matches analyzer's policy.
		// boilerplate.yml is always included regardless of the isLikelyText
		// heuristic — empty configs are valid and shouldn't be skipped.
		if base != config.BoilerplateConfigFile && !isLikelyText(data) {
			return nil
		}

		rel := slashRel(loc.dir, p)
		bundleKey := joinCleanBundlePath(bundleDir, rel)
		bundle.Files[bundleKey] = string(data)

		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	// Recurse into each local dep. Remote deps are recorded but not
	// followed: warm render will fail-loud (ErrDependencyNotInBundle) if
	// the consumer asks for an output that needs them, which is the right
	// signal for falling back to cold render.
	parentBundleKey := normalizeBundleDir(bundleDir)

	for i := range cfg.Dependencies {
		dep := &cfg.Dependencies[i]
		depURL := renderedDepURLs[i]

		if depURL == "" {
			continue
		}

		if strings.Contains(depURL, "://") {
			*notes = append(*notes, BundleNote{
				Kind:    KindUnresolvableDependency,
				Name:    dep.Name,
				Message: fmt.Sprintf("dependency %q has a remote template-url %q and was not bundled (warm render will fall back to cold for outputs under this dep)", dep.Name, depURL),
			})

			continue
		}

		cycleKey := depURL
		if _, busy := visiting[cycleKey]; busy {
			continue
		}

		visiting[cycleKey] = struct{}{}

		childLoc, depCleanup, resolveErr := resolver.Resolve(ctx, loc, depURL)
		if depCleanup != nil {
			*cleanups = append(*cleanups, depCleanup)
		}

		if resolveErr != nil {
			delete(visiting, cycleKey)
			*notes = append(*notes, BundleNote{
				Kind:    KindUnresolvableDependency,
				Name:    dep.Name,
				Message: resolveErr.Error(),
			})

			continue
		}

		// childBundleDir lives under the parent's bundle directory at a
		// deterministic slug so the resulting key is stable and free of
		// any path arithmetic against the rendered template-url.
		childBundleDir := path.Join(parentBundleKey, bundleDepsDir, sanitizeDepName(dep.Name))

		// Compute the dep's for_each iteration values (if any) against
		// the parent scope. A dep with no iteration produces a single
		// ResolvedDep entry with Each="" — the existing pre-fix shape.
		// A dep with N iterations produces N entries sharing Name +
		// BundlePath but each carrying its own pre-rendered
		// OutputFolder and the Each value the renderer must seed as
		// `__each__` before evaluating dep-variable defaults.
		forEachItems, feErr := resolveForEach(ctx, loc, dep, scope)
		if feErr != nil {
			// A broken for_each expression is a soft failure: record it
			// and emit a single un-iterated entry so warm dispatch can
			// at least route per the consumer's normal fall-back rules.
			*notes = append(*notes, BundleNote{
				Kind:    KindUnresolvableDependency,
				Name:    dep.Name,
				Message: fmt.Sprintf("resolving for_each for dependency %q: %v", dep.Name, feErr),
			})

			forEachItems = nil
		}

		// Build the iteration list. A dep without for_each gets a
		// single nil-Each pass so the code path below stays uniform.
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

			// Pre-render the dep's output-folder against the (possibly
			// __each__-seeded) parent scope. Doing it once here, at
			// bundle time, means the consumer never has to re-render
			// an expression in a context where the templating helpers
			// (notably {{ templateFolder }}) have lost their meaning.
			renderedOutputFolder, ofErr := renderForAnalysis(ctx, loc.absDir, dep.OutputFolder, renderVars)
			if ofErr != nil {
				renderedOutputFolder = dep.OutputFolder
			}

			renderedOutputFolder = strings.TrimSpace(renderedOutputFolder)

			entry := ResolvedDep{
				Name:         dep.Name,
				BundlePath:   childBundleDir,
				OutputFolder: renderedOutputFolder,
			}

			if isForEach {
				entry.Each = item
			}

			bundle.Dependencies[parentBundleKey] = append(bundle.Dependencies[parentBundleKey], entry)
		}

		// Recurse into the dep's bundle directory once: the file tree
		// is identical across for_each iterations (only the rendered
		// output-folder varies, and that's already captured above).
		// Threading `scope` (not `vars`) here lets deeper deps' own
		// for_each_reference / template-url / output-folder expressions
		// resolve against this template's declared defaults — same as
		// the runtime, where a child sees its parent's resolved
		// variable values.
		if recurErr := collectBundle(ctx, childLoc, childBundleDir, scope, resolver, bundle, notes, visiting, cleanups); recurErr != nil {
			delete(visiting, cycleKey)

			return recurErr
		}

		delete(visiting, cycleKey)
	}

	return nil
}

// applyDefaultsForBundle layers a template's variable defaults onto the
// caller-supplied vars map and returns the combined scope. It mirrors
// the renderer's applyConfigDefaults but is intentionally lenient: a
// default that fails to render (because it depends on runtime-only
// helpers or vars the user hasn't supplied) is silently skipped rather
// than aborting bundle collection. The renderer evaluates defaults
// strictly at render time, so any value that survives until then is
// the user's problem; what matters here is that simple, static defaults
// — most importantly, list variables that feed `for_each_reference`
// expressions in declared dependencies — are present in scope when
// collectBundle processes deeper deps.
//
// The input map is not mutated. Caller-supplied entries always win over
// the config's defaults (same precedence the runtime uses for
// non-interactive renders).
func applyDefaultsForBundle(ctx context.Context, loc templateLocation, cfg *config.BoilerplateConfig, parentVars map[string]any) map[string]any {
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

		rendered, err := renderDefaultValue(ctx, loc, def, out)
		if err != nil {
			// Lenient by design: this default depends on something the
			// producer can't resolve (a runtime helper, an unset user
			// var, etc.). Leaving the variable unset is correct — the
			// renderer will re-evaluate at render time and either
			// succeed (if it has more context) or surface the error
			// then.
			continue
		}

		typed, typeErr := variables.ConvertType(rendered, v)
		if typeErr != nil {
			continue
		}

		out[v.Name()] = typed
	}

	return out
}

// resolveForEach computes the for_each iteration list for a dep, mirroring
// the runtime's logic in templates.processDependency. Static for_each lists
// are returned as-is; a for_each_reference is rendered against parentVars
// (it evaluates to a variable name), and the named variable's value is
// expected to be a string list in parentVars.
//
// Returns (nil, nil) for a dep with no iteration — collectBundle treats
// that as the "single un-iterated entry" case.
func resolveForEach(ctx context.Context, loc templateLocation, dep *variables.Dependency, parentVars map[string]any) ([]string, error) {
	if len(dep.ForEachReference) > 0 {
		renderedReference, renderErr := renderForAnalysis(ctx, loc.absDir, dep.ForEachReference, parentVars)
		if renderErr != nil {
			return nil, fmt.Errorf("rendering for_each_reference: %w", renderErr)
		}

		renderedReference = strings.TrimSpace(renderedReference)
		if renderedReference == "" {
			return nil, nil
		}

		value, unmarshalErr := variables.UnmarshalListOfStrings(parentVars, renderedReference)
		if unmarshalErr != nil {
			return nil, fmt.Errorf("looking up for_each_reference %q in vars: %w", renderedReference, unmarshalErr)
		}

		return value, nil
	}

	return dep.ForEach, nil
}

// joinCleanBundlePath joins a parent bundle directory (already free of any
// disk-path-shaped content) with a clean relative child path. Both inputs
// are forward-slash and validateBundlePath-clean by construction; this
// helper just normalises "." and joins.
func joinCleanBundlePath(parent, child string) string {
	parent = normalizeBundleDir(parent)
	child = strings.TrimPrefix(strings.TrimSpace(child), "./")

	joined := path.Join(parent, child)

	if joined == "" {
		return "."
	}

	return joined
}

// normalizeBundleDir collapses "" / "./" / "." to the canonical root key
// "." that bundle.Dependencies and bundle.Files use.
func normalizeBundleDir(dir string) string {
	dir = strings.TrimSpace(dir)
	dir = strings.TrimPrefix(dir, "./")

	if dir == "" || dir == "." {
		return "."
	}

	return dir
}

// sanitizeDepName turns a dependency name into a stable, filesystem-safe
// slug suitable for a bundle path segment. Anything outside
// [A-Za-z0-9._-] is replaced with `_`. Empty names fall back to a stable
// placeholder so we still emit a usable bundle key.
func sanitizeDepName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "_unnamed"
	}

	var b strings.Builder

	b.Grow(len(name))

	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	return b.String()
}

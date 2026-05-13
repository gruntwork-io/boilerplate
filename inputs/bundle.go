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

// eachVarName mirrors templates.eachVarName — keep in sync.
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
	Files map[string]string `json:"files"`

	// Dependencies maps a parent template's bundle directory ("." for
	// the root, or another ResolvedDep.BundlePath for nested deps) to
	// its ordered list of resolved local deps. Remote deps and
	// unresolvable local deps do NOT appear here; consumers must treat
	// their omission as a hint to force cold render.
	//
	// Required for warm rendering of any template with deps.
	// RenderFileFromFS rejects bundles missing this field.
	Dependencies map[string][]ResolvedDep `json:"dependencies,omitempty"`

	RootPath string `json:"rootPath"`
}

// ResolvedDep is the bundle-time-resolved location and output-folder for one
// declared dependency. A for_each dep produces one entry per iteration,
// sharing Name + BundlePath but with distinct OutputFolder + Each.
type ResolvedDep struct {
	Name string `json:"name"`

	// BundlePath is the dep's directory inside Bundle.Files.
	// Forward-slash, strictly-relative, validateBundlePath-clean.
	BundlePath string `json:"bundlePath"`

	// OutputFolder is pre-rendered against the parent scope at bundle time.
	OutputFolder string `json:"outputFolder"`

	// Each is the for_each iteration value the consumer must seed into the
	// parent scope as `__each__` before evaluating dep-variable defaults.
	// Empty for non-for_each deps.
	Each string `json:"each,omitempty"`
}

// ErrRemoteDependencyInBundle is non-fatal at the bundle level: the bundle
// is returned with whatever local deps did resolve, and the caller treats
// the affected outputs as cold-only.
var ErrRemoteDependencyInBundle = errors.New("remote dependency cannot be bundled")

// bundleDepsDir is the reserved directory holding every dependency's files
// inside a bundle. A fixed prefix keeps bundle keys free of any leakage
// from `{{ templateFolder }}` in the parent.
const bundleDepsDir = "_deps"

// BundleFromOptions resolves the root template (via go-getter if needed) and
// walks the dep tree, collecting every boilerplate.yml and text template file.
// Remote dependencies are not followed. Any go-getter temp directories are
// cleaned up before returning; Files holds contents as strings, so the bundle
// remains usable.
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

// BundleNote is a soft diagnostic surfaced during bundle collection. The CLI
// maps these into inputs.AnalysisError so the JSON contract stays uniform.
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
	// Lenient: defaults that depend on runtime-only context are skipped
	// so the bundle still builds. Strict evaluation happens at render time.
	scope, _ := applyConfigDefaults(ctx, loc, cfg, vars, true)

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

		cycleKey := path.Clean(path.Join(loc.dir, depURL))
		if _, busy := visiting[cycleKey]; busy {
			*notes = append(*notes, BundleNote{
				Kind:    KindCycle,
				Name:    dep.Name,
				Message: fmt.Sprintf("dependency %q forms a cycle (template-url=%s); skipped during bundling", dep.Name, depURL),
			})

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

		childBundleDir := path.Join(parentBundleKey, bundleDepsDir, sanitizeDepName(dep.Name))

		// A dep with no iteration produces a single ResolvedDep entry with
		// Each=""; N iterations produce N entries sharing Name + BundlePath
		// but each carrying its own OutputFolder + Each.
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

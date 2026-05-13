package inputs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/variables"
)

// templateInfo holds analysis output for a single template (one boilerplate.yml).
type templateInfo struct {
	cfg      *config.BoilerplateConfig
	declared map[string]variables.Variable
	fileRefs map[string]map[string]struct{}
	// fileSources maps each output path produced by this template to the
	// source template file that produced it. In OS mode the values are
	// absolute disk paths; in FS mode they are slash-separated paths within
	// loc.fsys. Output paths whose filename template failed to render are
	// intentionally absent so callers can detect the dynamic-path case via
	// the existing KindFilenameRender soft error.
	fileSources   map[string]string
	skipFilesRefs map[string]struct{}
	loc           templateLocation
	outputPath    string
	declaredIn    string
	deps          []resolvedDep
}

// resolvedDep bundles every piece of state for a dependency that successfully
// resolved and was recursed into. Failed deps (cycles, unresolvable URLs,
// recursion errors) are recorded as soft errors but do NOT produce a
// resolvedDep entry, so deps[i] always refers to a fully-analyzed child.
type resolvedDep struct {
	child    *templateInfo
	cfg      *variables.Dependency
	edges    map[string]map[string]struct{} // childVar -> parent vars referenced in its value expressions
	pathRefs map[string]struct{}            // parent vars in dep.template-url / dep.output-folder
}

func analyze(ctx context.Context, root templateLocation, vars map[string]any, resolver dependencyResolver) (*Result, error) {
	result := &Result{
		Inputs:  map[string]InputEntry{},
		Files:   map[string][]string{},
		Sources: map[string]string{},
		Errors:  []AnalysisError{},
	}

	cleanups := []func(){}

	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()

	visiting := map[string]struct{}{}

	rootInfo, err := analyzeTree(ctx, root, ".", ".", vars, resolver, result, visiting, &cleanups)
	if err != nil {
		return nil, err
	}

	composeResult(rootInfo, result)
	finalizeResult(result)

	return result, nil
}

// analyzeTree recursively analyzes one template and all its dependencies.
// declaredIn is the path used in input keys; outputPath is the path where the
// template's output goes. They are kept separate because the spec emits
// declaredIn="." for the root, and declaredIn for deps is the path of the
// declaring template's output folder relative to root output.
//
// visiting tracks template URL strings currently in the active DFS path so we
// can detect cycles in the dependency graph.
func analyzeTree(
	ctx context.Context,
	loc templateLocation,
	outputPath string,
	declaredIn string,
	vars map[string]any,
	resolver dependencyResolver,
	result *Result,
	visiting map[string]struct{},
	cleanups *[]func(),
) (*templateInfo, error) {
	cfg, err := loadConfig(loc)
	if err != nil {
		return nil, err
	}

	info := &templateInfo{
		loc:         loc,
		outputPath:  outputPath,
		declaredIn:  declaredIn,
		cfg:         cfg,
		declared:    make(map[string]variables.Variable, len(cfg.Variables)),
		fileRefs:    map[string]map[string]struct{}{},
		fileSources: map[string]string{},
	}

	for _, v := range cfg.Variables {
		info.declared[v.Name()] = v
	}

	// Partials must be parsed before files so {{ template "name" }} invocations
	// can be expanded transitively into the per-file ref sets.
	partialRefs, partialErrs := analyzePartials(ctx, loc, cfg.Partials, vars)
	result.Errors = append(result.Errors, partialErrs...)

	// Render every dependency's template-url once, so analyzeFiles' skip-dir
	// computation and the recursion loop below see the same path. Without
	// this, a templated template-url (e.g. `./modules/{{ .Flavor }}`) would
	// fail to match an actual subdirectory in dependencySkipDirs and the
	// walker would treat the child's files as if they belonged to the
	// parent.
	renderedDepURLs := preRenderDepURLs(ctx, loc, cfg.Dependencies, vars, declaredIn, result)

	if walkErr := analyzeFiles(ctx, loc, info, partialRefs, vars, renderedDepURLs, result); walkErr != nil {
		return nil, walkErr
	}

	for i := range cfg.Dependencies {
		dep := &cfg.Dependencies[i]

		renderedURL := renderedDepURLs[i]

		// If the rendered URL is empty, the template-url was either blank or
		// composed entirely of missing-variable placeholders that scrubbed
		// down to nothing. There is no useful path to resolve; record a soft
		// error and skip without attempting a stat.
		if renderedURL == "" {
			info.recordDepFailure(result, KindUnresolvableDependency, declaredIn, dep.Name,
				fmt.Sprintf("template-url for dependency %q rendered to empty string (likely missing input variables)", dep.Name))

			continue
		}

		renderedOutputFolder, renderErr := renderForAnalysis(ctx, loc.absDir, dep.OutputFolder, vars)
		if renderErr != nil {
			result.Errors = append(result.Errors, AnalysisError{
				Kind:     KindFilenameRender,
				Template: declaredIn,
				Name:     dep.Name,
				Message:  fmt.Sprintf("could not render output-folder for dependency %q: %v", dep.Name, renderErr),
			})

			renderedOutputFolder = dep.OutputFolder
		}

		cycleKey := computeCycleKey(loc, renderedURL)

		if _, busy := visiting[cycleKey]; busy {
			info.recordDepFailure(result, KindCycle, declaredIn, dep.Name,
				fmt.Sprintf("dependency %q forms a cycle (template-url=%s)", dep.Name, renderedURL))

			continue
		}

		visiting[cycleKey] = struct{}{}

		childLoc, depCleanup, resolveErr := resolver.Resolve(ctx, loc, renderedURL)
		if depCleanup != nil {
			*cleanups = append(*cleanups, depCleanup)
		}

		if resolveErr != nil {
			delete(visiting, cycleKey)
			info.recordDepFailure(result, KindUnresolvableDependency, declaredIn, dep.Name, resolveErr.Error())

			continue
		}

		childOutputPath := joinOutputPath(outputPath, renderedOutputFolder)
		childDeclaredIn := joinOutputPath(declaredIn, renderedOutputFolder)

		childInfo, depErr := analyzeTree(ctx, childLoc, childOutputPath, childDeclaredIn, vars, resolver, result, visiting, cleanups)
		delete(visiting, cycleKey)

		if depErr != nil {
			// Treat dependency analysis failure as a soft error so the rest
			// of the result is still useful.
			result.Errors = append(result.Errors, AnalysisError{
				Kind:     KindParse,
				Template: childDeclaredIn,
				Name:     dep.Name,
				Message:  depErr.Error(),
			})

			continue
		}

		info.deps = append(info.deps, resolvedDep{
			child:    childInfo,
			cfg:      dep,
			edges:    computeDepEdges(dep, info.declared, declaredIn, result),
			pathRefs: computeDepPathRefs(dep),
		})
	}

	return info, nil
}

// preRenderDepURLs renders each dependency's template-url against the
// parent's vars. Failures are recorded once as soft errors here; the caller
// reuses the resulting slice for both dependencySkipDirs and the dependency
// recursion loop. On render failure, the entry falls back to the raw URL so
// downstream behavior matches what the runtime would do when faced with the
// same unrenderable string.
//
// The returned slice is always len(deps) long; entries for empty
// template-urls are the empty string. Callers handle that.
func preRenderDepURLs(ctx context.Context, loc templateLocation, deps []variables.Dependency, vars map[string]any, declaredIn string, result *Result) []string {
	out := make([]string, len(deps))

	for i := range deps {
		dep := &deps[i]
		if dep.TemplateURL == "" {
			continue
		}

		rendered, err := renderForAnalysis(ctx, loc.absDir, dep.TemplateURL, vars)
		if err != nil {
			result.Errors = append(result.Errors, AnalysisError{
				Kind:     KindFilenameRender,
				Template: declaredIn,
				Name:     dep.Name,
				Message:  fmt.Sprintf("could not render template-url for dependency %q: %v", dep.Name, err),
			})

			rendered = dep.TemplateURL
		}

		out[i] = strings.TrimSpace(rendered)
	}

	return out
}

// computeCycleKey returns a stable identifier for a dependency target,
// computable before the dep is resolved so the cycle check can short-circuit
// before calling resolver.Resolve. The key incorporates the parent's
// absolute disk path (OS mode) or fs-relative path (FS mode) so two
// distinct on-disk siblings whose template-url values clean to the same
// relative string produce distinct keys — without this, OS-mode
// `loc.dir == "."` would make every relative URL collide with itself
// across parents and false-detect cycles. Remote URLs (with a scheme) keep
// their raw form since path.Join would mangle them.
func computeCycleKey(parent templateLocation, renderedURL string) string {
	if strings.Contains(renderedURL, "://") {
		return renderedURL
	}

	if filepath.IsAbs(renderedURL) {
		return filepath.Clean(renderedURL)
	}

	if parent.absDir != "" {
		return filepath.Clean(filepath.Join(parent.absDir, renderedURL))
	}

	return path.Clean(path.Join(parent.dir, renderedURL))
}

// recordDepFailure records a soft error for a dependency that couldn't be
// processed. Failed deps are absent from info.deps; nothing else needs to stay
// aligned.
func (info *templateInfo) recordDepFailure(result *Result, kind, declaredIn, depName, message string) {
	result.Errors = append(result.Errors, AnalysisError{
		Kind:     kind,
		Template: declaredIn,
		Name:     depName,
		Message:  message,
	})
}

// loadConfig reads and parses boilerplate.yml from loc.
func loadConfig(loc templateLocation) (*config.BoilerplateConfig, error) {
	cfgPath := path.Join(loc.dir, config.BoilerplateConfigFile)

	data, err := fs.ReadFile(loc.fsys, cfgPath)
	if err != nil {
		// If the file does not exist, treat as a config-less template (no
		// inputs). Mirrors options.Ignore behavior.
		if errors.Is(err, fs.ErrNotExist) {
			return &config.BoilerplateConfig{}, nil
		}

		return nil, fmt.Errorf("read %s: %w", cfgPath, err)
	}

	return config.ParseBoilerplateConfig(data)
}

// analyzePartials parses every partial glob in the config and returns a map
// from defined-template-name to that template's body refs.
func analyzePartials(ctx context.Context, loc templateLocation, partialGlobs []string, vars map[string]any) (map[string]*templateRefs, []AnalysisError) {
	out := map[string]*templateRefs{}

	var errs []AnalysisError

	for _, glob := range partialGlobs {
		// Render the glob with vars (best effort) to handle any embedded
		// template syntax.
		rendered, err := renderForAnalysis(ctx, loc.absDir, glob, vars)
		if err != nil {
			rendered = glob
		}

		files, listErr := globPartialFiles(loc, rendered)
		if listErr != nil {
			errs = append(errs, AnalysisError{
				Kind:    KindParse,
				Message: fmt.Sprintf("could not resolve partial glob %q: %v", glob, listErr),
			})

			continue
		}

		for _, pf := range files {
			trees, parseErr := parseTemplateAll(pf.name, pf.contents)
			if parseErr != nil {
				errs = append(errs, AnalysisError{
					Kind:    KindParse,
					File:    pf.name,
					Message: parseErr.Error(),
				})

				continue
			}

			for _, t := range trees {
				if t.Name == "" {
					continue
				}

				refs := newTemplateRefs()
				walkTree(t, refs)

				out[t.Name] = refs
			}
		}
	}

	// After collecting per-template refs, expand transitively: if partial A
	// invokes partial B and B references X, A references X too.
	if !expandPartialRefs(out, partialExpansionMaxIterations) {
		errs = append(errs, AnalysisError{
			Kind: KindPartialExpansionLimit,
			Message: fmt.Sprintf(
				"partial-template invocation graph did not reach a fixed point within %d iterations; transitive references through deeply-nested partials may be missing from the result",
				partialExpansionMaxIterations,
			),
		})
	}

	return out, errs
}

// partialExpansionMaxIterations bounds the number of fixed-point passes
// expandPartialRefs may take. In practice the loop converges in a handful of
// iterations even for complex templates; the cap is a safety net to keep a
// bug or pathological input from spinning forever. Declared as a var so
// tests can lower it to drive the cap-hit branch without constructing a
// fixture with 100+ chained partials.
var partialExpansionMaxIterations = 100

// expandPartialRefs computes the transitive closure of partial -> partial
// invocations, so each entry in m has all variables it could reference if
// expanded. Returns true if the graph reached a fixed point within
// maxIterations passes, false if the cap was hit (in which case the result
// may be incomplete and the caller should surface a soft error).
func expandPartialRefs(m map[string]*templateRefs, maxIterations int) bool {
	for i := 0; i < maxIterations; i++ {
		changed := false

		for _, refs := range m {
			for inv := range refs.invocations {
				other, ok := m[inv]
				if !ok {
					continue
				}

				for v := range other.vars {
					if _, exists := refs.vars[v]; !exists {
						refs.vars[v] = struct{}{}
						changed = true
					}
				}

				for innerInv := range other.invocations {
					if _, exists := refs.invocations[innerInv]; !exists {
						refs.invocations[innerInv] = struct{}{}
						changed = true
					}
				}
			}
		}

		if !changed {
			return true
		}
	}

	return false
}

// partialFile is a parsed partial file's name + contents.
type partialFile struct {
	name     string
	contents string
}

// globPartialFiles resolves a partial glob pattern and reads each matching
// file. Behavior depends on whether loc is on disk or in-memory:
//
//   - On disk (loc.absDir != ""): use filepath.Glob against the absolute
//     joined path. This supports `..` patterns escaping the template folder,
//     matching boilerplate's runtime behavior.
//   - In-memory (loc.absDir == ""): use fs.Glob against the rooted FS. The
//     pattern is interpreted relative to loc.dir; `..` patterns are not
//     supported and will return an empty list.
func globPartialFiles(loc templateLocation, pattern string) ([]partialFile, error) {
	if loc.absDir != "" {
		// Resolve relative to the template folder, like
		// render.PathRelativeToTemplate does.
		resolved := pattern
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(loc.absDir, resolved)
		}

		matches, err := filepath.Glob(resolved)
		if err != nil {
			return nil, err
		}

		out := make([]partialFile, 0, len(matches))

		for _, m := range matches {
			data, readErr := os.ReadFile(m)
			if readErr != nil {
				return nil, readErr
			}

			out = append(out, partialFile{name: m, contents: string(data)})
		}

		return out, nil
	}

	// FS mode. Reject patterns whose cleaned join either escapes the FS
	// root (`..` segments) or attempts an absolute lookup. The previous
	// `HasPrefix(joined, "..")` check both rejected legitimate filenames
	// like `..eslintrc.tmpl` and accepted absolute patterns like
	// `/etc/passwd` (since path.Join("dir","/etc/passwd") yields
	// "/etc/passwd").
	joined := path.Join(loc.dir, pattern)
	cleaned := path.Clean(joined)

	if strings.HasPrefix(pattern, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return nil, fmt.Errorf("partial pattern %q escapes the FS root and is not supported in FS mode", pattern)
	}

	matches, err := fs.Glob(loc.fsys, joined)
	if err != nil {
		return nil, err
	}

	out := make([]partialFile, 0, len(matches))

	for _, m := range matches {
		data, readErr := fs.ReadFile(loc.fsys, m)
		if readErr != nil {
			return nil, readErr
		}

		out = append(out, partialFile{name: m, contents: string(data)})
	}

	return out, nil
}

// analyzeFiles walks every file under loc.dir, parses text files, and records
// their referenced variables in info.fileRefs.
//
// In FS mode (where dependencies share the same fs.FS as the parent), the
// walker would otherwise recurse into dependency subdirectories and treat
// their files as the parent's templates. To prevent this, we compute the set
// of subdirectories that hold local dependencies and skip them during the
// walk; each dep gets analyzed independently when its turn comes via
// analyzeTree.
func analyzeFiles(ctx context.Context, loc templateLocation, info *templateInfo, partialRefs map[string]*templateRefs, vars map[string]any, renderedDepURLs []string, result *Result) error {
	walkRoot := loc.dir
	if walkRoot == "" {
		walkRoot = "."
	}

	cfgPath := path.Join(loc.dir, config.BoilerplateConfigFile)
	skipDirs := dependencySkipDirs(loc, info.cfg, renderedDepURLs)

	// Evaluate skip rules against the template's resolved scope so a
	// condition like `if: '{{ eq .Mode "strict" }}'` sees defaults declared
	// in boilerplate.yml — matching what render_file.go does. Lenient mode
	// drops defaults that fail to render rather than aborting analysis; a
	// rule whose `if` references such a default falls back to inactive,
	// which is the correct conservative default for the analyzer.
	scope, _ := applyConfigDefaults(ctx, loc, info.cfg, vars, true)

	skipFilter, skipErrs := processSkipFiles(ctx, loc, info.cfg.SkipFiles, scope)
	result.Errors = append(result.Errors, skipErrs...)

	// Record vars referenced in any skip_files expression so a change to
	// them links to every file this template would produce.
	info.skipFilesRefs = computeSkipFilesRefs(info.cfg.SkipFiles)

	return fs.WalkDir(loc.fsys, walkRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Don't descend into a child dependency's directory; it will be
			// analyzed separately as its own template.
			if _, skip := skipDirs[p]; skip && p != walkRoot {
				return fs.SkipDir
			}

			return nil
		}

		// Skip the boilerplate.yml itself.
		if p == cfgPath {
			return nil
		}

		// Skip any boilerplate.yml in a subdirectory — those belong to nested
		// templates (typical when fixtures live alongside the parent in FS
		// mode), not to this template's body. fs.WalkDir always reports
		// slash-separated paths, so use path.Base, not filepath.Base.
		if path.Base(p) == config.BoilerplateConfigFile {
			return nil
		}

		// Honor skip_files: any file the runtime would exclude from the
		// rendered output is also excluded from the analyzer's results.
		if skipFilter.shouldSkip(slashRel(loc.dir, p)) {
			return nil
		}

		// Read once: we need to detect text-ness and parse from the same
		// bytes. fs.FS does not expose mime detection, so use a content-based
		// heuristic (NUL byte presence and UTF-8 validity).
		data, readErr := fs.ReadFile(loc.fsys, p)
		if readErr != nil {
			return readErr
		}

		if !isLikelyText(data) {
			// Binary file; boilerplate copies it as-is at render time. Its
			// CONTENTS are not affected by any input variable, but its PATH
			// may still depend on one (e.g., logo at `{{ .Brand }}/logo.png`),
			// in which case a change to that var relocates the file.
			outPath, filenameOK := computeOutputPath(ctx, loc, info, p, vars, result)
			if outPath != "" {
				if _, exists := info.fileRefs[outPath]; !exists {
					info.fileRefs[outPath] = map[string]struct{}{}
				}

				for v := range extractFilenameRefs(loc, p, result) {
					info.fileRefs[outPath][v] = struct{}{}
				}

				if filenameOK {
					info.fileSources[outPath] = sourcePathFor(loc, p)
				}
			}

			return nil
		}

		refs, parseErr := extractRefs(p, string(data))
		if parseErr != nil {
			result.Errors = append(result.Errors, AnalysisError{
				Kind:    KindParse,
				File:    p,
				Message: parseErr.Error(),
			})

			// Soft error: record and continue the walk rather than aborting.
			return nil //nolint:nilerr
		}

		// Expand template invocations through partials.
		for inv := range refs.invocations {
			if other, ok := partialRefs[inv]; ok {
				for v := range other.vars {
					refs.vars[v] = struct{}{}
				}
			}
		}

		// Variables referenced in the file's PATH (not just its body) also
		// affect the rendered output: changing such a var moves where the
		// file lands. Union them into the same ref set so they propagate
		// through the rest of the analysis identically to body refs.
		for v := range extractFilenameRefs(loc, p, result) {
			refs.vars[v] = struct{}{}
		}

		outPath, filenameOK := computeOutputPath(ctx, loc, info, p, vars, result)
		if outPath == "" {
			return nil
		}

		if _, exists := info.fileRefs[outPath]; !exists {
			info.fileRefs[outPath] = map[string]struct{}{}
		}

		for v := range refs.vars {
			info.fileRefs[outPath][v] = struct{}{}
		}

		if filenameOK {
			info.fileSources[outPath] = sourcePathFor(loc, p)
		}

		return nil
	})
}

// dependencySkipDirs returns the set of directory paths inside loc.fsys that
// hold a local dependency template. The walk in analyzeFiles uses this set to
// avoid descending into nested templates — those are analyzed separately by
// analyzeTree as their own units.
//
// renderedURLs is parallel to cfg.Dependencies; using the rendered (rather
// than raw) URL is what makes templated template-urls like
// `./modules/{{ .Flavor }}` match the actual on-disk subdirectory. Only
// local relative dependencies (template-url that is a relative path, not a
// URL scheme) contribute to the skip set; remote dependencies live outside
// the FS and never overlap with the walk anyway.
func dependencySkipDirs(loc templateLocation, cfg *config.BoilerplateConfig, renderedURLs []string) map[string]struct{} {
	out := map[string]struct{}{}
	if cfg == nil {
		return out
	}

	for i := range cfg.Dependencies {
		url := renderedURLs[i]
		if url == "" || strings.Contains(url, "://") {
			continue
		}

		// Skip absolute paths: fs.WalkDir reports relative paths, so an
		// absolute skip-dir entry would silently never match. Absolute URLs
		// commonly come from `{{ templateFolder }}/../sibling`-style rendering
		// in OS mode, where the dep's files live outside loc anyway and the
		// walk would not see them. In FS mode an absolute path is invalid
		// regardless. Either way, recording the entry is a no-op at best and
		// misleading at worst.
		if path.IsAbs(url) || filepath.IsAbs(url) {
			continue
		}

		// Resolve relative to loc.dir using slash-separated paths so the
		// result matches what fs.WalkDir reports.
		p := path.Clean(path.Join(loc.dir, url))
		if p == "" || p == "." {
			continue
		}

		out[p] = struct{}{}
	}

	return out
}

// extractFilenameRefs returns the set of variable names referenced in
// inputPath's filename relative to loc.dir. Returns nil if the filename has
// no template syntax. Soft parse errors land in result.Errors. The path is
// URL-decoded the same way computeOutputPath does so the two stay in sync —
// boilerplate fixtures encode `|` as %7C/%7c so the literal can survive
// Windows filesystems, and we want to extract refs from the decoded form.
func extractFilenameRefs(loc templateLocation, inputPath string, result *Result) map[string]struct{} {
	rel := slashRel(loc.dir, inputPath)
	if decoded, decodeErr := url.QueryUnescape(rel); decodeErr == nil {
		rel = decoded
	}

	if !strings.Contains(rel, "{{") {
		return nil
	}

	refs, err := extractRefs("filename:"+inputPath, rel)
	if err != nil {
		result.Errors = append(result.Errors, AnalysisError{
			Kind:    KindParse,
			File:    inputPath,
			Message: err.Error(),
		})

		return nil
	}

	return refs.vars
}

// computeOutputPath maps an input file path inside loc.fsys to its output path
// relative to the root output root. The filename portion may itself contain
// template syntax; we render it best-effort.
//
// Returns the joined output path plus a flag indicating whether the filename
// rendered cleanly. The flag is false when renderForAnalysis returned an
// error (and a KindFilenameRender entry was pushed onto result.Errors). The
// caller uses it to decide whether to record a source mapping for this file;
// dynamic-path files are omitted from Sources so consumers can detect them
// via the existing soft-error entry rather than receiving a fabricated path.
//
// inputPath comes from fs.WalkDir, which always uses slash separators
// regardless of OS, and loc.dir is similarly slash-separated. So path
// manipulation here uses the path package, not path/filepath, to avoid
// breaking on Windows where filepath uses backslash.
func computeOutputPath(ctx context.Context, loc templateLocation, info *templateInfo, inputPath string, vars map[string]any, result *Result) (string, bool) {
	// Path of the file relative to the template directory.
	rel := slashRel(loc.dir, inputPath)

	// Match the runtime: `|` is an illegal filename character on Windows, so
	// boilerplate fixtures URL-encode pipes as %7C / %7c. Decode before
	// rendering so the template syntax round-trips.
	if decoded, decodeErr := url.QueryUnescape(rel); decodeErr == nil {
		rel = decoded
	}

	// Render any template syntax in the filename. Pass the file's basename as
	// the template name for diagnostic messages.
	rendered, err := renderForAnalysis(ctx, loc.absDir, rel, vars)
	if err != nil {
		result.Errors = append(result.Errors, AnalysisError{
			Kind:    KindFilenameRender,
			File:    inputPath,
			Message: err.Error(),
		})

		rendered = rel

		return joinOutputPath(info.outputPath, rendered), false
	}

	// Join with the template's output path within the root output tree.
	return joinOutputPath(info.outputPath, rendered), true
}

// sourcePathFor returns the path to record in Result.Sources for a file
// found at inputPath inside loc.fsys. In OS mode this is an absolute disk
// path, suitable for consumers that read the template body directly. In FS
// mode loc.absDir is empty and we fall back to the slash-separated FS path,
// which is the only meaningful identifier available without an on-disk root.
func sourcePathFor(loc templateLocation, inputPath string) string {
	if loc.absDir == "" {
		return inputPath
	}

	return filepath.Join(loc.absDir, filepath.FromSlash(slashRel(loc.dir, inputPath)))
}

// joinOutputPath joins a parent output path with a child relative path,
// producing a slash-separated path relative to the root output root. Returns
// "." for the root only when both inputs are root-equivalent.
//
// Output paths are always relative; a leading "/" on either input is treated
// as a stray (commonly produced by stripping a leading "<no value>" from a
// rendered output-folder) and removed, since an absolute path here would
// bubble through to file keys and break the inverse index.
func joinOutputPath(parent, child string) string {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)

	parent = strings.TrimLeft(parent, "/")
	child = strings.TrimLeft(child, "/")

	if parent == "" || parent == "." {
		parent = ""
	}

	// Strip any leading "./" from child since path.Join handles it but we want
	// stable normalized output.
	child = strings.TrimPrefix(child, "./")
	if child == "." {
		child = ""
	}

	joined := path.Join(parent, child)

	if joined == "" || joined == "." {
		if parent == "" && child == "" {
			return "."
		}

		return joined
	}

	return joined
}

// analysisOutputSentinel is the OutputFolder value passed to the renderer
// during analysis. The `outputFolder` template helper resolves it via
// filepath.Abs, which returns cwd + sentinel; we strip that resolved form
// from the rendered output and replace it with ".". Without this, a
// dependency declared as `output-folder: "{{ outputFolder }}"` would render
// to the analyzer process's working directory, polluting every downstream
// path computation with that absolute prefix.
const analysisOutputSentinel = "__boilerplate_analysis_output_sentinel__"

// analysisOutputSentinelAbs returns the absolute form of analysisOutputSentinel
// (i.e., cwd + sentinel) that the `outputFolder` helper produces at render
// time. Cached because filepath.Abs allocates and renderForAnalysis runs
// hundreds of times per analyze() call.
var analysisOutputSentinelAbs = sync.OnceValue(func() string {
	abs, err := filepath.Abs(analysisOutputSentinel)
	if err != nil {
		return ""
	}

	return abs
})

// missingValueLiteral is what Go's text/template prints for a missing key
// when OnMissingKey=zero is used with map[string]any vars. This is a
// documented quirk of text/template (see render/render_template_test.go);
// for analysis purposes we treat missing values as empty strings.
const missingValueLiteral = "<no value>"

// renderForAnalysis renders a small path-shaped template fragment (filename,
// glob, or dep field) using the existing render pipeline with shell and
// hooks disabled and missing-key behavior set to zero-value. This keeps side
// effects out of the analysis path while still matching what
// `boilerplate template` would compute for the same inputs.
//
// The result is post-processed to (a) collapse the analysis output sentinel
// back to "." so paths produced via {{ outputFolder }} stay relative, and
// (b) strip the literal "<no value>" placeholder text/template emits when a
// `map[string]any` lookup hits a missing key (the documented quirk).
//
// The "<no value>" strip is suppressed when the source `contents` already
// contains that literal — text/template passes literals through verbatim, so
// stripping unconditionally would corrupt a template that legitimately
// contained the substring. This matters only for the contrived case of a
// path template like "<no value>foo.txt"; every real-world caller in this
// package renders path-shaped fragments where the literal does not appear.
//
// templateFolder is the absolute path of the template folder when known
// (empty in FS mode); it's only used by helpers that introspect the
// template path (which filenames almost never invoke).
func renderForAnalysis(ctx context.Context, templateFolder, contents string, vars map[string]any) (string, error) {
	if !strings.Contains(contents, "{{") {
		return contents, nil
	}

	opts := &options.BoilerplateOptions{
		TemplateFolder:  templateFolder,
		OutputFolder:    analysisOutputSentinel,
		NonInteractive:  true,
		NoHooks:         true,
		NoShell:         true,
		OnMissingKey:    options.ZeroValue,
		OnMissingConfig: options.Ignore,
	}

	rendered, err := render.RenderTemplateFromStringWithContext(ctx, logging.Discard(), "filename", contents, vars, opts)
	if err != nil {
		return rendered, err
	}

	return scrubAnalysisRender(contents, rendered), nil
}

// scrubAnalysisRender removes analysis-only artifacts from a rendered
// template fragment: the absolute form of the outputFolder sentinel
// (collapsed to ".") and the literal "<no value>" placeholder. The
// missing-key strip is suppressed when source already contained the
// literal — see renderForAnalysis for why.
func scrubAnalysisRender(source, rendered string) string {
	if sentinelAbs := analysisOutputSentinelAbs(); sentinelAbs != "" {
		rendered = strings.ReplaceAll(rendered, sentinelAbs, ".")
	}

	if !strings.Contains(source, missingValueLiteral) {
		rendered = strings.ReplaceAll(rendered, missingValueLiteral, "")
	}

	return rendered
}

// computeDepPathRefs returns the set of parent-scope variable names
// referenced in this dependency's template-url and output-folder fields.
// These vars don't bind to a specific child variable; they shape *which*
// subtree gets pulled in and *where* it lands. A change to any of them
// affects every file under the dependency.
//
// Parse errors are swallowed: the runtime would surface them at render
// time via the existing filename_render / unresolvable_dependency soft
// errors, so duplicating that surface here adds noise without information.
func computeDepPathRefs(dep *variables.Dependency) map[string]struct{} {
	out := map[string]struct{}{}

	for _, expr := range []string{dep.TemplateURL, dep.OutputFolder} {
		if !strings.Contains(expr, "{{") {
			continue
		}

		refs, err := extractRefs("dep-path", expr)
		if err != nil {
			continue
		}

		for v := range refs.vars {
			out[v] = struct{}{}
		}
	}

	return out
}

// computeSkipFilesRefs returns the set of variable names referenced in any
// of the supplied skip_files entries' path / not_path / if expressions. A
// change to any of these vars may include or exclude files in this
// template's output, so the caller treats them as affecting every file in
// the template.
//
// As with computeDepPathRefs, parse errors are swallowed: the runtime
// surfaces them via the existing skip_files soft error path, and the
// per-template ref set should not duplicate that surface.
func computeSkipFilesRefs(skipFiles []variables.SkipFile) map[string]struct{} {
	out := map[string]struct{}{}

	for _, sf := range skipFiles {
		for _, expr := range []string{sf.Path, sf.NotPath, sf.If} {
			if !strings.Contains(expr, "{{") {
				continue
			}

			refs, err := extractRefs("skip-files", expr)
			if err != nil {
				continue
			}

			for v := range refs.vars {
				out[v] = struct{}{}
			}
		}
	}

	return out
}

// computeDepEdges parses every value expression in a dependency's variables
// block (default + reference fields) and returns a map of
// childVarName -> set of parent var names referenced.
func computeDepEdges(dep *variables.Dependency, parentDeclared map[string]variables.Variable, parentDeclaredIn string, result *Result) map[string]map[string]struct{} {
	edges := map[string]map[string]struct{}{}

	for _, childVar := range dep.Variables {
		refs := map[string]struct{}{}

		// Reference field — equivalent to {{ .Name }} in scope.
		if r := childVar.Reference(); r != "" {
			refs[r] = struct{}{}
		}

		// Default value may be a string, list, or map containing template
		// expressions in any leaf string.
		collectStringRefs(childVar.Default(), refs, result, parentDeclaredIn, childVar.Name())

		// Filter to references that resolve to a parent declaration. Refs to
		// names that aren't declared anywhere are surfaced as
		// undeclared_variable in collectStringRefs's caller path; here we keep
		// only those that resolve to the parent.
		filtered := map[string]struct{}{}

		for v := range refs {
			if _, ok := parentDeclared[v]; ok {
				filtered[v] = struct{}{}
			}
		}

		if len(filtered) > 0 {
			edges[childVar.Name()] = filtered
		}
	}

	return edges
}

// collectStringRefs walks any value (string, []any, map[string]any, ...) and
// for every string leaf containing template syntax, parses it and records
// referenced variable names.
func collectStringRefs(v any, refs map[string]struct{}, result *Result, declaredIn, name string) {
	switch x := v.(type) {
	case string:
		if !strings.Contains(x, "{{") {
			return
		}

		r, err := extractRefs("dep-default", x)
		if err != nil {
			result.Errors = append(result.Errors, AnalysisError{
				Kind:     KindParse,
				Template: declaredIn,
				Name:     name,
				Message:  err.Error(),
			})

			return
		}

		for v := range r.vars {
			refs[v] = struct{}{}
		}
	case []any:
		for _, item := range x {
			collectStringRefs(item, refs, result, declaredIn, name)
		}
	case map[string]any:
		for _, item := range x {
			collectStringRefs(item, refs, result, declaredIn, name)
		}
	}
}

// composeResult walks the templateInfo tree and emits one InputEntry per
// (template_path, declared_input) pair into result, with transitive closure
// applied through dependency edges and partials already expanded in fileRefs.
func composeResult(root *templateInfo, result *Result) {
	all := flatten(root)

	emittedUndeclared := map[string]struct{}{}

	// Build the inverse index as a set-of-sets so duplicate (file, key) pairs
	// collapse in O(1); finalize materializes it into the slice form Result
	// exposes. Without this, repeatedly appending to result.Files via a
	// linear-search dedup is O(n²) per file in the count of contributing
	// inputs, and propagation through dependency edges makes that count
	// non-trivial.
	inverse := map[string]map[string]struct{}{}

	for _, t := range all {
		for declaredName, decl := range t.declared {
			key := inputKey(t.declaredIn, declaredName)
			files := filesAffectedBy(t, declaredName, map[string]struct{}{})

			result.Inputs[key] = InputEntry{
				Name:        declaredName,
				DeclaredIn:  t.declaredIn,
				Files:       sortedKeys(files),
				Type:        string(decl.Type()),
				Description: decl.Description(),
			}

			for f := range files {
				bucket, ok := inverse[f]
				if !ok {
					bucket = map[string]struct{}{}
					inverse[f] = bucket
				}

				bucket[key] = struct{}{}
			}
		}

		// Surface any var referenced in this template's bodies that doesn't
		// resolve to a declaration anywhere in scope (this template + name-
		// match ancestors).
		for filePath, refs := range t.fileRefs {
			for v := range refs {
				if _, declared := t.declared[v]; declared {
					continue
				}

				// The var may also be declared by an ancestor and reach this
				// template via name-match inheritance — that's checked by
				// hasNameMatchAncestor.
				if hasNameMatchAncestor(t, v, root) {
					continue
				}

				ek := t.declaredIn + "/" + filePath + "/" + v
				if _, dup := emittedUndeclared[ek]; dup {
					continue
				}

				emittedUndeclared[ek] = struct{}{}

				result.Errors = append(result.Errors, AnalysisError{
					Kind:     KindUndeclaredVariable,
					Template: t.declaredIn,
					Name:     v,
					File:     filePath,
				})
			}
		}
	}

	for f, bucket := range inverse {
		result.Files[f] = sortedKeys(bucket)
	}

	// Project every template's per-file source mapping into the top-level
	// Sources map. Each templateInfo only records files that rendered
	// cleanly, so dynamic-path outputs (KindFilenameRender) are naturally
	// absent here. Children may produce paths the root never sees and vice
	// versa, so we walk the whole tree.
	for _, t := range all {
		for out, src := range t.fileSources {
			result.Sources[out] = src
		}
	}
}

// flatten returns all templateInfo nodes in tree order.
func flatten(root *templateInfo) []*templateInfo {
	if root == nil {
		return nil
	}

	out := []*templateInfo{root}

	for _, dep := range root.deps {
		out = append(out, flatten(dep.child)...)
	}

	return out
}

// filesAffectedBy returns the set of files (relative to root output) that
// would need to be re-rendered if (t.declaredIn, varName)'s value changed.
//
// The closure includes:
//   - All files in t whose body references varName directly.
//   - For every dependency edge (parent var V -> child var C) where V==varName,
//     the closure of (child, C).
//   - For every same-name child variable that inherits via name match (child
//     declares V and dont-inherit-variables=false and parent passes no
//     explicit override for V), the closure of (child, varName).
//
// visiting tracks (declaredIn, varName) currently in the recursion to break
// cycles.
func filesAffectedBy(t *templateInfo, varName string, visiting map[string]struct{}) map[string]struct{} {
	key := t.declaredIn + ":" + varName
	if _, in := visiting[key]; in {
		return map[string]struct{}{}
	}

	visiting[key] = struct{}{}
	defer delete(visiting, key)

	out := map[string]struct{}{}

	// Direct: files in t that reference varName.
	for filePath, refs := range t.fileRefs {
		if _, ok := refs[varName]; ok {
			out[filePath] = struct{}{}
		}
	}

	// Skip-files refs: a change to a var that appears in any of this
	// template's skip_files entries (path / not_path / if) may include or
	// exclude any of this template's output files. Conservative signal:
	// link the var to every file currently produced by this template.
	if _, hits := t.skipFilesRefs[varName]; hits {
		for filePath := range t.fileRefs {
			out[filePath] = struct{}{}
		}
	}

	// Through explicit value-expression edges to children.
	for _, dep := range t.deps {
		// Explicit edges: parent var -> child var(s) referencing it.
		for childVar, parentVars := range dep.edges {
			if _, hits := parentVars[varName]; !hits {
				continue
			}

			for f := range filesAffectedBy(dep.child, childVar, visiting) {
				out[f] = struct{}{}
			}
		}

		// Implicit name-match inheritance: when the parent declares varName
		// and the child also declares varName, and the dependency does NOT
		// already provide an explicit override for varName, AND
		// dont-inherit-variables is false, the parent's value flows into the
		// child by name. Treat it as an edge.
		if !dep.cfg.DontInheritVariables {
			if _, declaredInChild := dep.child.declared[varName]; declaredInChild {
				if _, explicit := dep.edges[varName]; !explicit {
					for f := range filesAffectedBy(dep.child, varName, visiting) {
						out[f] = struct{}{}
					}
				}
			}
		}

		// Path-to-subtree edge: a parent var referenced in dep.template-url
		// or dep.output-folder shapes which subtree gets pulled in and where
		// it lands. A change to it relocates or replaces every file under
		// the dep, not just files referencing a specific child var.
		if _, hits := dep.pathRefs[varName]; hits {
			for f := range allFilesInSubtree(dep.child) {
				out[f] = struct{}{}
			}
		}
	}

	return out
}

// allFilesInSubtree returns every output path produced by t and its
// transitive dependencies. Used by path-to-subtree edges, where a change
// to a path-shaping parent var moves or replaces the whole subtree rather
// than affecting one specific child variable.
func allFilesInSubtree(t *templateInfo) map[string]struct{} {
	out := map[string]struct{}{}
	if t == nil {
		return out
	}

	for f := range t.fileRefs {
		out[f] = struct{}{}
	}

	for _, dep := range t.deps {
		for f := range allFilesInSubtree(dep.child) {
			out[f] = struct{}{}
		}
	}

	return out
}

// hasNameMatchAncestor returns true if any ancestor of t (along the chain from
// root to t, where each step does not have dont-inherit-variables=true)
// declares varName.
func hasNameMatchAncestor(t *templateInfo, varName string, root *templateInfo) bool {
	chain := findChainTo(root, t, nil)
	if chain == nil {
		return false
	}

	for i := 0; i < len(chain)-1; i++ {
		ancestor := chain[i]
		nextChild := chain[i+1]

		var depCfg *variables.Dependency

		for _, dep := range ancestor.deps {
			if dep.child == nextChild {
				depCfg = dep.cfg
				break
			}
		}

		if depCfg != nil && depCfg.DontInheritVariables {
			return false
		}

		if _, ok := ancestor.declared[varName]; ok {
			return true
		}
	}

	return false
}

// findChainTo returns the path from root to target, inclusive, or nil if
// target is not reachable from root.
func findChainTo(root, target *templateInfo, acc []*templateInfo) []*templateInfo {
	acc = append(acc, root)
	if root == target {
		return acc
	}

	for _, dep := range root.deps {
		if got := findChainTo(dep.child, target, acc); got != nil {
			return got
		}
	}

	return nil
}

// isLikelyText returns true if the data looks like a text file. Mirrors
// boilerplate's runtime IsTextFile heuristic without requiring on-disk
// access. Empty files are treated as binary (matching fileutil.IsTextFile).
func isLikelyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Fast path: most binary files contain at least one NUL byte in the head.
	const headSize = 8192

	head := data
	if len(head) > headSize {
		head = head[:headSize]
	}

	for _, b := range head {
		if b == 0 {
			return false
		}
	}

	return utf8.Valid(head)
}

func sortedKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return []string{}
	}

	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

// slashRel returns target relative to base, treating both as slash-separated
// paths from an fs.FS. It exists because filepath.Rel uses the OS separator
// and corrupts paths on Windows when the inputs come from fs.WalkDir.
//
// If target does not live under base, slashRel returns target unchanged.
func slashRel(base, target string) string {
	if base == "" || base == "." {
		return target
	}

	cleanBase := path.Clean(base)
	cleanTarget := path.Clean(target)

	if cleanTarget == cleanBase {
		return "."
	}

	prefix := cleanBase + "/"
	if !strings.HasPrefix(cleanTarget, prefix) {
		return target
	}

	return strings.TrimPrefix(cleanTarget, prefix)
}

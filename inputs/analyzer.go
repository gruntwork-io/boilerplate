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
	"unicode/utf8"

	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/variables"
)

// templateInfo holds analysis output for a single template (one boilerplate.yml).
type templateInfo struct {
	// loc identifies where this template lives.
	loc templateLocation

	// outputPath is the path where this template's output goes, relative to
	// the root template's output root. "." for the root.
	outputPath string

	// declaredIn is the path used in input keys (`<declaredIn>:<name>`). It
	// matches outputPath but with "." preserved as "." (rather than "" or
	// some normalized form).
	declaredIn string

	// config is the parsed boilerplate.yml.
	cfg *config.BoilerplateConfig

	// declared maps variable name -> declaration.
	declared map[string]variables.Variable

	// fileRefs[outputFilePath] is the set of variable names referenced when
	// rendering that file. Output paths are relative to the root output root.
	fileRefs map[string]map[string]struct{}

	// dependencies are children that this template depends on.
	dependencies []*templateInfo

	// depEdges records explicit value-expression edges: for each dependency
	// in this template, for each child variable name, the parent-scope vars
	// referenced in the value expression. Indexed as
	// depEdges[depIndex][childVarName] = set of parent var names.
	depEdges []map[string]map[string]struct{}
}

// analyze is the top-level entry point used by both FromOptions and FromFS.
func analyze(ctx context.Context, root templateLocation, vars map[string]any, resolver dependencyResolver) (*Result, error) {
	result := &Result{
		Inputs: map[string]InputEntry{},
		Files:  map[string][]string{},
		Errors: []AnalysisError{},
	}

	cleanups := []func(){}
	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()

	// Walk the dependency tree and build the per-template analysis.
	visiting := map[string]struct{}{}

	rootInfo, err := analyzeTree(ctx, root, ".", ".", vars, resolver, result, visiting, &cleanups)
	if err != nil {
		return nil, err
	}

	// Compose the final input -> files map across the tree, including
	// transitive propagation through dependency edges.
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
		loc:        loc,
		outputPath: outputPath,
		declaredIn: declaredIn,
		cfg:        cfg,
		declared:   make(map[string]variables.Variable, len(cfg.Variables)),
		fileRefs:   map[string]map[string]struct{}{},
	}

	for _, v := range cfg.Variables {
		info.declared[v.Name()] = v
	}

	// Parse partials first so file analysis can resolve {{ template "name" }}
	// invocations transitively.
	partialRefs, partialErrs := analyzePartials(loc, cfg.Partials, vars)
	result.Errors = append(result.Errors, partialErrs...)

	// Walk the template files and collect refs per file.
	if walkErr := analyzeFiles(loc, info, partialRefs, vars, result); walkErr != nil {
		return nil, walkErr
	}

	// Recurse into dependencies.
	for i := range cfg.Dependencies {
		dep := &cfg.Dependencies[i]

		// Render template-url and output-folder from parent vars.
		renderedURL, renderErr := renderForAnalysis(loc.absDir, dep.TemplateURL, vars)
		if renderErr != nil {
			result.Errors = append(result.Errors, AnalysisError{
				Kind:     "filename_render",
				Template: declaredIn,
				Name:     dep.Name,
				Message:  fmt.Sprintf("could not render template-url for dependency %q: %v", dep.Name, renderErr),
			})

			renderedURL = dep.TemplateURL
		}

		renderedOutputFolder, renderErr := renderForAnalysis(loc.absDir, dep.OutputFolder, vars)
		if renderErr != nil {
			result.Errors = append(result.Errors, AnalysisError{
				Kind:     "filename_render",
				Template: declaredIn,
				Name:     dep.Name,
				Message:  fmt.Sprintf("could not render output-folder for dependency %q: %v", dep.Name, renderErr),
			})

			renderedOutputFolder = dep.OutputFolder
		}

		// Cycle detection.
		cycleKey := renderedURL
		if _, busy := visiting[cycleKey]; busy {
			result.Errors = append(result.Errors, AnalysisError{
				Kind:     "cycle",
				Template: declaredIn,
				Name:     dep.Name,
				Message:  fmt.Sprintf("dependency %q forms a cycle (template-url=%s)", dep.Name, renderedURL),
			})
			info.depEdges = append(info.depEdges, nil)

			continue
		}

		visiting[cycleKey] = struct{}{}

		childLoc, depCleanup, resolveErr := resolver.Resolve(ctx, loc, renderedURL)
		if depCleanup != nil {
			*cleanups = append(*cleanups, depCleanup)
		}

		if resolveErr != nil {
			delete(visiting, cycleKey)

			kind := "unresolvable_dependency"
			result.Errors = append(result.Errors, AnalysisError{
				Kind:     kind,
				Template: declaredIn,
				Name:     dep.Name,
				Message:  resolveErr.Error(),
			})
			info.depEdges = append(info.depEdges, nil)

			continue
		}

		childOutputPath := joinOutputPath(outputPath, renderedOutputFolder)
		childDeclaredIn := joinOutputPath(declaredIn, renderedOutputFolder)

		// Compute explicit value-expression edges from parent vars to this
		// child's input variables, BEFORE recursing — those edges are
		// independent of the child's body refs.
		edges := computeDepEdges(dep, info.declared, declaredIn, result)
		info.depEdges = append(info.depEdges, edges)

		childInfo, depErr := analyzeTree(ctx, childLoc, childOutputPath, childDeclaredIn, vars, resolver, result, visiting, cleanups)
		delete(visiting, cycleKey)

		if depErr != nil {
			// Treat dependency analysis failure as a soft error so the rest
			// of the result is still useful.
			result.Errors = append(result.Errors, AnalysisError{
				Kind:     "parse",
				Template: childDeclaredIn,
				Name:     dep.Name,
				Message:  depErr.Error(),
			})

			continue
		}

		info.dependencies = append(info.dependencies, childInfo)
	}

	return info, nil
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
func analyzePartials(loc templateLocation, partialGlobs []string, vars map[string]any) (map[string]*templateRefs, []AnalysisError) {
	out := map[string]*templateRefs{}

	var errs []AnalysisError

	for _, glob := range partialGlobs {
		// Render the glob with vars (best effort) to handle any embedded
		// template syntax.
		rendered, err := renderForAnalysis(loc.absDir, glob, vars)
		if err != nil {
			rendered = glob
		}

		files, listErr := globPartialFiles(loc, rendered)
		if listErr != nil {
			errs = append(errs, AnalysisError{
				Kind:    "parse",
				Message: fmt.Sprintf("could not resolve partial glob %q: %v", glob, listErr),
			})

			continue
		}

		for _, pf := range files {
			trees, parseErr := parseTemplateAll(pf.name, pf.contents)
			if parseErr != nil {
				errs = append(errs, AnalysisError{
					Kind:    "parse",
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
	expandPartialRefs(out)

	return out, errs
}

// expandPartialRefs computes the transitive closure of partial -> partial
// invocations, so each entry in m has all variables it could reference if
// expanded.
func expandPartialRefs(m map[string]*templateRefs) {
	const maxIterations = 100

	for i := 0; i < maxIterations; i++ {
		changed := false

		for name, refs := range m {
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

			m[name] = refs
		}

		if !changed {
			break
		}
	}
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

	// FS mode.
	joined := path.Join(loc.dir, pattern)
	if strings.HasPrefix(joined, "..") || strings.Contains(joined, "/../") {
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
func analyzeFiles(loc templateLocation, info *templateInfo, partialRefs map[string]*templateRefs, vars map[string]any, result *Result) error {
	walkRoot := loc.dir
	if walkRoot == "" {
		walkRoot = "."
	}

	cfgPath := path.Join(loc.dir, config.BoilerplateConfigFile)
	skipDirs := dependencySkipDirs(loc, info.cfg)

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
		// mode), not to this template's body.
		if filepath.Base(p) == config.BoilerplateConfigFile {
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
			// Binary file; boilerplate copies it as-is at render time. It is
			// not affected by any input variable, but we still record an
			// empty entry so the file appears in the output. Use the rendered
			// output path.
			outPath := computeOutputPath(loc, info, p, vars, result)
			if outPath != "" {
				if _, exists := info.fileRefs[outPath]; !exists {
					info.fileRefs[outPath] = map[string]struct{}{}
				}
			}

			return nil
		}

		refs, parseErr := extractRefs(p, string(data))
		if parseErr != nil {
			result.Errors = append(result.Errors, AnalysisError{
				Kind:    "parse",
				File:    p,
				Message: parseErr.Error(),
			})

			return nil
		}

		// Expand template invocations through partials.
		for inv := range refs.invocations {
			if other, ok := partialRefs[inv]; ok {
				for v := range other.vars {
					refs.vars[v] = struct{}{}
				}
			}
		}

		outPath := computeOutputPath(loc, info, p, vars, result)
		if outPath == "" {
			return nil
		}

		if _, exists := info.fileRefs[outPath]; !exists {
			info.fileRefs[outPath] = map[string]struct{}{}
		}

		for v := range refs.vars {
			info.fileRefs[outPath][v] = struct{}{}
		}

		return nil
	})
}

// dependencySkipDirs returns the set of directory paths inside loc.fsys that
// hold a local dependency template. The walk in analyzeFiles uses this set to
// avoid descending into nested templates — those are analyzed separately by
// analyzeTree as their own units.
//
// Only local relative dependencies (template-url that is a relative path,
// not a URL scheme) contribute to the skip set; remote dependencies live
// outside the FS and never overlap with the walk anyway.
func dependencySkipDirs(loc templateLocation, cfg *config.BoilerplateConfig) map[string]struct{} {
	out := map[string]struct{}{}
	if cfg == nil {
		return out
	}

	for _, dep := range cfg.Dependencies {
		url := dep.TemplateURL
		if url == "" || strings.Contains(url, "://") {
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

// computeOutputPath maps an input file path inside loc.fsys to its output path
// relative to the root output root. The filename portion may itself contain
// template syntax; we render it best-effort.
func computeOutputPath(loc templateLocation, info *templateInfo, inputPath string, vars map[string]any, result *Result) string {
	// Path of the file relative to the template directory.
	rel := inputPath
	if loc.dir != "" && loc.dir != "." {
		var trimErr error

		rel, trimErr = filepath.Rel(loc.dir, inputPath)
		if trimErr != nil {
			rel = inputPath
		}
	}

	rel = filepath.ToSlash(rel)

	// Match the runtime: `|` is an illegal filename character on Windows, so
	// boilerplate fixtures URL-encode pipes as %7C / %7c. Decode before
	// rendering so the template syntax round-trips.
	if decoded, decodeErr := url.QueryUnescape(rel); decodeErr == nil {
		rel = decoded
	}

	// Render any template syntax in the filename. Pass the file's basename as
	// the template name for diagnostic messages.
	rendered, err := renderForAnalysis(loc.absDir, rel, vars)
	if err != nil {
		result.Errors = append(result.Errors, AnalysisError{
			Kind:    "filename_render",
			File:    inputPath,
			Message: err.Error(),
		})

		rendered = rel
	}

	// Join with the template's output path within the root output tree.
	return joinOutputPath(info.outputPath, rendered)
}

// joinOutputPath joins a parent output path with a child relative path,
// producing a slash-separated path relative to the root output root. Returns
// "." for the root only when both inputs are root-equivalent.
func joinOutputPath(parent, child string) string {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)

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

// renderForAnalysis renders a small template fragment (filename, glob, or
// dep field) using the existing render pipeline with shell and hooks
// disabled and missing-key behavior set to zero-value. This keeps side
// effects out of the analysis path while still matching what
// `boilerplate template` would compute for the same inputs.
//
// templateFolder is the absolute path of the template folder when known
// (empty in FS mode); it's only used by helpers that introspect the
// template path (which filenames almost never invoke).
func renderForAnalysis(templateFolder, contents string, vars map[string]any) (string, error) {
	if !strings.Contains(contents, "{{") {
		return contents, nil
	}

	opts := &options.BoilerplateOptions{
		TemplateFolder:  templateFolder,
		NonInteractive:  true,
		NoHooks:         true,
		NoShell:         true,
		OnMissingKey:    options.ZeroValue,
		OnMissingConfig: options.Ignore,
	}

	return render.RenderTemplateFromString(logging.Discard(), "filename", contents, vars, opts)
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
				Kind:     "parse",
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
	case map[any]any:
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

	// Build an index from (declaredIn, varName) to the templateInfo of that
	// declaration's owning template. Used to resolve transitive edges.
	indexByOwner := map[string]*templateInfo{}
	for _, t := range all {
		indexByOwner[t.declaredIn] = t
	}

	// Build undeclared-variable errors: any var referenced in any file but
	// not declared in that template's chain (this template + all ancestors
	// via inherited edges).
	emittedUndeclared := map[string]struct{}{}

	// Emit one InputEntry per declared input, with files computed via
	// transitive closure.
	for _, t := range all {
		// Build the set of inputs that are reachable from this template's
		// inputs through outgoing edges (used to compute Files for parent
		// inputs that propagate to children).
		for declaredName, decl := range t.declared {
			key := inputKey(t.declaredIn, declaredName)
			files := filesAffectedBy(t, declaredName, all, map[string]struct{}{})

			result.Inputs[key] = InputEntry{
				Name:        declaredName,
				DeclaredIn:  t.declaredIn,
				Files:       sortedKeys(files),
				Type:        string(decl.Type()),
				Description: decl.Description(),
			}

			for f := range files {
				result.Files[f] = appendUnique(result.Files[f], key)
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
					Kind:     "undeclared_variable",
					Template: t.declaredIn,
					Name:     v,
					File:     filePath,
				})
			}
		}
	}
}

// flatten returns all templateInfo nodes in tree order.
func flatten(root *templateInfo) []*templateInfo {
	if root == nil {
		return nil
	}

	out := []*templateInfo{root}

	for _, c := range root.dependencies {
		out = append(out, flatten(c)...)
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
func filesAffectedBy(t *templateInfo, varName string, all []*templateInfo, visiting map[string]struct{}) map[string]struct{} {
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

	// Through explicit value-expression edges to children.
	for i, child := range t.dependencies {
		var depConfig *variables.Dependency
		if i < len(t.cfg.Dependencies) {
			depConfig = &t.cfg.Dependencies[i]
		}

		var edges map[string]map[string]struct{}
		if i < len(t.depEdges) {
			edges = t.depEdges[i]
		}

		// Explicit edges: parent var -> child var(s) referencing it.
		for childVar, parentVars := range edges {
			if _, hits := parentVars[varName]; !hits {
				continue
			}

			for f := range filesAffectedBy(child, childVar, all, visiting) {
				out[f] = struct{}{}
			}
		}

		// Implicit name-match inheritance: when the parent declares varName
		// and the child also declares varName, and the dependency does NOT
		// already provide an explicit override for varName, AND
		// dont-inherit-variables is false, the parent's value flows into the
		// child by name. Treat it as an edge.
		if depConfig != nil && !depConfig.DontInheritVariables {
			if _, declaredInChild := child.declared[varName]; declaredInChild {
				if _, explicit := edges[varName]; !explicit {
					for f := range filesAffectedBy(child, varName, all, visiting) {
						out[f] = struct{}{}
					}
				}
			}
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
		// Find which dep config corresponds to the next step.
		nextChild := chain[i+1]

		var depCfg *variables.Dependency

		for j, candidate := range ancestor.dependencies {
			if candidate == nextChild && j < len(ancestor.cfg.Dependencies) {
				depCfg = &ancestor.cfg.Dependencies[j]
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

	for _, c := range root.dependencies {
		if got := findChainTo(c, target, acc); got != nil {
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

func appendUnique(list []string, item string) []string {
	for _, existing := range list {
		if existing == item {
			return list
		}
	}

	return append(list, item)
}

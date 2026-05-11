package inputs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	zglob "github.com/mattn/go-zglob"

	"github.com/gruntwork-io/boilerplate/variables"
)

// processedSkipRule is one fully-resolved skip_files entry: the rendered
// `if` condition and the concrete files that the entry's path / not_path
// glob expanded to. Paths are slash-separated relative to the template root.
type processedSkipRule struct {
	paths    map[string]struct{}
	notPaths map[string]struct{}
	skipIf   bool
}

// skipFileFilter answers "should this walked path be skipped?" for one
// template's worth of skip_files. Construct it once via processSkipFiles,
// then call shouldSkip for each file the walker visits.
//
// Mirrors templates.shouldSkipPath at the runtime: a path is skipped if it
// matches any rule's path set, OR if any rule has effective not_path entries
// and the path does not appear in any of them.
type skipFileFilter struct {
	rules []processedSkipRule
}

// processSkipFiles renders the boilerplate config's skip_files into a
// skipFileFilter. Path/glob expansion uses zglob on disk (matching the
// runtime, which supports `**`); in FS mode it falls back to fs.Glob, which
// supports only the `*` and `?` wildcards.
func processSkipFiles(ctx context.Context, loc templateLocation, skipFiles []variables.SkipFile, vars map[string]any) (*skipFileFilter, []AnalysisError) {
	filter := &skipFileFilter{}

	if len(skipFiles) == 0 {
		return filter, nil
	}

	var errs []AnalysisError

	for _, sf := range skipFiles {
		rule := processedSkipRule{
			skipIf:   true,
			paths:    map[string]struct{}{},
			notPaths: map[string]struct{}{},
		}

		// Render the `if` condition. An empty condition defaults to true,
		// matching the runtime.
		if sf.If != "" {
			rendered, err := renderForAnalysis(ctx, loc.absDir, sf.If, vars)
			if err != nil {
				errs = append(errs, AnalysisError{
					Kind:    "skip_files",
					Message: fmt.Sprintf("could not render skip_files if condition %q: %v", sf.If, err),
				})
				// Be conservative: if we can't render the condition, assume
				// the rule is inactive so the analyzer reports a superset of
				// the actual outputs rather than silently dropping files.
				rule.skipIf = false
			} else {
				rule.skipIf = rendered == "true"
			}
		}

		if sf.Path != "" {
			matches, matchErrs := expandSkipGlob(ctx, loc, sf.Path, vars, "path")
			errs = append(errs, matchErrs...)

			for _, m := range matches {
				rule.paths[m] = struct{}{}
			}
		}

		if sf.NotPath != "" {
			matches, matchErrs := expandSkipGlob(ctx, loc, sf.NotPath, vars, "not_path")
			errs = append(errs, matchErrs...)

			for _, m := range matches {
				rule.notPaths[m] = struct{}{}
			}
		}

		filter.rules = append(filter.rules, rule)
	}

	return filter, errs
}

// shouldSkip returns true if walkedPath (slash-separated, relative to the
// template root) should be excluded from analysis output.
//
// Composition mirrors templates.shouldSkipPath:
//   - `path` rules combine with OR: any matching path-rule with a true `if`
//     skips the file.
//   - `not_path` rules combine such that if any rule has effective
//     `not_path` entries, the file must appear in some not_path list to be
//     kept; otherwise it is skipped.
func (f *skipFileFilter) shouldSkip(walkedPath string) bool {
	if f == nil || len(f.rules) == 0 {
		return false
	}

	canonical := path.Clean(walkedPath)

	for _, r := range f.rules {
		if !r.skipIf {
			continue
		}

		if _, ok := r.paths[canonical]; ok {
			return true
		}
	}

	hasEffectiveNotPath := false

	for _, r := range f.rules {
		if r.skipIf && len(r.notPaths) > 0 {
			hasEffectiveNotPath = true
			break
		}
	}

	if !hasEffectiveNotPath {
		return false
	}

	for _, r := range f.rules {
		if !r.skipIf {
			continue
		}

		if _, ok := r.notPaths[canonical]; ok {
			return false
		}

		// not_path also keeps any directory whose contents are kept by an
		// included path. Mirror runtime: if a not_path entry lives under
		// canonical+"/", canonical is a parent dir and should be kept.
		for kept := range r.notPaths {
			if strings.HasPrefix(kept, canonical+"/") {
				return false
			}
		}
	}

	return true
}

// expandSkipGlob renders pattern with vars and resolves it to a set of
// slash-separated paths relative to the template root. Returns soft errors
// (kind="skip_files") for any failure that prevents resolution.
//
// On disk it uses zglob to match the runtime (which supports `**`); in FS
// mode it uses fs.Glob, which only supports `*` and `?`. Patterns containing
// `**` in FS mode produce a soft error.
func expandSkipGlob(ctx context.Context, loc templateLocation, pattern string, vars map[string]any, attr string) ([]string, []AnalysisError) {
	rendered, err := renderForAnalysis(ctx, loc.absDir, pattern, vars)
	if err != nil {
		return nil, []AnalysisError{{
			Kind:    "skip_files",
			Message: fmt.Sprintf("could not render skip_files %s %q: %v", attr, pattern, err),
		}}
	}

	if loc.absDir != "" {
		joined := rendered
		if !filepath.IsAbs(joined) {
			joined = filepath.Join(loc.absDir, rendered)
		}

		matches, globErr := zglob.Glob(joined)
		if globErr != nil {
			// zglob returns os.ErrNotExist when nothing matched (and other
			// errors on real failures). An unmatched glob isn't a problem for
			// the analyzer; only surface other errors.
			if errors.Is(globErr, os.ErrNotExist) {
				return nil, nil
			}

			return nil, []AnalysisError{{
				Kind:    "skip_files",
				Message: fmt.Sprintf("could not glob skip_files %s %q: %v", attr, pattern, globErr),
			}}
		}

		out := make([]string, 0, len(matches))

		for _, m := range matches {
			rel, relErr := filepath.Rel(loc.absDir, m)
			if relErr != nil {
				continue
			}

			out = append(out, filepath.ToSlash(rel))
		}

		return out, nil
	}

	// FS mode. fs.Glob does not support `**`; surface that as a soft error
	// rather than silently producing an empty match set.
	if strings.Contains(rendered, "**") {
		return nil, []AnalysisError{{
			Kind:    "skip_files",
			Message: fmt.Sprintf("skip_files %s %q uses `**` glob; only `*`/`?` are supported in WASM/FS mode", attr, pattern),
		}}
	}

	joined := path.Join(loc.dir, rendered)

	matches, globErr := fs.Glob(loc.fsys, joined)
	if globErr != nil {
		return nil, []AnalysisError{{
			Kind:    "skip_files",
			Message: fmt.Sprintf("could not glob skip_files %s %q: %v", attr, pattern, globErr),
		}}
	}

	out := make([]string, 0, len(matches))

	for _, m := range matches {
		out = append(out, slashRel(loc.dir, m))
	}

	return out, nil
}

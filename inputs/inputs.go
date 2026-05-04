// Package inputs provides static analysis of boilerplate templates to compute
// which output files are affected by each declared input variable.
//
// Unlike the render path in package render/templates, this package never
// executes a template body. It parses each template into an AST and walks the
// AST to collect variable references. The result is a JSON-serializable map
// suitable for live-diff UIs that need to know which files to re-render when
// a single input changes.
package inputs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/gruntwork-io/boilerplate/getterhelper"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
)

// Result is the top-level analysis output. It is JSON-serializable and matches
// the shape documented for the `boilerplate inputs map` CLI subcommand.
type Result struct {
	// Inputs is keyed by "<template_path>:<input_name>". Each entry describes
	// one declared input variable across the entire dependency tree.
	Inputs map[string]InputEntry `json:"inputs"`

	// Files is the inverse index: keyed by output path (relative to the root
	// template's output root), each entry lists the input keys whose change
	// would re-render that file.
	Files map[string][]string `json:"files"`

	// Errors collects soft errors encountered during analysis. A non-empty
	// list does not imply the run failed; callers should still consume Inputs
	// and Files. Hard errors (parse failures of the boilerplate config tree
	// itself) are returned via the error return of FromOptions / FromFS.
	Errors []AnalysisError `json:"errors"`
}

// InputEntry describes a single declared input.
type InputEntry struct {
	Name        string   `json:"name"`
	DeclaredIn  string   `json:"declared_in"`
	Files       []string `json:"files"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
}

// AnalysisError is a soft error encountered during analysis. Soft errors do
// not abort the run; they accumulate in Result.Errors so the caller can
// surface them to the user.
//
// Defined Kind values:
//
//	"undeclared_variable" — referenced in a template body but not declared in
//	    any boilerplate.yml in scope.
//	"cycle"               — a dependency cycle was detected.
//	"unresolvable_dependency" — a remote URL in FS-only mode, or a path that
//	    does not exist.
//	"filename_render"     — failed to render a template-bearing filename.
//	"parse"               — failed to parse a template body or value
//	    expression.
type AnalysisError struct {
	Kind     string `json:"kind"`
	Template string `json:"template,omitempty"`
	Name     string `json:"name,omitempty"`
	File     string `json:"file,omitempty"`
	Message  string `json:"message,omitempty"`
}

// inputKey builds the fully-qualified input identifier used as a map key.
func inputKey(templatePath, inputName string) string {
	if templatePath == "" {
		templatePath = "."
	}

	return templatePath + ":" + inputName
}

// FromOptions runs analysis using the same template-resolution rules as
// `boilerplate template`: it can resolve remote URLs through go-getter, accepts
// --var and --var-file values via opts, and reads from the local filesystem.
//
// Use this entry point from the CLI. For a side-effect-free pure analysis
// (as required by the WASM build), use FromFS instead.
func FromOptions(ctx context.Context, l logging.Logger, opts *options.BoilerplateOptions) (*Result, error) {
	rootLoc, cleanup, err := resolveRootLocation(l, opts)
	if cleanup != nil {
		defer cleanup()
	}

	if err != nil {
		return nil, err
	}

	resolver := &osResolver{logger: l}

	return analyze(ctx, rootLoc, opts.Vars, resolver)
}

// FromFS runs analysis with no I/O outside the supplied fs.FS. Used by the
// WASM bridge.
//
// rootFS must contain a `boilerplate.yml` at rootPath (use "." for the root of
// the FS). Local dependency template URLs are resolved as relative paths
// inside rootFS; remote URLs land in Result.Errors as
// "unresolvable_dependency".
func FromFS(ctx context.Context, rootFS fs.FS, rootPath string, vars map[string]any) (*Result, error) {
	if rootPath == "" {
		rootPath = "."
	}

	root := templateLocation{fsys: rootFS, dir: rootPath}
	resolver := &fsResolver{root: rootFS}

	return analyze(ctx, root, vars, resolver)
}

// templateLocation identifies a single template folder. It bundles an fs.FS
// with the path inside that fs.FS at which the boilerplate.yml lives, plus an
// optional absolute disk path for OS-mode resolution of relative dependency
// URLs (empty for in-memory FSes).
type templateLocation struct {
	fsys   fs.FS
	dir    string // path inside fsys; "." for the root of the FS
	absDir string // absolute disk path of dir, or "" if not on disk
}

// dependencyResolver locates a dependency template's filesystem and the path
// inside that filesystem at which the dependency's boilerplate.yml lives.
type dependencyResolver interface {
	Resolve(ctx context.Context, parent templateLocation, templateURL string) (child templateLocation, cleanup func(), err error)
}

// osResolver resolves dependencies via the local filesystem and go-getter for
// remote URLs.
type osResolver struct {
	logger logging.Logger
}

func (r *osResolver) Resolve(_ context.Context, parent templateLocation, templateURL string) (templateLocation, func(), error) {
	_, templateFolder, err := getterhelper.DetermineTemplateConfig(templateURL)
	if err != nil {
		return templateLocation{}, nil, err
	}

	if templateFolder != "" {
		// Local relative path. Resolve against the parent's directory using
		// the absolute disk path, then create a fresh os.DirFS rooted at the
		// child template directory.
		if parent.absDir == "" {
			return templateLocation{}, nil, fmt.Errorf("cannot resolve relative dependency %q: parent has no on-disk path", templateURL)
		}

		joined := filepath.Join(parent.absDir, templateURL)

		abs, absErr := filepath.Abs(joined)
		if absErr != nil {
			return templateLocation{}, nil, absErr
		}

		return templateLocation{fsys: os.DirFS(abs), dir: ".", absDir: abs}, nil, nil
	}

	// Remote URL: go-getter clone, then return an FS rooted at the cloned dir.
	workingDir, downloaded, err := getterhelper.DownloadTemplatesToTemporaryFolder(r.logger, templateURL)

	cleanup := func() {
		if workingDir != "" {
			if rmErr := os.RemoveAll(workingDir); rmErr != nil {
				r.logger.Errorf("failed to clean up dependency working directory %s: %v", workingDir, rmErr)
			}
		}
	}

	if err != nil {
		return templateLocation{}, cleanup, err
	}

	abs, absErr := filepath.Abs(downloaded)
	if absErr != nil {
		return templateLocation{}, cleanup, absErr
	}

	return templateLocation{fsys: os.DirFS(abs), dir: ".", absDir: abs}, cleanup, nil
}

// fsResolver resolves dependencies as relative paths inside a single fs.FS.
// Remote URLs are not supported.
type fsResolver struct {
	root fs.FS
}

// errUnresolvableRemote signals the caller that the dependency is a remote URL
// and cannot be analyzed in FS-only mode.
var errUnresolvableRemote = errors.New("remote template URLs are not supported in FS mode")

func (r *fsResolver) Resolve(_ context.Context, parent templateLocation, templateURL string) (templateLocation, func(), error) {
	_, templateFolder, err := getterhelper.DetermineTemplateConfig(templateURL)
	if err != nil {
		return templateLocation{}, nil, err
	}

	if templateFolder == "" {
		return templateLocation{}, nil, errUnresolvableRemote
	}

	joined := path.Join(parent.dir, templateURL)
	joined = path.Clean(joined)

	if joined == "" {
		joined = "."
	}

	return templateLocation{fsys: r.root, dir: joined}, nil, nil
}

// resolveRootLocation prepares the root template location, downloading it via
// go-getter if it's a remote URL.
func resolveRootLocation(l logging.Logger, opts *options.BoilerplateOptions) (templateLocation, func(), error) {
	if opts.TemplateFolder != "" {
		abs, err := filepath.Abs(opts.TemplateFolder)
		if err != nil {
			return templateLocation{}, nil, err
		}

		return templateLocation{fsys: os.DirFS(abs), dir: ".", absDir: abs}, nil, nil
	}

	if opts.TemplateURL == "" {
		return templateLocation{}, nil, fmt.Errorf("template-url is required")
	}

	workingDir, templateFolder, err := getterhelper.DownloadTemplatesToTemporaryFolder(l, opts.TemplateURL)

	cleanup := func() {
		if workingDir != "" {
			if rmErr := os.RemoveAll(workingDir); rmErr != nil {
				l.Errorf("failed to clean up working directory %s: %v", workingDir, rmErr)
			}
		}
	}

	if err != nil {
		return templateLocation{}, cleanup, err
	}

	opts.TemplateFolder = templateFolder

	abs, absErr := filepath.Abs(templateFolder)
	if absErr != nil {
		return templateLocation{}, cleanup, absErr
	}

	return templateLocation{fsys: os.DirFS(abs), dir: ".", absDir: abs}, cleanup, nil
}

// finalizeResult sorts file lists and error lists deterministically and
// ensures every declared input has at least an empty Files slice for stable
// JSON output.
func finalizeResult(r *Result) {
	for k, entry := range r.Inputs {
		if entry.Files == nil {
			entry.Files = []string{}
		}

		sort.Strings(entry.Files)
		r.Inputs[k] = entry
	}

	for k, list := range r.Files {
		sort.Strings(list)
		r.Files[k] = list
	}

	sort.Slice(r.Errors, func(i, j int) bool {
		a, b := r.Errors[i], r.Errors[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}

		if a.Template != b.Template {
			return a.Template < b.Template
		}

		if a.Name != b.Name {
			return a.Name < b.Name
		}

		return a.File < b.File
	})
}

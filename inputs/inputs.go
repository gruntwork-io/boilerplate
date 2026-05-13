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

	"github.com/gruntwork-io/boilerplate/config"
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

	// Sources maps each output path to the absolute path of the source
	// template file that produced it. Consumers that re-render a single file
	// (e.g., the WASM warm-dispatch path in runbooks) use this to locate the
	// template body to feed into boilerplateRenderTemplate.
	//
	// In OS mode (CLI) the values are absolute disk paths; for templates
	// pulled in via go-getter the path lives under the go-getter temp dir.
	// In FS mode (WASM) the values are slash-separated paths within the
	// supplied rootFS — no absolute disk path is available there.
	//
	// Files whose output path is dynamic and whose filename template failed
	// to render (KindFilenameRender) are absent from Sources; the missing
	// entry plus the existing soft error tells consumers to fall back to a
	// full render rather than guess at a path.
	Sources map[string]string `json:"sources"`

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
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Files       []string `json:"files"`
}

// AnalysisError is a soft error encountered during analysis. Soft errors do
// not abort the run; they accumulate in Result.Errors so the caller can
// surface them to the user. See the Kind* constants for the canonical set of
// values that may appear in the Kind field.
type AnalysisError struct {
	Kind     string `json:"kind"`
	Template string `json:"template,omitempty"`
	Name     string `json:"name,omitempty"`
	File     string `json:"file,omitempty"`
	Message  string `json:"message,omitempty"`
}

// Kind values that may appear in AnalysisError.Kind. Listed here so callers
// can switch on a stable identifier rather than a string literal.
const (
	// KindUndeclaredVariable: referenced in a template body but not declared
	// in any boilerplate.yml in scope.
	KindUndeclaredVariable = "undeclared_variable"

	// KindCycle: a dependency cycle was detected.
	KindCycle = "cycle"

	// KindUnresolvableDependency: a remote URL in FS-only mode, or a path
	// that does not exist.
	KindUnresolvableDependency = "unresolvable_dependency"

	// KindFilenameRender: failed to render a template-bearing filename.
	KindFilenameRender = "filename_render"

	// KindParse: failed to parse a template body or value expression.
	KindParse = "parse"

	// KindParseArgs: failed to parse CLI arguments before analysis began.
	KindParseArgs = "parse_args"

	// KindSkipFiles: failed to render or expand a skip_files entry's path,
	// not_path, or if condition.
	KindSkipFiles = "skip_files"

	// KindPartialExpansionLimit: partial-template invocation graph did not
	// reach a fixed point within the analyzer's iteration cap; results may
	// be missing some transitive references.
	KindPartialExpansionLimit = "partial_expansion_limit"
)

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

	// Honor opts.OnMissingConfig at the root: when set to Exit, a missing
	// boilerplate.yml at the resolved template-url is a hard error rather
	// than an empty-result success. Children reached via declared
	// dependencies are not subject to this check — a dep template-url that
	// happens to point at a directory with no config still degrades to the
	// "config-less template (no inputs)" path inside loadConfig, since
	// failing the whole run on a single missing child would defeat the
	// soft-error model the rest of the analyzer uses.
	if opts.OnMissingConfig == options.Exit {
		cfgPath := path.Join(rootLoc.dir, config.BoilerplateConfigFile)
		if _, statErr := fs.Stat(rootLoc.fsys, cfgPath); statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				return nil, fmt.Errorf("no %s found at template root (set --%s=ignore to allow)", config.BoilerplateConfigFile, options.OptMissingConfigAction)
			}

			return nil, fmt.Errorf("stat %s: %w", cfgPath, statErr)
		}
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
		// Local path. Absolute paths (commonly produced by templates that use
		// {{ templateFolder }}/../sibling) are used as-is; relative paths are
		// resolved against the parent template's directory.
		var joined string
		if filepath.IsAbs(templateURL) {
			joined = templateURL
		} else {
			if parent.absDir == "" {
				return templateLocation{}, nil, fmt.Errorf("cannot resolve relative dependency %q: parent has no on-disk path", templateURL)
			}

			joined = filepath.Join(parent.absDir, templateURL)
		}

		abs, absErr := filepath.Abs(joined)
		if absErr != nil {
			return templateLocation{}, nil, absErr
		}

		// Validate up-front that the resolved path exists and is a directory.
		// Without this check, os.DirFS would happily wrap a non-existent path
		// and the failure would surface much later as a cryptic
		// "stat .: no such file or directory" from the file walker. This
		// commonly happens when an output-folder template renders to junk
		// because the user's input vars were unset (e.g.,
		// "<no value>/_global/<no value>-role" pre-scrub).
		info, statErr := os.Stat(abs)
		if statErr != nil {
			return templateLocation{}, nil, fmt.Errorf("local dependency %q does not exist at %s: %w", templateURL, abs, statErr)
		}

		if !info.IsDir() {
			return templateLocation{}, nil, fmt.Errorf("local dependency %q resolved to %s, which is not a directory", templateURL, abs)
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
		return templateLocation{}, nil, errors.New("template-url is required")
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

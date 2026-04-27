//go:build !(js && wasm)

// Package render provides functionality for rendering templates and processing various file formats.
package render

import (
	"encoding/json"
	"path"

	jsonnet "github.com/google/go-jsonnet"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/vfs"
	"github.com/gruntwork-io/boilerplate/util"
)

// The following variables need to be stripped as it can't be rendered into json.
var incompatibleVariables = []string{
	"This",
}

// RenderJsonnetTemplate renders the jsonnet template at templatePath to the target location specified in the
// boilerplate options, passing in the boilerplate variables as Top Level Arguments. Note that this function will also
// make available the following helper values as External variables:
//
// - templateFolder
// - outputFolder
func RenderJsonnetTemplate(
	fsys vfs.FS,
	templatePath string,
	variables map[string]any,
	opts *options.BoilerplateOptions,
) (string, error) {
	jsonnetVM := jsonnet.MakeVM()
	jsonnetVM.Importer(&vfsImporter{fsys: fsys, anchor: templatePath})
	configureExternalVars(opts, jsonnetVM)

	if err := configureTLAVarsFromBoilerplateVars(jsonnetVM, variables); err != nil {
		return "", err
	}

	contents, err := vfs.ReadFile(fsys, templatePath)
	if err != nil {
		return "", err
	}

	output, err := jsonnetVM.EvaluateAnonymousSnippet(templatePath, string(contents))
	if err != nil {
		return "", err
	}

	return output, nil
}

// vfsImporter is a jsonnet.Importer that loads imports through a vfs.FS.
// anchor is the path the top-level snippet was loaded from; it is used to resolve
// relative imports when the importing file is not known (e.g. the entry snippet).
type vfsImporter struct {
	fsys   vfs.FS
	anchor string
}

// Import implements jsonnet.Importer by reading files through the configured filesystem.
func (i *vfsImporter) Import(importedFrom, importedPath string) (jsonnet.Contents, string, error) {
	from := importedFrom
	if from == "" {
		from = i.anchor
	}

	resolved := importedPath
	if !path.IsAbs(resolved) && from != "" {
		resolved = path.Join(path.Dir(from), importedPath)
	}

	data, err := vfs.ReadFile(i.fsys, resolved)
	if err != nil {
		return jsonnet.Contents{}, "", err
	}

	return jsonnet.MakeContents(string(data)), resolved, nil
}

// configureExternalVars registers the helper values as external variables for the jsonnet template engine.
func configureExternalVars(opts *options.BoilerplateOptions, vm *jsonnet.VM) {
	vm.ExtVar("templateFolder", opts.TemplateFolder)
	vm.ExtVar("outputFolder", opts.OutputFolder)
}

// configureTLAVarsFromBoilerplateVars translates the boilerplate variables into jsonnet values to make accessible to
// the top level function. Each boilerplate variable will be nested in an object boilerplateVars to avoid requiring
// every variable be defined.
// To pass through the boilerplate variables, we cheat by using json as an intermediary representation.
func configureTLAVarsFromBoilerplateVars(vm *jsonnet.VM, vars map[string]any) error {
	// Some of the auto injected vars are not json marshable at the moment, so we skip those.
	jsonCompatibleMap := map[string]any{}

	for k, v := range vars {
		if !util.ListContains(k, incompatibleVariables) {
			jsonCompatibleMap[k] = v
		}
	}

	jsonBytes, err := json.Marshal(jsonCompatibleMap)
	if err != nil {
		return err
	}

	vm.TLACode("boilerplateVars", string(jsonBytes))

	return nil
}

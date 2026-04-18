// Package bridge provides the JSON request/response types and translation
// helpers used by the WASM entry points in cmd/wasm. Keeping the marshalling
// code in a separate, build-tag-free package lets the host-side tests exercise
// it without a js runtime.
package bridge

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

// ProcessTemplateRequest is the JSON input shape accepted by the WASM
// boilerplateProcessTemplate entry point. Fields mirror a conservative subset
// of options.BoilerplateOptions; anything not listed here uses the defaults
// encoded in NewBoilerplateOptions.
type ProcessTemplateRequest struct {
	TemplateFolder          string         `json:"templateFolder"`
	OutputFolder            string         `json:"outputFolder"`
	Vars                    map[string]any `json:"vars,omitempty"`
	VarFiles                []string       `json:"varFiles,omitempty"`
	NonInteractive          *bool          `json:"nonInteractive,omitempty"`
	NoShell                 *bool          `json:"noShell,omitempty"`
	DisableDependencyPrompt *bool          `json:"disableDependencyPrompt,omitempty"`
	OnMissingKey            string         `json:"onMissingKey,omitempty"`
	OnMissingConfig         string         `json:"onMissingConfig,omitempty"`
	Manifest                *bool          `json:"manifest,omitempty"`
}

// ProcessTemplateResponse is the JSON output shape returned by the WASM
// boilerplateProcessTemplate entry point. Error is the empty string on success.
// Warnings is a list of non-fatal notices emitted during the run — e.g. that
// custom variable validations were skipped in the WASM build.
type ProcessTemplateResponse struct {
	Error          string   `json:"error"`
	GeneratedFiles []string `json:"generatedFiles"`
	SourceChecksum string   `json:"sourceChecksum"`
	Warnings       []string `json:"warnings"`
}

// ParseProcessTemplateRequest decodes the JSON-encoded request payload.
func ParseProcessTemplateRequest(reqJSON string) (*ProcessTemplateRequest, error) {
	if reqJSON == "" {
		return nil, errors.New("request JSON is empty")
	}

	req := &ProcessTemplateRequest{}
	if err := json.Unmarshal([]byte(reqJSON), req); err != nil {
		return nil, fmt.Errorf("failed to parse request JSON: %w", err)
	}

	return req, nil
}

// BuildBoilerplateOptions converts a request into a *options.BoilerplateOptions
// suitable for passing to templates.ProcessTemplateWithContext. VarFiles on
// disk are read here so that an invalid var file produces a validation error
// synchronously (matching CLI behavior). Values from var files override inline
// vars on key conflict — the same precedence applied by variables.ParseVars
// (see variables/yaml_helpers.go: MergeMaps is called with inline vars first
// and var-file vars last, so var-file values win).
func BuildBoilerplateOptions(req *ProcessTemplateRequest) (*options.BoilerplateOptions, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	if req.TemplateFolder == "" {
		return nil, errors.New("templateFolder is required")
	}

	if req.OutputFolder == "" {
		return nil, errors.New("outputFolder is required")
	}

	inlineVars := req.Vars
	if inlineVars == nil {
		inlineVars = map[string]any{}
	}

	varsFromFiles := map[string]any{}

	for _, varFile := range req.VarFiles {
		parsed, err := variables.ParseVariablesFromVarFile(varFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse var file %q: %w", varFile, err)
		}

		varsFromFiles = util.MergeMaps(varsFromFiles, parsed)
	}

	mergedVars := util.MergeMaps(inlineVars, varsFromFiles)

	missingKey := options.ExitWithError
	if req.OnMissingKey != "" {
		parsed, err := options.ParseMissingKeyAction(req.OnMissingKey)
		if err != nil {
			return nil, err
		}

		missingKey = parsed
	}

	missingConfig := options.Ignore
	if req.OnMissingConfig != "" {
		parsed, err := options.ParseMissingConfigAction(req.OnMissingConfig)
		if err != nil {
			return nil, err
		}

		missingConfig = parsed
	}

	opts := &options.BoilerplateOptions{
		TemplateFolder:          req.TemplateFolder,
		OutputFolder:            req.OutputFolder,
		Vars:                    mergedVars,
		ShellCommandAnswers:     map[string]bool{},
		OnMissingKey:            missingKey,
		OnMissingConfig:         missingConfig,
		NonInteractive:          boolOrDefault(req.NonInteractive, true),
		NoShell:                 boolOrDefault(req.NoShell, true),
		DisableDependencyPrompt: boolOrDefault(req.DisableDependencyPrompt, true),
		Manifest:                boolOrDefault(req.Manifest, false),
	}

	return opts, nil
}

// ErrorResponse builds a ProcessTemplateResponse representing a failed run.
// Callers return an error in a data field rather than throwing a JS Error so
// that the downstream consumer can branch on result.error without wrapping the
// call in try/catch. Any warnings accumulated before the failure are still
// passed through so the caller can see them.
func ErrorResponse(err error, warnings []string) *ProcessTemplateResponse {
	msg := ""
	if err != nil {
		msg = err.Error()
	}

	if warnings == nil {
		warnings = []string{}
	}

	return &ProcessTemplateResponse{
		Error:          msg,
		GeneratedFiles: []string{},
		SourceChecksum: "",
		Warnings:       warnings,
	}
}

// SuccessResponse builds a ProcessTemplateResponse from a ProcessResult-like
// tuple. Pass generated and checksum directly rather than importing
// templates.ProcessResult to keep this package free of build-tagged
// dependencies.
func SuccessResponse(generated []string, checksum string, warnings []string) *ProcessTemplateResponse {
	if generated == nil {
		generated = []string{}
	}

	if warnings == nil {
		warnings = []string{}
	}

	return &ProcessTemplateResponse{
		Error:          "",
		GeneratedFiles: generated,
		SourceChecksum: checksum,
		Warnings:       warnings,
	}
}

func boolOrDefault(v *bool, def bool) bool {
	if v == nil {
		return def
	}

	return *v
}

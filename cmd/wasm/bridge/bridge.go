// Package bridge holds the JSON request/response types for the WASM entry
// points. Kept free of build tags so host-side tests can exercise it.
package bridge

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

// ProcessTemplateRequest is the JSON input for boilerplateProcessTemplate.
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

// ProcessTemplateResponse is the JSON output for boilerplateProcessTemplate.
// Error is empty on success.
type ProcessTemplateResponse struct {
	Error          string   `json:"error"`
	GeneratedFiles []string `json:"generatedFiles"`
	SourceChecksum string   `json:"sourceChecksum"`
	Warnings       []string `json:"warnings"`
}

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

// BuildBoilerplateOptions converts a request into BoilerplateOptions. Values
// from varFiles override inline vars on key conflict, matching the CLI.
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

// ErrorResponse returns a failure response. The error goes in a data field
// rather than a thrown JS Error so callers can branch on result.error.
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

// SuccessResponse takes generated and checksum directly to keep this package
// free of build-tagged dependencies on templates.ProcessResult.
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

package bridge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/boilerplate/options"
)

func TestParseProcessTemplateRequest(t *testing.T) {
	t.Parallel()

	in := `{
	    "templateFolder": "/tmpl",
	    "outputFolder": "/out",
	    "vars": {"Name": "alice", "Count": 3},
	    "varFiles": ["/a.yml", "/b.yml"],
	    "nonInteractive": false,
	    "onMissingKey": "zero",
	    "manifest": true
	}`

	req, err := ParseProcessTemplateRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.TemplateFolder != "/tmpl" || req.OutputFolder != "/out" {
		t.Errorf("folders not parsed: %+v", req)
	}

	if req.Vars["Name"] != "alice" {
		t.Errorf("vars not parsed: %+v", req.Vars)
	}

	if len(req.VarFiles) != 2 {
		t.Errorf("var files not parsed: %+v", req.VarFiles)
	}

	if req.NonInteractive == nil || *req.NonInteractive != false {
		t.Errorf("nonInteractive pointer: %+v", req.NonInteractive)
	}

	if req.Manifest == nil || *req.Manifest != true {
		t.Errorf("manifest pointer: %+v", req.Manifest)
	}

	if req.OnMissingKey != "zero" {
		t.Errorf("onMissingKey: %q", req.OnMissingKey)
	}
}

func TestParseProcessTemplateRequestEmpty(t *testing.T) {
	t.Parallel()

	if _, err := ParseProcessTemplateRequest(""); err == nil {
		t.Fatal("expected error for empty JSON")
	}

	if _, err := ParseProcessTemplateRequest("not-json"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestBuildBoilerplateOptionsDefaults(t *testing.T) {
	t.Parallel()

	opts, err := BuildBoilerplateOptions(&ProcessTemplateRequest{
		TemplateFolder: "/tmpl",
		OutputFolder:   "/out",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.TemplateFolder != "/tmpl" || opts.OutputFolder != "/out" {
		t.Errorf("folders not set: %+v", opts)
	}

	if !opts.NonInteractive {
		t.Error("NonInteractive default should be true")
	}

	if !opts.NoShell {
		t.Error("NoShell default should be true")
	}

	if !opts.DisableDependencyPrompt {
		t.Error("DisableDependencyPrompt default should be true")
	}

	if opts.Manifest {
		t.Error("Manifest default should be false")
	}

	if opts.OnMissingKey != options.ExitWithError {
		t.Errorf("OnMissingKey default = %v", opts.OnMissingKey)
	}

	if opts.OnMissingConfig != options.Ignore {
		t.Errorf("OnMissingConfig default = %v", opts.OnMissingConfig)
	}

	if opts.Vars == nil {
		t.Error("Vars should be non-nil")
	}

	if opts.ShellCommandAnswers == nil {
		t.Error("ShellCommandAnswers should be non-nil")
	}
}

func TestBuildBoilerplateOptionsOverrides(t *testing.T) {
	t.Parallel()

	falsePtr := false
	truePtr := true

	opts, err := BuildBoilerplateOptions(&ProcessTemplateRequest{
		TemplateFolder:          "/tmpl",
		OutputFolder:            "/out",
		Vars:                    map[string]any{"Name": "alice"},
		NonInteractive:          &falsePtr,
		NoShell:                 &falsePtr,
		DisableDependencyPrompt: &falsePtr,
		Manifest:                &truePtr,
		OnMissingKey:            "zero",
		OnMissingConfig:         "exit",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.NonInteractive || opts.NoShell || opts.DisableDependencyPrompt {
		t.Errorf("bool overrides not applied: %+v", opts)
	}

	if !opts.Manifest {
		t.Error("Manifest override not applied")
	}

	if opts.OnMissingKey != options.ZeroValue {
		t.Errorf("OnMissingKey override: %v", opts.OnMissingKey)
	}

	if opts.OnMissingConfig != options.Exit {
		t.Errorf("OnMissingConfig override: %v", opts.OnMissingConfig)
	}

	if opts.Vars["Name"] != "alice" {
		t.Errorf("inline Vars not preserved: %+v", opts.Vars)
	}
}

func TestBuildBoilerplateOptionsValidation(t *testing.T) {
	t.Parallel()

	if _, err := BuildBoilerplateOptions(nil); err == nil {
		t.Error("expected error for nil request")
	}

	if _, err := BuildBoilerplateOptions(&ProcessTemplateRequest{OutputFolder: "/out"}); err == nil {
		t.Error("expected error for missing templateFolder")
	}

	if _, err := BuildBoilerplateOptions(&ProcessTemplateRequest{TemplateFolder: "/tmpl"}); err == nil {
		t.Error("expected error for missing outputFolder")
	}

	if _, err := BuildBoilerplateOptions(&ProcessTemplateRequest{
		TemplateFolder: "/tmpl",
		OutputFolder:   "/out",
		OnMissingKey:   "nonsense",
	}); err == nil {
		t.Error("expected error for invalid onMissingKey")
	}

	if _, err := BuildBoilerplateOptions(&ProcessTemplateRequest{
		TemplateFolder:  "/tmpl",
		OutputFolder:    "/out",
		OnMissingConfig: "nonsense",
	}); err == nil {
		t.Error("expected error for invalid onMissingConfig")
	}
}

func TestBuildBoilerplateOptionsVarFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	varFile := filepath.Join(dir, "vars.yml")

	if err := os.WriteFile(varFile, []byte("Name: fromfile\nCount: 7\n"), 0o600); err != nil {
		t.Fatalf("write var file: %v", err)
	}

	opts, err := BuildBoilerplateOptions(&ProcessTemplateRequest{
		TemplateFolder: "/tmpl",
		OutputFolder:   "/out",
		Vars:           map[string]any{"Name": "fromInline", "Extra": "keep"},
		VarFiles:       []string{varFile},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// VarFiles override inline vars (matches cli/parse_options.go precedence).
	if opts.Vars["Name"] != "fromfile" {
		t.Errorf("var file should override inline vars: got %v", opts.Vars["Name"])
	}

	if opts.Vars["Count"] != 7 {
		t.Errorf("var file Count not merged: %v", opts.Vars["Count"])
	}

	if opts.Vars["Extra"] != "keep" {
		t.Errorf("inline var not preserved when file lacks key: %v", opts.Vars["Extra"])
	}
}

func TestBuildBoilerplateOptionsVarFileMissing(t *testing.T) {
	t.Parallel()

	_, err := BuildBoilerplateOptions(&ProcessTemplateRequest{
		TemplateFolder: "/tmpl",
		OutputFolder:   "/out",
		VarFiles:       []string{"/definitely/does/not/exist.yml"},
	})
	if err == nil {
		t.Error("expected error for missing var file")
	}
}

func TestErrorResponse(t *testing.T) {
	t.Parallel()

	resp := ErrorResponse(nil)
	if resp.Error != "" {
		t.Errorf("nil err should yield empty message, got %q", resp.Error)
	}

	if resp.GeneratedFiles == nil {
		t.Error("GeneratedFiles should be non-nil empty slice")
	}

	resp = ErrorResponse(os.ErrNotExist)
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestSuccessResponse(t *testing.T) {
	t.Parallel()

	resp := SuccessResponse([]string{"a", "b"}, "deadbeef")
	if resp.Error != "" {
		t.Errorf("error should be empty: %q", resp.Error)
	}

	if len(resp.GeneratedFiles) != 2 || resp.SourceChecksum != "deadbeef" {
		t.Errorf("fields not set: %+v", resp)
	}

	resp = SuccessResponse(nil, "")
	if resp.GeneratedFiles == nil {
		t.Error("GeneratedFiles should be non-nil empty slice")
	}
}

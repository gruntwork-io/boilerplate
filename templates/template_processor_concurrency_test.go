package templates_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/templates"
	"github.com/gruntwork-io/boilerplate/variables"
)

func TestForEachConcurrent_AllItemsProcessed(t *testing.T) {
	t.Parallel()

	items := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}
	opts := createForEachFixture(t, items)
	opts.Parallelism = 4

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	for _, item := range items {
		content := readOutputFile(t, opts, item)
		assert.Equal(t, "item="+item, content, "output for item %q", item)
	}
}

func TestForEachConcurrent_VariableIsolation(t *testing.T) {
	t.Parallel()

	// Use enough items and high parallelism to provoke any sharing bugs.
	items := make([]string, 50)
	for i := range items {
		items[i] = fmt.Sprintf("item-%03d", i)
	}

	opts := createForEachFixture(t, items)
	opts.Parallelism = runtime.NumCPU()

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	for _, item := range items {
		content := readOutputFile(t, opts, item)
		assert.Equal(t, "item="+item, content, "variable isolation broken for %q", item)
	}
}

func TestForEachConcurrent_ParallelismOne_Sequential(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c", "d", "e"}
	opts := createForEachFixture(t, items)
	opts.Parallelism = 1

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	for _, item := range items {
		content, err := os.ReadFile(filepath.Join(opts.OutputFolder, item, "output.txt"))
		require.NoError(t, err)
		assert.Equal(t, "item="+item, string(content))
	}
}

func TestForEachConcurrent_ErrorPropagation(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Child template with an undefined variable reference
	childDir := filepath.Join(tempDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "boilerplate.yml"), []byte("variables: []\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "output.txt"), []byte("{{ .UndefinedVar }}"), 0o644))

	parentDir := filepath.Join(tempDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	parentConfig := `variables: []
dependencies:
  - name: error-dep
    template-url: ../child
    output-folder: "{{ .__each__ }}"
    for_each:
      - a
      - b
      - c
`
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "boilerplate.yml"), []byte(parentConfig), 0o644))

	opts := &options.BoilerplateOptions{
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             parentDir,
		TemplateFolder:          parentDir,
		OutputFolder:            filepath.Join(tempDir, "output"),
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NonInteractive:          true,
		NoHooks:                 true,
		DisableDependencyPrompt: true,
		Parallelism:             4,
	}

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.Error(t, err, "expected error from undefined variable to propagate")
}

func TestForEachConcurrent_HighParallelism_ActuallyConcurrent(t *testing.T) {
	t.Parallel()

	if runtime.NumCPU() < 2 {
		t.Skip("need at least 2 CPUs to test actual concurrency")
	}

	tempDir := t.TempDir()

	// Child template with a sleep hook
	childDir := filepath.Join(tempDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))

	childConfig := `variables: []
hooks:
  before:
    - command: sleep
      args:
        - "0.2"
`
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "boilerplate.yml"), []byte(childConfig), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "output.txt"), []byte("{{ .__each__ }}"), 0o644))

	items := []string{"a", "b", "c", "d"}

	parentDir := filepath.Join(tempDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	forEachYAML := "    for_each:\n"

	var forEachYAMLSb strings.Builder
	for _, item := range items {
		forEachYAMLSb.WriteString(fmt.Sprintf("      - %s\n", item))
	}

	forEachYAML += forEachYAMLSb.String()

	parentConfig := "variables: []\ndependencies:\n  - name: timing-dep\n    template-url: ../child\n    output-folder: \"{{ .__each__ }}\"\n" + forEachYAML
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "boilerplate.yml"), []byte(parentConfig), 0o644))

	opts := &options.BoilerplateOptions{
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             parentDir,
		TemplateFolder:          parentDir,
		OutputFolder:            filepath.Join(tempDir, "output"),
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NonInteractive:          true,
		DisableDependencyPrompt: true,
		Parallelism:             len(items),
	}

	start := time.Now()
	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	elapsed := time.Since(start)

	require.NoError(t, err)

	// 4 items x 200ms each = 800ms serial. With full parallelism should be ~200ms.
	// Use a generous threshold to avoid flaky tests: just check it's under 600ms
	// (which proves at least some concurrency happened).
	serialTime := time.Duration(len(items)) * 200 * time.Millisecond
	assert.Less(t, elapsed, serialTime,
		"expected concurrent execution to be faster than serial (%s), but took %s", serialTime, elapsed)

	for _, item := range items {
		content, err := os.ReadFile(filepath.Join(opts.OutputFolder, item, "output.txt"))
		require.NoError(t, err)
		assert.Equal(t, item, string(content))
	}
}

func TestForEachConcurrent_ParallelismLimitsActiveTasks(t *testing.T) {
	t.Parallel()

	if runtime.NumCPU() < 2 {
		t.Skip("need at least 2 CPUs to test parallelism limiting")
	}

	tempDir := t.TempDir()

	childDir := filepath.Join(tempDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))

	childConfig := `variables: []
hooks:
  before:
    - command: sleep
      args:
        - "0.1"
`
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "boilerplate.yml"), []byte(childConfig), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "output.txt"), []byte("{{ .__each__ }}"), 0o644))

	items := []string{"a", "b", "c", "d", "e", "f"}

	parentDir := filepath.Join(tempDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	forEachYAML := "    for_each:\n"

	var forEachYAMLSb strings.Builder
	for _, item := range items {
		forEachYAMLSb.WriteString(fmt.Sprintf("      - %s\n", item))
	}

	forEachYAML += forEachYAMLSb.String()

	parentConfig := "variables: []\ndependencies:\n  - name: limit-dep\n    template-url: ../child\n    output-folder: \"{{ .__each__ }}\"\n" + forEachYAML
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "boilerplate.yml"), []byte(parentConfig), 0o644))

	opts := &options.BoilerplateOptions{
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             parentDir,
		TemplateFolder:          parentDir,
		OutputFolder:            filepath.Join(tempDir, "output"),
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NonInteractive:          true,
		DisableDependencyPrompt: true,
		Parallelism:             2,
	}

	start := time.Now()
	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	elapsed := time.Since(start)

	require.NoError(t, err)

	// 6 items, parallelism 2, each 100ms => at least 3 batches => ~300ms minimum.
	// With full parallelism (6) it would be ~100ms.
	// Assert it took at least 250ms, proving the limit was respected.
	minExpected := 250 * time.Millisecond
	assert.GreaterOrEqual(t, elapsed, minExpected,
		"expected parallelism=2 to take at least %s for 6 items, but took %s", minExpected, elapsed)

	for _, item := range items {
		content, err := os.ReadFile(filepath.Join(opts.OutputFolder, item, "output.txt"))
		require.NoError(t, err)
		assert.Equal(t, item, string(content))
	}
}

func TestForEachConcurrent_OriginalVarsNotMutated(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	childDir := filepath.Join(tempDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "boilerplate.yml"), []byte("variables: []\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "output.txt"), []byte("item={{ .__each__ }}"), 0o644))

	parentDir := filepath.Join(tempDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	parentConfig := `variables: []
dependencies:
  - name: mutation-dep
    template-url: ../child
    output-folder: "{{ .__each__ }}"
    for_each:
      - x
      - "y"
      - z
`
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "boilerplate.yml"), []byte(parentConfig), 0o644))

	cliVars := map[string]any{
		"existing_key": "existing_value",
	}

	// Take a snapshot before processing
	snapshotBefore := map[string]any{
		"existing_key": "existing_value",
	}

	opts := &options.BoilerplateOptions{
		Vars:                    cliVars,
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             parentDir,
		TemplateFolder:          parentDir,
		OutputFolder:            filepath.Join(tempDir, "output"),
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NonInteractive:          true,
		NoHooks:                 true,
		DisableDependencyPrompt: true,
		Parallelism:             runtime.NumCPU(),
	}

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	// CLI vars must not have been modified (no __each__ key injected, etc.)
	assert.Equal(t, snapshotBefore, cliVars, "CLI vars were mutated during concurrent for_each")
}

func TestForEachConcurrent_EmptyForEach_FallsThrough(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	childDir := filepath.Join(tempDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "boilerplate.yml"), []byte("variables: []\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "output.txt"), []byte("no-loop"), 0o644))

	parentDir := filepath.Join(tempDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	parentConfig := `variables: []
dependencies:
  - name: no-loop-dep
    template-url: ../child
    output-folder: out
    for_each: []
`
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "boilerplate.yml"), []byte(parentConfig), 0o644))

	outputDir := filepath.Join(tempDir, "output")

	opts := &options.BoilerplateOptions{
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             parentDir,
		TemplateFolder:          parentDir,
		OutputFolder:            outputDir,
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NonInteractive:          true,
		NoHooks:                 true,
		DisableDependencyPrompt: true,
		Parallelism:             4,
	}

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	// The non-for_each path should have produced output in "out/"
	content, err := os.ReadFile(filepath.Join(outputDir, "out", "output.txt"))
	require.NoError(t, err)
	assert.Equal(t, "no-loop", string(content))
}

func TestForEachConcurrent_ForEachReference_Concurrent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	childDir := filepath.Join(tempDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "boilerplate.yml"), []byte("variables: []\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "output.txt"), []byte("{{ .__each__ }}"), 0o644))

	parentDir := filepath.Join(tempDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	parentConfig := `variables:
  - name: myList
    type: list
    default:
      - one
      - two
      - three
dependencies:
  - name: ref-dep
    template-url: ../child
    output-folder: "{{ .__each__ }}"
    for_each_reference: myList
`
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "boilerplate.yml"), []byte(parentConfig), 0o644))

	outputDir := filepath.Join(tempDir, "output")

	opts := &options.BoilerplateOptions{
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             parentDir,
		TemplateFolder:          parentDir,
		OutputFolder:            outputDir,
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NonInteractive:          true,
		NoHooks:                 true,
		DisableDependencyPrompt: true,
		Parallelism:             4,
	}

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	for _, item := range []string{"one", "two", "three"} {
		content, err := os.ReadFile(filepath.Join(outputDir, item, "output.txt"))
		require.NoError(t, err)
		assert.Equal(t, item, string(content))
	}
}

func TestForEachConcurrent_ManyItems_StressTest(t *testing.T) {
	t.Parallel()

	const numItems = 100

	items := make([]string, numItems)
	for i := range items {
		items[i] = fmt.Sprintf("stress-%03d", i)
	}

	opts := createForEachFixture(t, items)
	opts.Parallelism = runtime.NumCPU()

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	for _, item := range items {
		content := readOutputFile(t, opts, item)
		assert.Equal(t, "item="+item, content, "mismatch for %q under stress", item)
	}
}

// TestForEachConcurrent_OrderingPreservedAtParallelismOne verifies that with
// parallelism=1, items are processed in the order they appear in the for_each list.
// We observe this by recording file mod-times: each successive file
// should have a mod-time >= the previous one.
func TestForEachConcurrent_OrderingPreservedAtParallelismOne(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	childDir := filepath.Join(tempDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))

	childConfig := `variables: []
hooks:
  after:
    - command: sleep
      args:
        - "0.05"
`
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "boilerplate.yml"), []byte(childConfig), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "output.txt"), []byte("{{ .__each__ }}"), 0o644))

	items := []string{"first", "second", "third", "fourth"}

	parentDir := filepath.Join(tempDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	forEachYAML := "    for_each:\n"

	var forEachYAMLSb strings.Builder
	for _, item := range items {
		forEachYAMLSb.WriteString(fmt.Sprintf("      - %s\n", item))
	}

	forEachYAML += forEachYAMLSb.String()

	parentConfig := "variables: []\ndependencies:\n  - name: order-dep\n    template-url: ../child\n    output-folder: \"{{ .__each__ }}\"\n" + forEachYAML
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "boilerplate.yml"), []byte(parentConfig), 0o644))

	outputDir := filepath.Join(tempDir, "output")

	opts := &options.BoilerplateOptions{
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             parentDir,
		TemplateFolder:          parentDir,
		OutputFolder:            outputDir,
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NonInteractive:          true,
		DisableDependencyPrompt: true,
		Parallelism:             1,
	}

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	modTimes := make([]time.Time, 0, len(items))

	for _, item := range items {
		info, err := os.Stat(filepath.Join(outputDir, item, "output.txt"))
		require.NoError(t, err)

		modTimes = append(modTimes, info.ModTime())
	}

	for i := 1; i < len(modTimes); i++ {
		assert.False(t, modTimes[i].Before(modTimes[i-1]),
			"item %q (mod %v) was written before item %q (mod %v) — ordering violated",
			items[i], modTimes[i], items[i-1], modTimes[i-1])
	}
}

// TestForEachConcurrent_EachIterationGetsDistinctOutputDir verifies there are no
// path collisions — every item creates its own unique directory.
func TestForEachConcurrent_EachIterationGetsDistinctOutputDir(t *testing.T) {
	t.Parallel()

	items := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	opts := createForEachFixture(t, items)
	opts.Parallelism = runtime.NumCPU()

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	entries, err := os.ReadDir(opts.OutputFolder)
	require.NoError(t, err)

	var dirs []string

	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}

	sort.Strings(dirs)
	sort.Strings(items)
	assert.Equal(t, items, dirs, "output directories should match for_each items exactly")
}

// TestForEachConcurrent_SharedVarsReadSafety verifies that shared read-only
// variables (originalVars) can be safely read by multiple concurrent goroutines
// without data races. Run this test with -race to detect issues.
func TestForEachConcurrent_SharedVarsReadSafety(t *testing.T) {
	t.Parallel()

	items := make([]string, 30)
	for i := range items {
		items[i] = fmt.Sprintf("race-%02d", i)
	}

	opts := createForEachFixture(t, items)
	opts.Parallelism = runtime.NumCPU()
	opts.Vars = map[string]any{
		"string_var": "hello",
		"int_var":    42,
		"list_var":   []string{"a", "b", "c"},
		"map_var":    map[string]any{"nested": "value"},
	}

	// The race detector (go test -race) will catch concurrent map read/write issues.
	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.NoError(t, err)

	for _, item := range items {
		content := readOutputFile(t, opts, item)
		assert.Equal(t, "item="+item, content)
	}
}

// TestForEachConcurrent_PartialFailure verifies that when one iteration fails,
// the error from the failing iteration is returned.
func TestForEachConcurrent_PartialFailure(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	childDir := filepath.Join(tempDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "boilerplate.yml"), []byte("variables: []\n"), 0o644))

	templateContent := `{{ if eq .__each__ "fail" }}{{ .ThisVarDoesNotExist }}{{ else }}{{ .__each__ }}{{ end }}`
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "output.txt"), []byte(templateContent), 0o644))

	parentDir := filepath.Join(tempDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	parentConfig := `variables: []
dependencies:
  - name: partial-dep
    template-url: ../child
    output-folder: "{{ .__each__ }}"
    for_each:
      - ok1
      - ok2
      - fail
      - ok3
`
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "boilerplate.yml"), []byte(parentConfig), 0o644))

	opts := &options.BoilerplateOptions{
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             parentDir,
		TemplateFolder:          parentDir,
		OutputFolder:            filepath.Join(tempDir, "output"),
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NonInteractive:          true,
		NoHooks:                 true,
		DisableDependencyPrompt: true,
		Parallelism:             1,
	}

	err := templates.ProcessTemplateWithContext(t.Context(), opts, opts, &variables.Dependency{})
	require.Error(t, err, "should propagate error from failing iteration")
}

// TestErrGroupSetLimitOne_Sequential directly verifies that errgroup.SetLimit(1)
// processes tasks sequentially by tracking concurrent execution.
func TestErrGroupSetLimitOne_Sequential(t *testing.T) {
	t.Parallel()

	var (
		mu            sync.Mutex
		maxConcurrent int32
		current       atomic.Int32
	)

	order := make([]int, 0, 10)

	g, _ := errgroup.WithContext(t.Context())
	g.SetLimit(1)

	for i := range 10 {
		g.Go(func() error {
			c := current.Add(1)

			mu.Lock()

			order = append(order, i)

			if c > maxConcurrent {
				maxConcurrent = c
			}
			mu.Unlock()

			// Simulate work
			time.Sleep(5 * time.Millisecond)

			current.Add(-1)

			return nil
		})
	}

	require.NoError(t, g.Wait())

	assert.Equal(t, int32(1), maxConcurrent, "with SetLimit(1), at most 1 goroutine should be active at a time")
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, order, "with SetLimit(1), tasks should execute in submission order")
}

// createForEachFixture sets up a parent template with a for_each dependency
// pointing to a child template that renders {{ .__each__ }} into output.txt.
// Returns the configured options pointing at the parent template.
func createForEachFixture(t *testing.T, items []string) *options.BoilerplateOptions {
	t.Helper()

	tempDir := t.TempDir()

	// Child template: renders __each__ into output.txt
	childDir := filepath.Join(tempDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "boilerplate.yml"), []byte("variables: []\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "output.txt"), []byte("item={{ .__each__ }}"), 0o644))

	// Parent template: for_each dependency on child
	parentDir := filepath.Join(tempDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	forEachYAML := "    for_each:\n"

	var forEachYAMLSb strings.Builder
	for _, item := range items {
		forEachYAMLSb.WriteString(fmt.Sprintf("      - %s\n", item))
	}

	forEachYAML += forEachYAMLSb.String()

	parentConfig := "variables: []\ndependencies:\n  - name: test-dep\n    template-url: ../child\n    output-folder: \"{{ .__each__ }}\"\n" + forEachYAML
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "boilerplate.yml"), []byte(parentConfig), 0o644))

	outputDir := filepath.Join(tempDir, "output")

	return &options.BoilerplateOptions{
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             parentDir,
		TemplateFolder:          parentDir,
		OutputFolder:            outputDir,
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NonInteractive:          true,
		NoHooks:                 true,
		DisableDependencyPrompt: true,
	}
}

// readOutputFile reads the rendered output.txt for a given for_each item.
func readOutputFile(t *testing.T, opts *options.BoilerplateOptions, item string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(opts.OutputFolder, item, "output.txt"))
	require.NoError(t, err)

	return string(content)
}

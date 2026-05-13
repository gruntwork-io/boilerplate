package inputs

import (
	"context"
	"text/template"
	"text/template/parse"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/render"
)

// Built-in identifiers that boilerplate exposes via the variables map but that
// are not user-declared inputs. These should never appear in the result.
var builtinVarNames = map[string]struct{}{
	"BoilerplateConfigVars": {},
	"BoilerplateConfigDeps": {},
	"This":                  {},
	"__each__":              {},
}

// templateRefs is the result of walking a parsed template tree.
type templateRefs struct {
	// vars is the set of top-level variable names referenced via {{ .X }}.
	vars map[string]struct{}
	// invocations is the set of named templates this template invokes via
	// {{ template "name" . }}, used to expand partials transitively.
	invocations map[string]struct{}
}

func newTemplateRefs() *templateRefs {
	return &templateRefs{
		vars:        map[string]struct{}{},
		invocations: map[string]struct{}{},
	}
}

// parseTemplateAll parses contents and returns every named template defined
// (the body itself plus any {{ define "name" }} blocks). Each entry is the
// parse tree for one of those templates.
func parseTemplateAll(name, contents string) ([]*parse.Tree, error) {
	tmpl := template.New(name).Funcs(stubFuncs).Option("missingkey=zero")

	parsed, err := tmpl.Parse(contents)
	if err != nil {
		return nil, err
	}

	out := make([]*parse.Tree, 0, len(parsed.Templates()))

	for _, t := range parsed.Templates() {
		if t.Tree != nil {
			out = append(out, t.Tree)
		}
	}

	return out, nil
}

// walkTree walks a parsed template tree and collects variable references and
// template invocations into refs.
func walkTree(tree *parse.Tree, refs *templateRefs) {
	if tree == nil || tree.Root == nil {
		return
	}

	walkNode(tree.Root, refs)
}

func walkNode(node parse.Node, refs *templateRefs) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}

		for _, child := range n.Nodes {
			walkNode(child, refs)
		}
	case *parse.ActionNode:
		walkPipe(n.Pipe, refs)
	case *parse.IfNode:
		walkPipe(n.Pipe, refs)
		walkNode(n.List, refs)
		walkNode(n.ElseList, refs)
	case *parse.RangeNode:
		walkPipe(n.Pipe, refs)
		walkNode(n.List, refs)
		walkNode(n.ElseList, refs)
	case *parse.WithNode:
		walkPipe(n.Pipe, refs)
		walkNode(n.List, refs)
		walkNode(n.ElseList, refs)
	case *parse.TemplateNode:
		// {{ template "name" pipeline }} — record the invocation so the caller
		// can transitively pull in referenced inputs from the named partial,
		// and walk the optional pipeline argument.
		if n.Name != "" {
			refs.invocations[n.Name] = struct{}{}
		}

		walkPipe(n.Pipe, refs)
	}
}

func walkPipe(pipe *parse.PipeNode, refs *templateRefs) {
	if pipe == nil {
		return
	}

	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			walkArg(arg, refs)
		}
	}
}

func walkArg(node parse.Node, refs *templateRefs) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parse.FieldNode:
		// {{ .Foo }} — first ident is the top-level variable name. Trailing
		// idents (.Foo.Bar.Baz) are field accesses; only Foo is the input.
		if len(n.Ident) > 0 {
			name := n.Ident[0]
			if _, builtin := builtinVarNames[name]; !builtin {
				refs.vars[name] = struct{}{}
			}
		}
	case *parse.PipeNode:
		walkPipe(n, refs)
	case *parse.CommandNode:
		for _, arg := range n.Args {
			walkArg(arg, refs)
		}
	case *parse.ChainNode:
		// {{ (something).Foo.Bar }} — walk the inner node; the chain's field
		// accesses do not introduce new top-level vars.
		walkArg(n.Node, refs)
	}

	// Other node kinds (StringNode, NumberNode, BoolNode, IdentifierNode,
	// VariableNode for $-prefixed locals, NilNode, DotNode) do not introduce
	// references to declared inputs.
}

// extractRefs is a convenience helper that parses contents and returns the
// referenced top-level vars and template invocations.
func extractRefs(name, contents string) (*templateRefs, error) {
	refs := newTemplateRefs()

	trees, err := parseTemplateAll(name, contents)
	if err != nil {
		return nil, err
	}

	for _, t := range trees {
		walkTree(t, refs)
	}

	return refs, nil
}

// stubFuncs is the FuncMap registered when parsing templates for analysis.
// It contains every identifier the runtime exposes (sprig + boilerplate
// helpers) but with each value swapped for a no-op stub. text/template only
// checks identifier existence at parse time, so the stubs are sufficient to
// let arbitrary templates parse — and analysis never calls Execute, so the
// stubs are never invoked.
//
// Derived from render.CreateTemplateHelpers so the set automatically picks up
// any helper added to the runtime, eliminating the prior drift hazard.
//
// Built once at package init and shared across every parseTemplateAll call;
// text/template's Funcs() copies the map internally, so concurrent reuse is
// safe.
var stubFuncs = buildStubFuncs()

func buildStubFuncs() template.FuncMap {
	real := render.CreateTemplateHelpers(
		context.Background(),
		logging.Discard(),
		"",
		&options.BoilerplateOptions{NoHooks: true, NoShell: true},
		template.New(""),
	)

	out := make(template.FuncMap, len(real))
	for name := range real {
		out[name] = stubFunc
	}

	return out
}

// stubFunc is the no-op identifier used to register every helper at parse
// time. It accepts any number of arguments and returns a string. text/template
// only requires that an identifier exists when parsing; the actual signature
// is checked at execute time, which we never reach.
func stubFunc(_ ...any) string { return "" }

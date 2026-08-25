// Package resolver interpolates templated values in action context data using
// the trigger record of a workflow run.
//
// Handlers call Resolve before side effects (create/update record, trigger
// pipeline, …) so action configs can reference trigger data without embedding
// it at authoring time.
//
// # Template language
//
// String values may embed Go text/templates (https://pkg.go.dev/text/template).
// text/template is used rather than html/template so interpolated values are
// not HTML-escaped; action payloads are JSON data, not HTML.
//
// The root data object is env: the trigger record is .Record and pipeline
// level data is .Input. Numeric pipeline indexes are written as
// {{ .Input.1.body.name }} and rewritten to index calls because Go templates
// cannot parse ".1" as a field name. The only custom function today is now,
// which returns the current UTC time in RFC 3339:
//
//	{"email": "{{ .Record.email }}", "createdAt": "{{ now }}", "note": "hi {{ .Record.email }}"}
//
// Built-in template functions such as or work as usual, e.g.
// {{ or .Record.nickname "friend" }}.
//
// # Typed vs string results
//
// text/template always writes strings. That is fine for mixed interpolation
// ("Hello {{ .Record.email }}"), but action fields that should stay numbers,
// booleans, objects, or arrays would otherwise become strings and fail schema
// validation after JSON marshaling.
//
// So when a string is exactly one field path — "{{ .Record.age }}" or even
// "{{ . }}" — Resolve looks the path up with reflect and returns the native
// value. Any other template (functions, pipelines, surrounding text) executes
// normally and yields a string.
//
// Missing keys inside .Record (a map) resolve to nil rather than erroring;
// schema validation downstream rejects them where they are required, and or
// provides defaults. Missing fields on the env struct itself (typos like
// .Contxt) still error, matching text/template's struct behavior.
package resolver

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"
	"time"
)

// nowFunc returns the current time; a package variable so tests can pin the
// clock without depending on the real wall clock.
var nowFunc = time.Now

// tmplCache holds parsed templates keyed by source string. Parsing is the
// expensive part; *template.Template is safe for concurrent Execute after
// Parse. LoadOrStore avoids duplicate work if two goroutines parse the same
// source for the first time.
var tmplCache sync.Map // map[string]*template.Template

// env is the root data passed to every template. Exported fields become the
// top-level names templates can reference (.Record, .Input). Keeping a struct
// (rather than passing the record map as ".") leaves room to add more roots
// later without breaking existing .Record.* templates.
type env struct {
	Record map[string]any
	Input  map[string]any
}

// Resolve returns a shallow-copied tree of data where every templated string
// has been replaced by its resolved value. Nested maps and slices are walked
// recursively; non-string values and strings without "{{" pass through
// unchanged. The input maps/slices are not mutated.
//
// record is the workflow trigger's data document. A nil record is treated as
// an empty map so templates that only use now still work.
func Resolve(data map[string]any, record map[string]any) (map[string]any, error) {
	if record == nil {
		record = map[string]any{}
	}
	resolved, err := resolveValue(data, env{Record: record, Input: map[string]any{}})
	if err != nil {
		return nil, err
	}
	// resolveValue preserves map[string]any for map inputs; the assertion is
	// safe for the top-level call.
	return resolved.(map[string]any), nil
}

// ResolveInput is like Resolve but templates read from .Input (pipeline level
// data) instead of .Record.
func ResolveInput(data map[string]any, input map[string]any) (map[string]any, error) {
	if input == nil {
		input = map[string]any{}
	}
	resolved, err := resolveValue(data, env{Record: map[string]any{}, Input: input})
	if err != nil {
		return nil, err
	}
	return resolved.(map[string]any), nil
}

// ResolveString interpolates a single templated string against .Input.
func ResolveString(s string, input map[string]any) (any, error) {
	if input == nil {
		input = map[string]any{}
	}
	return resolveString(s, env{Record: map[string]any{}, Input: input})
}

// resolveValue dispatches on the JSON-shaped types that appear in action
// context after encoding/json unmarshal: string, map[string]any, []any, plus
// scalars that need no work. Errors are wrapped with the map key or slice
// index so callers can see which field failed.
func resolveValue(v any, e env) (any, error) {
	switch v := v.(type) {
	case string:
		return resolveString(v, e)
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, elem := range v {
			rv, err := resolveValue(elem, e)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", k, err)
			}
			out[k] = rv
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, elem := range v {
			rv, err := resolveValue(elem, e)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			out[i] = rv
		}
		return out, nil
	default:
		return v, nil
	}
}

// resolveString evaluates one string field. Fast path: no "{{" means the
// string is a literal. Otherwise parse (or reuse a cached parse), then either
// typed field lookup or full template execution — see package docs.
func resolveString(s string, e env) (any, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}
	rewritten := rewriteInputIndexes(s)
	tmpl, err := parseTemplate(rewritten)
	if err != nil {
		return nil, err
	}
	// Pure field paths bypass Execute so the result keeps its Go type.
	if path, ok := pureFieldPath(tmpl); ok {
		v, err := lookup(e, path)
		if err != nil {
			return nil, fmt.Errorf("executing template %q: %w", s, err)
		}
		return v, nil
	}
	if path, ok := pureInputIndexPath(tmpl); ok {
		v, err := lookup(e, path)
		if err != nil {
			return nil, fmt.Errorf("executing template %q: %w", s, err)
		}
		return v, nil
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, e); err != nil {
		return nil, fmt.Errorf("executing template %q: %w", s, err)
	}
	return b.String(), nil
}

// inputIndexPattern matches ".Input.1" or ".Input.1.body.name" so they can be
// rewritten to index calls. Go text/template rejects ".1" as a field name.
var inputIndexPattern = regexp.MustCompile(`\.Input\.(\d+)((?:\.[A-Za-z_][A-Za-z0-9_]*)*)`)

func rewriteInputIndexes(s string) string {
	return inputIndexPattern.ReplaceAllStringFunc(s, func(match string) string {
		sub := inputIndexPattern.FindStringSubmatch(match)
		args := []string{fmt.Sprintf("%q", sub[1])}
		if sub[2] != "" {
			for _, part := range strings.Split(strings.TrimPrefix(sub[2], "."), ".") {
				args = append(args, fmt.Sprintf("%q", part))
			}
		}
		return "(index .Input " + strings.Join(args, " ") + ")"
	})
}

// parseTemplate returns a cached template for s, or parses and stores one.
// missingkey=zero makes missing map keys evaluate to the zero value (nil for
// any) instead of aborting — matching Resolve's "missing record fields are
// nil" contract. Struct field typos still error inside Execute.
func parseTemplate(s string) (*template.Template, error) {
	if v, ok := tmplCache.Load(s); ok {
		return v.(*template.Template), nil
	}
	tmpl, err := template.New("").
		Option("missingkey=zero").
		Funcs(template.FuncMap{
			"now": func() string {
				return nowFunc().UTC().Format(time.RFC3339)
			},
		}).
		Parse(s)
	if err != nil {
		return nil, fmt.Errorf("parsing template %q: %w", s, err)
	}
	actual, _ := tmplCache.LoadOrStore(s, tmpl)
	return actual.(*template.Template), nil
}

// pureFieldPath reports whether tmpl is exactly one field selection on the
// root data — "{{ .Record.email }}", "{{ .Record }}", or "{{ . }}" — with no
// surrounding text, function calls, pipelines, or declarations.
//
// It inspects the parse tree rather than matching strings so the check stays
// correct for any field path shape text/template accepts, without hardcoding
// names like Record.
//
// On success, the returned path is the FieldNode identifiers (nil means the
// whole root, i.e. "{{ . }}").
func pureFieldPath(tmpl *template.Template) ([]string, bool) {
	nodes := tmpl.Tree.Root.Nodes
	// Mixed text + action produces multiple root nodes (TextNode + ActionNode).
	if len(nodes) != 1 {
		return nil, false
	}
	action, ok := nodes[0].(*parse.ActionNode)
	// Reject pipelines with decls ({{ $x := ... }}), multi-command pipes
	// ({{ .A | printf }}), and non-action nodes.
	if !ok || action.Pipe == nil || len(action.Pipe.Decl) != 0 || len(action.Pipe.Cmds) != 1 {
		return nil, false
	}
	cmd := action.Pipe.Cmds[0]
	// A single arg is a bare field/dot; multiple args means a function call
	// such as {{ or .Record.nickname "friend" }} or {{ now }}.
	if len(cmd.Args) != 1 {
		return nil, false
	}
	switch arg := cmd.Args[0].(type) {
	case *parse.FieldNode:
		// Ident is already split: ".Record.address.city" → ["Record","address","city"].
		return arg.Ident, true
	case *parse.DotNode:
		return nil, true
	default:
		return nil, false
	}
}

// pureInputIndexPath reports whether tmpl is exactly one rewritten
// "{{ (index .Input "1" "body" "name") }}" call, which is how ".Input.1.body.name"
// is expressed after rewriteInputIndexes. The returned path is
// ["Input","1","body","name"] for lookup.
func pureInputIndexPath(tmpl *template.Template) ([]string, bool) {
	nodes := tmpl.Tree.Root.Nodes
	if len(nodes) != 1 {
		return nil, false
	}
	action, ok := nodes[0].(*parse.ActionNode)
	if !ok || action.Pipe == nil || len(action.Pipe.Decl) != 0 || len(action.Pipe.Cmds) != 1 {
		return nil, false
	}
	cmd := action.Pipe.Cmds[0]
	if len(cmd.Args) < 3 {
		return nil, false
	}
	ident, ok := cmd.Args[0].(*parse.IdentifierNode)
	if !ok || ident.Ident != "index" {
		return nil, false
	}
	field, ok := cmd.Args[1].(*parse.FieldNode)
	if !ok || len(field.Ident) != 1 || field.Ident[0] != "Input" {
		return nil, false
	}
	path := []string{"Input"}
	for _, arg := range cmd.Args[2:] {
		str, ok := arg.(*parse.StringNode)
		if !ok {
			return nil, false
		}
		path = append(path, str.Text)
	}
	return path, true
}

// lookup walks path from data using the same rules templates use for field
// selection: exported struct fields by name, string-keyed map entries by key.
// Interfaces and pointers are dereferenced at each step.
//
// Missing map keys return (nil, nil) — the zero value — so "{{ .Record.missing }}"
// yields JSON null. Missing struct fields return an error, so typos on env
// ({{ .Contxt }}) fail the same way Execute would.
//
// An empty path returns data itself (the "{{ . }}" case).
func lookup(data any, path []string) (any, error) {
	cur := reflect.ValueOf(data)
	for _, key := range path {
		for cur.Kind() == reflect.Interface || cur.Kind() == reflect.Pointer {
			if cur.IsNil() {
				return nil, nil
			}
			cur = cur.Elem()
		}
		switch cur.Kind() {
		case reflect.Map:
			if cur.Type().Key().Kind() != reflect.String {
				return nil, nil
			}
			cur = cur.MapIndex(reflect.ValueOf(key))
			if !cur.IsValid() {
				return nil, nil
			}
		case reflect.Struct:
			f := cur.FieldByName(key)
			if !f.IsValid() {
				return nil, fmt.Errorf("can't evaluate field %s in type %s", key, cur.Type())
			}
			cur = f
		default:
			// Intermediate value is a scalar/slice/etc.; further selection is
			// undefined, treat as missing.
			return nil, nil
		}
	}
	if !cur.IsValid() {
		return nil, nil
	}
	if (cur.Kind() == reflect.Interface || cur.Kind() == reflect.Pointer) && cur.IsNil() {
		return nil, nil
	}
	return cur.Interface(), nil
}

package resolver

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"text/template"
	"text/template/parse"
	"time"

	"github.com/lutia-io/huma/pkg/uuid"
)

// typedFuncs are custom functions whose result should keep its Go type when
// the template is exactly one call — "{{ add 1 1 }}" must stay a number so
// integer/number schema fields validate. Mixed text still stringifies.
var typedFuncs = map[string]func(...any) (any, error){
	"add": add,
}

// uuidFunc returns a UUID v4; a package variable so tests can pin the value.
var uuidFunc = uuid.New

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"now": func() string {
			return nowFunc().UTC().Format(time.RFC3339)
		},
		"uuid": func() (string, error) {
			return uuidFunc()
		},
		"add": add,
	}
}

// add sums two or more numbers. Whole sums return int64 so JSON integer
// fields accept the value; otherwise the result is float64.
func add(values ...any) (any, error) {
	if len(values) < 2 {
		return nil, fmt.Errorf("add requires at least 2 arguments")
	}
	var sum float64
	allWhole := true
	for i, v := range values {
		n, whole, err := toFloat(v)
		if err != nil {
			return nil, fmt.Errorf("add argument %d: %w", i+1, err)
		}
		if !whole {
			allWhole = false
		}
		sum += n
	}
	if allWhole && sum >= math.MinInt64 && sum <= math.MaxInt64 {
		return int64(sum), nil
	}
	return sum, nil
}

func toFloat(v any) (float64, bool, error) {
	switch n := v.(type) {
	case nil:
		return 0, false, fmt.Errorf("not a number")
	case int:
		return float64(n), true, nil
	case int8:
		return float64(n), true, nil
	case int16:
		return float64(n), true, nil
	case int32:
		return float64(n), true, nil
	case int64:
		return float64(n), true, nil
	case uint:
		return float64(n), true, nil
	case uint8:
		return float64(n), true, nil
	case uint16:
		return float64(n), true, nil
	case uint32:
		return float64(n), true, nil
	case uint64:
		return float64(n), true, nil
	case float32:
		f := float64(n)
		return f, isWhole(f), nil
	case float64:
		return n, isWhole(n), nil
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return float64(i), true, nil
		}
		f, err := n.Float64()
		if err != nil {
			return 0, false, fmt.Errorf("not a number")
		}
		return f, isWhole(f), nil
	case string:
		if i, err := strconv.ParseInt(n, 10, 64); err == nil {
			return float64(i), true, nil
		}
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false, fmt.Errorf("not a number")
		}
		return f, isWhole(f), nil
	default:
		return 0, false, fmt.Errorf("not a number")
	}
}

func isWhole(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0) && f == math.Trunc(f)
}

// pureTypedCall reports whether tmpl is exactly one call to a typed function,
// with no surrounding text, pipelines, or declarations.
func pureTypedCall(tmpl *template.Template) (string, []parse.Node, bool) {
	nodes := tmpl.Tree.Root.Nodes
	if len(nodes) != 1 {
		return "", nil, false
	}
	action, ok := nodes[0].(*parse.ActionNode)
	if !ok || action.Pipe == nil || len(action.Pipe.Decl) != 0 || len(action.Pipe.Cmds) != 1 {
		return "", nil, false
	}
	cmd := action.Pipe.Cmds[0]
	if len(cmd.Args) == 0 {
		return "", nil, false
	}
	ident, ok := cmd.Args[0].(*parse.IdentifierNode)
	if !ok {
		return "", nil, false
	}
	if _, ok := typedFuncs[ident.Ident]; !ok {
		return "", nil, false
	}
	return ident.Ident, cmd.Args[1:], true
}

func evalTypedCall(name string, args []parse.Node, e env) (any, error) {
	fn, ok := typedFuncs[name]
	if !ok {
		return nil, fmt.Errorf("unknown function %q", name)
	}
	values, err := evalArgs(args, e)
	if err != nil {
		return nil, err
	}
	return fn(values...)
}

func evalArgs(args []parse.Node, e env) ([]any, error) {
	out := make([]any, len(args))
	for i, arg := range args {
		v, err := evalArg(arg, e)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func evalArg(n parse.Node, e env) (any, error) {
	switch n := n.(type) {
	case *parse.NumberNode:
		if n.IsInt {
			return n.Int64, nil
		}
		if n.IsUint {
			return n.Uint64, nil
		}
		return n.Float64, nil
	case *parse.StringNode:
		return n.Text, nil
	case *parse.FieldNode:
		return lookup(e, n.Ident)
	case *parse.DotNode:
		return lookup(e, nil)
	case *parse.PipeNode:
		if n == nil || len(n.Decl) != 0 || len(n.Cmds) != 1 {
			return nil, fmt.Errorf("unsupported template argument")
		}
		return evalCmd(n.Cmds[0], e)
	default:
		return nil, fmt.Errorf("unsupported template argument")
	}
}

func evalCmd(cmd *parse.CommandNode, e env) (any, error) {
	if cmd == nil || len(cmd.Args) == 0 {
		return nil, fmt.Errorf("unsupported template argument")
	}
	switch arg := cmd.Args[0].(type) {
	case *parse.IdentifierNode:
		if arg.Ident == "index" {
			return evalIndex(cmd.Args[1:], e)
		}
		if _, ok := typedFuncs[arg.Ident]; ok {
			return evalTypedCall(arg.Ident, cmd.Args[1:], e)
		}
		return nil, fmt.Errorf("unsupported function %q", arg.Ident)
	case *parse.FieldNode:
		return lookup(e, arg.Ident)
	default:
		return evalArg(arg, e)
	}
}

func evalIndex(args []parse.Node, e env) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("index requires a collection and a key")
	}
	field, ok := args[0].(*parse.FieldNode)
	if !ok {
		return nil, fmt.Errorf("unsupported index target")
	}
	path := append([]string{}, field.Ident...)
	for _, arg := range args[1:] {
		switch a := arg.(type) {
		case *parse.StringNode:
			path = append(path, a.Text)
		case *parse.NumberNode:
			path = append(path, a.Text)
		default:
			v, err := evalArg(a, e)
			if err != nil {
				return nil, err
			}
			path = append(path, fmt.Sprint(v))
		}
	}
	return lookup(e, path)
}

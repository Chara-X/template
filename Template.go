package template

import (
	"fmt"
	"io"
	"reflect"
	"text/template/parse"
)

type Template struct {
	tree  *parse.Tree
	trees map[string]*parse.Tree
	funcs map[string]reflect.Value
}

func New(name, text string, funcs map[string]any) *Template {
	var t = &Template{}
	t.trees = map[string]*parse.Tree{}
	t.tree, _ = parse.New(name).Parse(text, "", "", t.trees, funcs)
	t.funcs = map[string]reflect.Value{}
	for name, fn := range funcs {
		t.funcs[name] = reflect.ValueOf(fn)
	}
	return t
}
func (t *Template) Execute(w io.Writer, data any) { t.execute(t.tree.Root, w, reflect.ValueOf(data)) }
func (t *Template) execute(node parse.Node, w io.Writer, data reflect.Value) {
	switch node := node.(type) {
	case *parse.CommentNode:
	case *parse.IfNode:
		if t.eval(node.Pipe, data).Bool() {
			t.execute(node.List, w, data)
		} else if node.ElseList != nil {
			t.execute(node.ElseList, w, data)
		}
	case *parse.RangeNode:
		var iter = func(elem reflect.Value) {
			defer func() {
				if err := recover(); err != nil && err != errContinue {
					panic(err)
				}
			}()
			t.execute(node.List, w, elem)
		}
		defer func() {
			if err := recover(); err != nil && err != errBreak {
				panic(err)
			}
		}()
		switch arg := t.eval(node.Pipe, data); arg.Kind() {
		case reflect.Array, reflect.Slice:
			for i := 0; i < arg.Len(); i++ {
				iter(arg.Index(i))
			}
		default:
			panic("not implemented")
		}
	case *parse.ContinueNode:
		panic(errContinue)
	case *parse.BreakNode:
		panic(errBreak)
	case *parse.ListNode:
		for _, node := range node.Nodes {
			t.execute(node, w, data)
		}
	case *parse.TemplateNode:
		(&Template{t.trees[node.Name], t.trees, t.funcs}).Execute(w, t.eval(node.Pipe, data))
	case *parse.ActionNode:
		fmt.Fprint(w, t.eval(node.Pipe.Cmds[0], data).Interface())
	case *parse.TextNode:
		w.Write([]byte(node.Text))
	default:
		panic(node.Type())
	}
}
func (t *Template) eval(node parse.Node, data reflect.Value) reflect.Value {
	switch node := node.(type) {
	case *parse.PipeNode:
		return t.eval(node.Cmds[0], data)
	case *parse.CommandNode:
		switch arg := node.Args[0].(type) {
		case *parse.IdentifierNode:
			var args = []reflect.Value{}
			for _, arg := range node.Args[1:] {
				args = append(args, t.eval(arg, data))
			}
			return t.funcs[arg.Ident].Call(args)[0]
		default:
			return t.eval(arg, data)
		}
	case *parse.FieldNode:
		var field = data.FieldByName(node.Ident[0])
		for _, ident := range node.Ident[1:] {
			field = field.FieldByName(ident)
		}
		return field
	case *parse.NumberNode:
		switch {
		case node.IsFloat:
			return reflect.ValueOf(node.Float64)
		case node.IsInt:
			return reflect.ValueOf(int(node.Int64))
		}
	case *parse.BoolNode:
		return reflect.ValueOf(node.True)
	case *parse.StringNode:
		return reflect.ValueOf(node.Text)
	case *parse.DotNode:
		return data
	}
	panic(node.Type())
}

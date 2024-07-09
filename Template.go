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
		if t.evalPipe(node.Pipe, data).Bool() {
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
		switch arg := t.evalPipe(node.Pipe, data); arg.Kind() {
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
		(&Template{t.trees[node.Name], t.trees, t.funcs}).Execute(w, t.evalPipe(node.Pipe, data))
	case *parse.ActionNode:
		fmt.Fprint(w, t.evalCommand(node.Pipe.Cmds[0], data).Interface())
	case *parse.TextNode:
		w.Write([]byte(node.Text))
	default:
		panic(node.Type())
	}
}
func (t *Template) evalPipe(pipe *parse.PipeNode, data reflect.Value) reflect.Value {
	return t.evalCommand(pipe.Cmds[0], data)
}
func (t *Template) evalCommand(cmd *parse.CommandNode, data reflect.Value) reflect.Value {
	switch arg := cmd.Args[0].(type) {
	case *parse.IdentifierNode:
		var args = make([]reflect.Value, len(cmd.Args)-1)
		for i, arg := range cmd.Args[1:] {
			args[i] = t.evalCommand(arg.(*parse.CommandNode), data)
		}
		return t.funcs[arg.Ident].Call(args)[0]
	case *parse.NumberNode:
		switch {
		case arg.IsFloat:
			return reflect.ValueOf(arg.Float64)
		case arg.IsInt:
			return reflect.ValueOf(int(arg.Int64))
		}
	case *parse.BoolNode:
		return reflect.ValueOf(arg.True)
	case *parse.StringNode:
		return reflect.ValueOf(arg.Text)
	case *parse.DotNode:
		return data
	}
	panic(cmd.Args[0].Type())
}

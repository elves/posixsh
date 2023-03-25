package parse

import (
	"bytes"
	"fmt"
	"reflect"
)

func PprintAST(n Node) string {
	var b bytes.Buffer
	pprintAST(&b, "", toAST(n))
	return b.String()
}

// An intermediate representation for nodes, keeping information relevant in the
// AST.
type ast struct {
	name   string
	fields []*astField
}

type astField struct {
	name   string
	scalar interface{}
	node   *ast
	nodes  []*ast
}

func newAST(name string) *ast {
	return &ast{name: name}
}

var nodeTyp = reflect.TypeOf((*Node)(nil)).Elem()

func toAST(n Node) *ast {
	if n == nil || reflect.ValueOf(n).IsNil() {
		return nil
	}

	nVal := reflect.ValueOf(n).Elem()
	nTyp := nVal.Type()
	a := newAST(nTyp.Name())

	for i := 0; i < nVal.NumField(); i++ {
		if nTyp.Field(i).PkgPath != "" {
			// Skip unexported fields
			continue
		}

		f := &astField{name: nTyp.Field(i).Name}

		fieldTyp := nTyp.Field(i).Type
		fieldVal := nVal.Field(i)
		field := fieldVal.Interface()

		if child, ok := field.(Node); ok {
			f.node = toAST(child)
		} else if fieldTyp.Kind() == reflect.Slice && fieldTyp.Elem().AssignableTo(nodeTyp) {
			// []T where T < Node
			nodes := make([]*ast, fieldVal.Len())
			for j := 0; j < fieldVal.Len(); j++ {
				nodes[j] = toAST(fieldVal.Index(j).Interface().(Node))
			}
			f.nodes = nodes
		} else {
			f.scalar = field
		}

		a.fields = append(a.fields, f)
	}
	return a
}

func pprintAST(buf *bytes.Buffer, indent string, a *ast) {
	if a == nil {
		buf.WriteString("nil")
		return
	}

	buf.WriteString(a.name)

	indent1 := indent + "  "
	indent2 := indent1 + "  "

	for _, f := range a.fields {
		buf.WriteString("\n" + indent1 + "." + f.name + " = ")
		switch {
		case f.scalar != nil:
			if s, ok := f.scalar.(string); ok {
				fmt.Fprintf(buf, "%q", s)
			} else {
				fmt.Fprint(buf, f.scalar)
			}
		case f.node != nil:
			pprintAST(buf, indent1, f.node)
		case f.nodes != nil:
			for _, node := range f.nodes {
				buf.WriteString("\n" + indent2)
				pprintAST(buf, indent2, node)
			}
		default:
			buf.WriteString("nil")
		}
	}
}

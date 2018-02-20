package parse

import (
	"bytes"
	"fmt"
	"reflect"
)

func PprintAST(n Node) string {
	var b bytes.Buffer
	pprintAST(&b, "", n)
	return b.String()
}

func pprintAST(buf *bytes.Buffer, indent string, n Node) {
	if n == nil || reflect.ValueOf(n).IsNil() {
		buf.WriteString("nil")
		return
	}

	indent1 := indent + "  "
	indent2 := indent1 + "  "
	nVal := reflect.ValueOf(n).Elem()
	nTyp := nVal.Type()

	buf.WriteString(nTyp.Name() + "{")

	writtenField := false
	for i := 0; i < nVal.NumField(); i++ {
		if nTyp.Field(i).PkgPath != "" {
			// Skip unexported fields
			continue
		}
		// fmt.Println("field", nTyp.Field(i).Name)
		buf.WriteString("\n" + indent1 + nTyp.Field(i).Name + ": ")
		writtenField = true

		fieldTyp := nTyp.Field(i).Type
		fieldVal := nVal.Field(i)
		field := fieldVal.Interface()

		switch field := field.(type) {
		case Node:
			pprintAST(buf, indent1, field)
		case string:
			fmt.Fprintf(buf, "%q", field)
		default:
			if fieldTyp.Kind() == reflect.Slice && fieldTyp.Elem().AssignableTo(nodeTyp) {
				// []T, where T.AssignableTo(Node)
				buf.WriteRune('[')
				for j := 0; j < fieldVal.Len(); j++ {
					buf.WriteString("\n" + indent2)
					pprintAST(buf, indent2, fieldVal.Index(j).Interface().(Node))
				}
				if fieldVal.Len() > 0 {
					buf.WriteString("\n" + indent1)
				}
				buf.WriteRune(']')
			} else {
				fmt.Fprint(buf, field)
			}
		}
	}
	if writtenField {
		buf.WriteString("\n" + indent)
	}
	buf.WriteRune('}')
}

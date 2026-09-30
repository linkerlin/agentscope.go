package main

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
)

// funcSig renders a FuncDecl's signature (parameters + results), receiver
// excluded — enough fidelity for API-compat diffing.
func funcSig(fn *ast.FuncDecl) string {
	var buf bytes.Buffer
	buf.WriteByte('(')
	printCommaList(&buf, fn.Type.Params)
	buf.WriteString(") ")
	if fn.Type.Results != nil {
		if len(fn.Type.Results.List) > 1 || (len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) > 0) {
			buf.WriteByte('(')
			printCommaList(&buf, fn.Type.Results)
			buf.WriteByte(')')
		} else {
			printCommaList(&buf, fn.Type.Results)
		}
	}
	return buf.String()
}

func printCommaList(buf *bytes.Buffer, fl *ast.FieldList) {
	if fl == nil {
		return
	}
	for i, f := range fl.List {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(exprString(f.Type))
	}
}

func exprString(e ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), e)
	return buf.String()
}

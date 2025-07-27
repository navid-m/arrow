package building

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/navid-m/arrow/parsing"
)

// Build a comprehensive function signature
func BuildFunctionSignature(d *ast.FuncDecl, params, results string) string {
	var sig strings.Builder

	sig.WriteString("func ")

	if d.Recv != nil {
		sig.WriteString("(")
		sig.WriteString(parsing.ExtractFieldList(d.Recv))
		sig.WriteString(") ")
	}

	sig.WriteString(d.Name.Name)
	sig.WriteString("(")
	sig.WriteString(params)
	sig.WriteString(")")

	if results != "" {
		if strings.Contains(results, ",") || strings.Contains(results, " ") {
			sig.WriteString(" (")
			sig.WriteString(results)
			sig.WriteString(")")
		} else {
			sig.WriteString(" ")
			sig.WriteString(results)
		}
	}

	return sig.String()
}

// Build variable/constant declaration
func BuildVariableDeclaration(tok token.Token, name, typeStr string, values []string, index int) string {
	var decl strings.Builder

	decl.WriteString(strings.ToLower(tok.String()))
	decl.WriteString(" ")
	decl.WriteString(name)

	if typeStr != "" {
		decl.WriteString(" ")
		decl.WriteString(typeStr)
	}

	if len(values) > index && values[index] != "" {
		decl.WriteString(" = ")
		decl.WriteString(values[index])
	}

	return decl.String()
}

package parsing

import (
	"fmt"
	"go/ast"
	"strings"
)

// Extract fields from the function signature
func ExtractFieldList(fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	var parts []string
	for _, field := range fl.List {
		typeStr := ExprToString(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, typeStr)
		} else {
			for _, name := range field.Names {
				parts = append(parts, name.Name+" "+typeStr)
			}
		}
	}
	return strings.Join(parts, ", ")
}

// Extract struct fields
func ExtractStructFields(structType *ast.StructType) string {
	if structType.Fields == nil {
		return ""
	}
	var fields []string
	for _, field := range structType.Fields.List {
		typeStr := ExprToString(field.Type)
		if len(field.Names) == 0 {
			fields = append(fields, typeStr)
		} else {
			for _, name := range field.Names {
				fieldStr := name.Name + " " + typeStr
				if field.Tag != nil {
					fieldStr += " " + field.Tag.Value
				}
				fields = append(fields, fieldStr)
			}
		}
	}
	return strings.Join(fields, "\n")
}

// Extract interface methods
func ExtractInterfaceMethods(interfaceType *ast.InterfaceType) string {
	if interfaceType.Methods == nil {
		return ""
	}

	var methods []string
	for _, method := range interfaceType.Methods.List {
		if len(method.Names) == 0 {
			methods = append(methods, ExprToString(method.Type))
		} else {
			for _, name := range method.Names {
				if funcType, ok := method.Type.(*ast.FuncType); ok {
					var (
						params  = ExtractFieldList(funcType.Params)
						results = ExtractFieldList(funcType.Results)
						sig     = name.Name + "(" + params + ")"
					)
					if results != "" {
						if strings.Contains(results, ",") {
							sig += " (" + results + ")"
						} else {
							sig += " " + results
						}
					}
					methods = append(methods, sig)
				}
			}
		}
	}

	return strings.Join(methods, "\n")
}

// Extract documentation with formatting
func ExtractDocumentation(docGroup *ast.CommentGroup) string {
	if docGroup == nil {
		return ""
	}

	var lines []string
	for _, comment := range docGroup.List {
		line := strings.TrimPrefix(comment.Text, "//")
		line = strings.TrimPrefix(line, "/*")
		line = strings.TrimSuffix(line, "*/")
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// Convert expression to string with type handling
func ExprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return ExprToString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + ExprToString(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + ExprToString(e.Elt)
		}
		return "[" + ExprToString(e.Len) + "]" + ExprToString(e.Elt)
	case *ast.Ellipsis:
		return "..." + ExprToString(e.Elt)
	case *ast.FuncType:
		params := ExtractFieldList(e.Params)
		results := ExtractFieldList(e.Results)
		sig := "func(" + params + ")"
		if results != "" {
			if strings.Contains(results, ",") {
				sig += " (" + results + ")"
			} else {
				sig += " " + results
			}
		}
		return sig
	case *ast.MapType:
		return "map[" + ExprToString(e.Key) + "]" + ExprToString(e.Value)
	case *ast.ChanType:
		dir := ""
		switch e.Dir {
		case ast.SEND:
			dir = "chan<- "
		case ast.RECV:
			dir = "<-chan "
		default:
			dir = "chan "
		}
		return dir + ExprToString(e.Value)
	case *ast.InterfaceType:
		if e.Methods == nil || len(e.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.StructType:
		return "struct{...}"
	case *ast.BasicLit:
		return e.Value
	case *ast.CompositeLit:
		return ExprToString(e.Type) + "{...}"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

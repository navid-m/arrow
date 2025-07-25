package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/navid-m/arrow/models"
)

// Render the documentation, return a slice of index entries.
func renderDocs(
	docFileName string,
	relPath string,
	pkgs map[string]*ast.Package,
	docDir string,
	srcPath string,
) []models.IndexEntry {
	var (
		docFile    = fmt.Sprintf("%s-docs.html", docFileName)
		indexEntry = models.IndexEntry{
			PackageName: relPath,
			DocFile:     docFile,
		}
	)

	for pkgName, pkg := range pkgs {
		pageData := models.PageData{PackageName: pkgName}
		subItems, err := os.ReadDir(filepath.Join(srcPath, relPath))
		if err == nil {
			for _, item := range subItems {
				if item.IsDir() && !strings.HasPrefix(item.Name(), ".") {
					subPkgPath := filepath.Join(relPath, item.Name())
					goFiles, _ := filepath.Glob(filepath.Join(srcPath, subPkgPath, "*.go"))
					var nonTestFiles []string
					for _, file := range goFiles {
						if !strings.HasSuffix(filepath.Base(file), "_test.go") {
							nonTestFiles = append(nonTestFiles, file)
						}
					}
					if len(nonTestFiles) > 0 {
						subDocFile := strings.ReplaceAll(subPkgPath, string(filepath.Separator), "_") + "-docs.html"
						pageData.SubPackages = append(pageData.SubPackages, models.IndexEntry{
							PackageName: item.Name(),
							DocFile:     subDocFile,
						})
					}
				}
			}
		}

		for fileName, file := range pkg.Files {
			if strings.HasSuffix(fileName, "_test.go") {
				continue
			}

			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if strings.HasPrefix(d.Name.Name, "Test") ||
						strings.HasPrefix(d.Name.Name, "Benchmark") ||
						strings.HasPrefix(d.Name.Name, "Example") {
						continue
					}

					params := extractFieldList(d.Type.Params)
					results := extractFieldList(d.Type.Results)
					fullSig := buildFunctionSignature(d, params, results)
					doc := extractDocumentation(d.Doc)

					var receiver string
					if d.Recv != nil {
						receiver = extractFieldList(d.Recv)
					}

					pageData.Functions = append(pageData.Functions, models.Function{
						Name:     d.Name.Name,
						Params:   params,
						Results:  results,
						FullSig:  fullSig,
						Doc:      doc,
						Receiver: receiver,
						IsMethod: d.Recv != nil,
					})

				case *ast.GenDecl:
					switch d.Tok {
					case token.TYPE:
						for _, spec := range d.Specs {
							typeSpec, ok := spec.(*ast.TypeSpec)
							if !ok {
								continue
							}

							doc := extractDocumentation(d.Doc)
							if typeSpec.Doc != nil {
								doc = extractDocumentation(typeSpec.Doc)
							}

							switch t := typeSpec.Type.(type) {
							case *ast.StructType:
								pageData.Structs = append(pageData.Structs, models.Struct{
									Name:   typeSpec.Name.Name,
									Fields: extractStructFields(t),
									Doc:    doc,
									Kind:   "struct",
								})

							case *ast.InterfaceType:
								pageData.Interfaces = append(pageData.Interfaces, models.Interface{
									Name:    typeSpec.Name.Name,
									Methods: extractInterfaceMethods(t),
									Doc:     doc,
								})

							default:
								typeStr := exprToString(typeSpec.Type)
								pageData.Types = append(pageData.Types, models.TypeAlias{
									Name: typeSpec.Name.Name,
									Type: typeStr,
									Doc:  doc,
								})
							}
						}

					case token.VAR, token.CONST:
						for _, spec := range d.Specs {
							valueSpec, ok := spec.(*ast.ValueSpec)
							if !ok {
								continue
							}

							doc := extractDocumentation(d.Doc)
							if valueSpec.Doc != nil {
								doc = extractDocumentation(valueSpec.Doc)
							}

							typeStr := ""
							if valueSpec.Type != nil {
								typeStr = exprToString(valueSpec.Type)
							}

							var values []string
							for _, val := range valueSpec.Values {
								values = append(values, exprToString(val))
							}

							for i, name := range valueSpec.Names {
								decl := buildVariableDeclaration(d.Tok, name.Name, typeStr, values, i)

								global := models.Global{
									Name:        name.Name,
									Declaration: decl,
									Doc:         doc,
									Kind:        strings.ToLower(d.Tok.String()),
								}

								pageData.Globals = append(pageData.Globals, global)
							}
						}

					case token.IMPORT:
						for _, spec := range d.Specs {
							importSpec, ok := spec.(*ast.ImportSpec)
							if !ok {
								continue
							}

							path := strings.Trim(importSpec.Path.Value, `"`)
							name := ""
							if importSpec.Name != nil {
								name = importSpec.Name.Name
							}

							pageData.Imports = append(pageData.Imports, models.Import{
								Name: name,
								Path: path,
							})
						}
					}
				}
			}
		}

		sort.Slice(pageData.Functions, func(i, j int) bool {
			return pageData.Functions[i].Name < pageData.Functions[j].Name
		})
		sort.Slice(pageData.Structs, func(i, j int) bool {
			return pageData.Structs[i].Name < pageData.Structs[j].Name
		})
		sort.Slice(pageData.Interfaces, func(i, j int) bool {
			return pageData.Interfaces[i].Name < pageData.Interfaces[j].Name
		})
		sort.Slice(pageData.Types, func(i, j int) bool {
			return pageData.Types[i].Name < pageData.Types[j].Name
		})
		sort.Slice(pageData.Globals, func(i, j int) bool {
			return pageData.Globals[i].Name < pageData.Globals[j].Name
		})

		outFile := filepath.Join(docDir, docFile)
		f, err := os.Create(outFile)
		if err != nil {
			fmt.Printf("Failed to create %s: %v\n", outFile, err)
			continue
		}
		defer f.Close()

		t := template.Must(template.New("doc").Parse(tmpl))
		if err := t.Execute(f, pageData); err != nil {
			fmt.Printf("Error executing template for %s: %v\n", outFile, err)
		}

		fmt.Printf("Generated %s\n", outFile)
	}
	return []models.IndexEntry{indexEntry}
}

// Extract fields from the function signature
func extractFieldList(fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	var parts []string
	for _, field := range fl.List {
		typeStr := exprToString(field.Type)
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

// Build a comprehensive function signature
func buildFunctionSignature(d *ast.FuncDecl, params, results string) string {
	var sig strings.Builder

	sig.WriteString("func ")

	if d.Recv != nil {
		sig.WriteString("(")
		sig.WriteString(extractFieldList(d.Recv))
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
func buildVariableDeclaration(tok token.Token, name, typeStr string, values []string, index int) string {
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

// Extract struct fields
func extractStructFields(structType *ast.StructType) string {
	if structType.Fields == nil {
		return ""
	}

	var fields []string
	for _, field := range structType.Fields.List {
		typeStr := exprToString(field.Type)
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
func extractInterfaceMethods(interfaceType *ast.InterfaceType) string {
	if interfaceType.Methods == nil {
		return ""
	}

	var methods []string
	for _, method := range interfaceType.Methods.List {
		if len(method.Names) == 0 {
			methods = append(methods, exprToString(method.Type))
		} else {
			for _, name := range method.Names {
				if funcType, ok := method.Type.(*ast.FuncType); ok {
					var (
						params  = extractFieldList(funcType.Params)
						results = extractFieldList(funcType.Results)
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
func extractDocumentation(docGroup *ast.CommentGroup) string {
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
func exprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToString(e.Elt)
		}
		return "[" + exprToString(e.Len) + "]" + exprToString(e.Elt)
	case *ast.Ellipsis:
		return "..." + exprToString(e.Elt)
	case *ast.FuncType:
		params := extractFieldList(e.Params)
		results := extractFieldList(e.Results)
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
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
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
		return dir + exprToString(e.Value)
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
		return exprToString(e.Type) + "{...}"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

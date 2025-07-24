package main

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type Function struct {
	Name    string
	Params  string
	Results string
	FullSig string
	Doc     string
}

type Struct struct {
	Name   string
	Fields string
	Doc    string
}

type PageData struct {
	PackageName string
	Functions   []Function
	Structs     []Struct
}

//go:embed views/template.html
var tmpl string

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: arrow <path-to-go-source>")
		return
	}

	srcPath := os.Args[1]
	fset := token.NewFileSet()

	// Walk all subdirectories
	err := filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only consider directories
		if !info.IsDir() {
			return nil
		}

		// Parse Go files in this directory
		pkgs, err := parser.ParseDir(fset, path, func(fi os.FileInfo) bool {
			// include only .go files (exclude test files if desired)
			return strings.HasSuffix(fi.Name(), ".go")
		}, parser.ParseComments)
		if err != nil {
			// skip directories that can't be parsed
			return nil
		}

		// No packages here
		if len(pkgs) == 0 {
			return nil
		}

		// Generate an HTML file for each package in this dir
		for pkgName, pkg := range pkgs {
			pageData := PageData{PackageName: pkgName}

			// Collect functions and structs
			for _, file := range pkg.Files {
				for _, decl := range file.Decls {
					switch d := decl.(type) {
					case *ast.FuncDecl:
						params := extractFieldList(d.Type.Params)
						results := extractFieldList(d.Type.Results)

						fullSig := fmt.Sprintf("func %s(%s)", d.Name.Name, params)
						if results != "" {
							if strings.Contains(results, ",") {
								fullSig += " (" + results + ")"
							} else {
								fullSig += " " + results
							}
						}

						doc := ""
						if d.Doc != nil {
							doc = strings.TrimSpace(d.Doc.Text())
						}

						pageData.Functions = append(pageData.Functions, Function{
							Name:    d.Name.Name,
							Params:  params,
							Results: results,
							FullSig: fullSig,
							Doc:     doc,
						})

					case *ast.GenDecl:
						if d.Tok == token.TYPE {
							for _, spec := range d.Specs {
								typeSpec, ok := spec.(*ast.TypeSpec)
								if !ok {
									continue
								}
								structType, ok := typeSpec.Type.(*ast.StructType)
								if !ok {
									continue
								}

								var fields []string
								for _, field := range structType.Fields.List {
									typeStr := exprToString(field.Type)
									if len(field.Names) == 0 {
										fields = append(fields, typeStr)
									} else {
										for _, name := range field.Names {
											fields = append(fields, name.Name+" "+typeStr)
										}
									}
								}

								doc := ""
								if d.Doc != nil {
									doc = strings.TrimSpace(d.Doc.Text())
								}

								pageData.Structs = append(pageData.Structs, Struct{
									Name:   typeSpec.Name.Name,
									Fields: strings.Join(fields, "\n"),
									Doc:    doc,
								})
							}
						}
					}
				}
			}

			// Create output file in the same directory
			outFile := filepath.Join(path, fmt.Sprintf("%s-docs.html", pkgName))
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

		return nil
	})

	if err != nil {
		panic(err)
	}
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

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.Ellipsis:
		return "..." + exprToString(e.Elt)
	case *ast.FuncType:
		return "func"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

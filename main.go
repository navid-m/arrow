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

type Global struct {
	Name        string
	Declaration string
	Doc         string
}

type PageData struct {
	PackageName string
	Functions   []Function
	Structs     []Struct
	Globals     []Global
}

type IndexEntry struct {
	PackageName string
	DocFile     string
}

//go:embed views/template.html
var tmpl string

//go:embed views/index.html
var indexTmpl string

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: arrow <path-to-go-source-directory>")
		return
	}

	var (
		srcPath = os.Args[1]
		fset    = token.NewFileSet()
		docDir  = filepath.Join(".", "docs")
	)

	if err := os.MkdirAll(docDir, 0755); err != nil {
		fmt.Printf("Failed to create docs directory: %v\n", err)
		return
	}

	var indexEntries []IndexEntry

	err := filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return err
		}

		pkgs, err := parser.ParseDir(fset, path, func(fi os.FileInfo) bool {
			return strings.HasSuffix(fi.Name(), ".go")
		}, parser.ParseComments)

		if err != nil || len(pkgs) == 0 {
			return nil
		}

		relPath, _ := filepath.Rel(srcPath, path)
		docFileName := strings.ReplaceAll(relPath, string(filepath.Separator), "_")
		if docFileName == "." || docFileName == "" {
			docFileName = "main"
		}

		docFile := fmt.Sprintf("%s-docs.html", docFileName)
		indexEntries = append(indexEntries, IndexEntry{
			PackageName: relPath,
			DocFile:     docFile,
		})

		for pkgName, pkg := range pkgs {
			pageData := PageData{PackageName: pkgName}

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
						switch d.Tok {
						case token.TYPE:
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

						case token.VAR, token.CONST:
							for _, spec := range d.Specs {
								valueSpec, ok := spec.(*ast.ValueSpec)
								if !ok {
									continue
								}

								doc := ""
								if valueSpec.Doc != nil {
									doc = strings.TrimSpace(valueSpec.Doc.Text())
								} else if d.Doc != nil {
									doc = strings.TrimSpace(d.Doc.Text())
								}

								typeStr := ""
								if valueSpec.Type != nil {
									typeStr = exprToString(valueSpec.Type)
								}

								for _, name := range valueSpec.Names {
									decl := name.Name
									if typeStr != "" {
										decl += " " + typeStr
									}
									pageData.Globals = append(pageData.Globals, Global{
										Name:        name.Name,
										Declaration: decl,
										Doc:         doc,
									})
								}
							}
						}
					}
				}
			}

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
		return nil
	})

	if err != nil {
		panic(err)
	}

	indexFile := filepath.Join(docDir, "index.html")
	f, err := os.Create(indexFile)
	if err != nil {
		fmt.Printf("Failed to create index.html: %v\n", err)
		return
	}
	defer f.Close()

	t := template.Must(template.New("index").Parse(indexTmpl))
	if err := t.Execute(f, indexEntries); err != nil {
		fmt.Printf("Error creating index: %v\n", err)
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

// Convert some expression to its string value
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

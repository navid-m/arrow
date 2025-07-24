package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
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
	docFile := fmt.Sprintf("%s-docs.html", docFileName)
	indexEntry := models.IndexEntry{
		PackageName: relPath,
		DocFile:     docFile,
	}

	for pkgName, pkg := range pkgs {
		pageData := models.PageData{PackageName: pkgName}

		subItems, err := os.ReadDir(filepath.Join(srcPath, relPath))
		if err == nil {
			for _, item := range subItems {
				if item.IsDir() {
					subPkgPath := filepath.Join(relPath, item.Name())
					goFiles, _ := filepath.Glob(filepath.Join(srcPath, subPkgPath, "*.go"))
					if len(goFiles) > 0 {
						subDocFile := strings.ReplaceAll(subPkgPath, string(filepath.Separator), "_") + "-docs.html"
						pageData.SubPackages = append(pageData.SubPackages, models.IndexEntry{
							PackageName: item.Name(),
							DocFile:     subDocFile,
						})
					}
				}
			}
		}

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

					pageData.Functions = append(pageData.Functions, models.Function{
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

							pageData.Structs = append(pageData.Structs, models.Struct{
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
								pageData.Globals = append(pageData.Globals, models.Global{
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

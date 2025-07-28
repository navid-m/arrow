package building

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/navid-m/arrow/models"
	"github.com/navid-m/arrow/parsing"
)

// Find the immediate subpackages relative to current location (one level deep)
func findImmediateSubpackages(srcPath, relPath string) []models.IndexEntry {
	var subPackages []models.IndexEntry

	searchPath := filepath.Join(srcPath, relPath)
	subItems, err := os.ReadDir(searchPath)
	if err != nil {
		return subPackages
	}

	for _, item := range subItems {
		if !item.IsDir() || strings.HasPrefix(item.Name(), ".") {
			continue
		}

		if item.Name() == "vendor" || item.Name() == "testdata" {
			continue
		}

		var (
			subPkgPath     = filepath.Join(relPath, item.Name())
			subPkgFullPath = filepath.Join(srcPath, subPkgPath)
			goFiles, err   = filepath.Glob(filepath.Join(subPkgFullPath, "*.go"))
		)
		if err != nil {
			continue
		}

		var nonTestFiles []string
		for _, file := range goFiles {
			if !strings.HasSuffix(filepath.Base(file), "_test.go") {
				nonTestFiles = append(nonTestFiles, file)
			}
		}

		if len(nonTestFiles) > 0 {
			fset := token.NewFileSet()
			pkgs, err := parser.ParseDir(fset, subPkgFullPath, func(fi os.FileInfo) bool {
				return strings.HasSuffix(fi.Name(), ".go") && !strings.HasSuffix(fi.Name(), "_test.go")
			}, 0)

			if err != nil {
				continue
			}

			filteredPkgs := make(map[string]*ast.Package)
			for pkgName, pkg := range pkgs {
				if !strings.HasSuffix(pkgName, "_test") {
					filteredPkgs[pkgName] = pkg
				}
			}

			baseDocFile := strings.ReplaceAll(subPkgPath, string(filepath.Separator), "_")
			if relPath == "." || relPath == "" {
				baseDocFile = item.Name()
			}

			var subDocFile string
			if len(filteredPkgs) > 1 {
				if _, exists := filteredPkgs[item.Name()]; exists {
					subDocFile = fmt.Sprintf("%s-%s-docs.html", baseDocFile, item.Name())
				} else {
					var pkgNames []string
					for pkgName := range filteredPkgs {
						pkgNames = append(pkgNames, pkgName)
					}
					sort.Strings(pkgNames)
					subDocFile = fmt.Sprintf("%s-%s-docs.html", baseDocFile, pkgNames[0])
				}
			} else {
				subDocFile = baseDocFile + "-docs.html"
			}

			subPackages = append(subPackages, models.IndexEntry{
				PackageName: item.Name(),
				DocFile:     subDocFile,
			})
		}
	}

	return subPackages
}

// Render the documentation, return a slice of index entries.
func RenderDocs(
	tmpl string,
	docFileName string,
	relPath string,
	pkgs map[string]*ast.Package,
	docDir string,
	srcPath string,
) []models.IndexEntry {
	var indexEntries []models.IndexEntry
	for pkgName, pkg := range pkgs {
		var currentDocFile string
		if len(pkgs) > 1 {
			currentDocFile = fmt.Sprintf("%s-%s-docs.html", docFileName, pkgName)
		} else {
			currentDocFile = fmt.Sprintf("%s-docs.html", docFileName)
		}

		pageData := models.PageData{PackageName: pkgName}
		pageData.SubPackages = findImmediateSubpackages(srcPath, relPath)

		functionCount := 0
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

					functionCount++

					var (
						params   = parsing.ExtractFieldList(d.Type.Params)
						results  = parsing.ExtractFieldList(d.Type.Results)
						fullSig  = BuildFunctionSignature(d, params, results)
						doc      = parsing.ExtractDocumentation(d.Doc)
						receiver string
					)
					if d.Recv != nil {
						receiver = parsing.ExtractFieldList(d.Recv)
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

							doc := parsing.ExtractDocumentation(d.Doc)
							if typeSpec.Doc != nil {
								doc = parsing.ExtractDocumentation(typeSpec.Doc)
							}

							switch t := typeSpec.Type.(type) {
							case *ast.StructType:
								pageData.Structs = append(pageData.Structs, models.Struct{
									Name:   typeSpec.Name.Name,
									Fields: parsing.ExtractStructFields(t),
									Doc:    doc,
									Kind:   "struct",
								})

							case *ast.InterfaceType:
								pageData.Interfaces = append(pageData.Interfaces, models.Interface{
									Name:    typeSpec.Name.Name,
									Methods: parsing.ExtractInterfaceMethods(t),
									Doc:     doc,
								})

							default:
								typeStr := parsing.ExprToString(typeSpec.Type)
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

							doc := parsing.ExtractDocumentation(d.Doc)
							if valueSpec.Doc != nil {
								doc = parsing.ExtractDocumentation(valueSpec.Doc)
							}

							typeStr := ""
							if valueSpec.Type != nil {
								typeStr = parsing.ExprToString(valueSpec.Type)
							}

							var values []string
							for _, val := range valueSpec.Values {
								values = append(values, parsing.ExprToString(val))
							}

							for i, name := range valueSpec.Names {
								decl := BuildVariableDeclaration(d.Tok, name.Name, typeStr, values, i)

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
		sort.Slice(pageData.SubPackages, func(i, j int) bool {
			return pageData.SubPackages[i].PackageName < pageData.SubPackages[j].PackageName
		})

		outFile := filepath.Join(docDir, currentDocFile)
		f, err := os.Create(outFile)
		if err != nil {
			fmt.Printf("Failed to create %s: %v\n", outFile, err)
			continue
		}
		defer f.Close()

		t := template.Must(template.New("doc").Parse(tmpl))
		if err := t.Execute(f, pageData); err != nil {
			fmt.Printf("Error executing template for %s: %v\n", outFile, err)
			continue
		}

		fmt.Printf("Generated %s\n", outFile)

		indexEntry := models.IndexEntry{
			PackageName: pkgName,
			DocFile:     currentDocFile,
		}
		indexEntries = append(indexEntries, indexEntry)
	}

	return indexEntries
}

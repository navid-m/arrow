package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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

type PageData struct {
	PackageName string
	Functions   []Function
}

const tmpl = `<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>{{.PackageName}} Docs</title>
	<link href="https://cdn.jsdelivr.net/npm/@picocss/pico@1/css/pico.min.css" rel="stylesheet">
	<style>
		html { font-size: 13px; }
		body {
			font-family: 'Fira Code', monospace;
			padding: 1rem;
			margin: 0 auto;
			max-width: 960px;
		}
		h1 {
			font-size: 1.4rem;
			margin-bottom: 1rem;
		}
		h2 {
			font-size: 1rem;
			margin: 0.3rem 0 0.2rem;
		}
		p {
			margin: 0 0 0.3rem;
			line-height: 1.3;
		}
		pre {
			background: #1e1e1e;
			color: #dcdcdc;
			padding: 0.4rem 0.6rem;
			border-radius: 6px;
			margin: 0 0 1rem;
			font-size: 0.85rem;
			overflow-x: auto;
		}
		code {
			color: #ffcc66;
			white-space: pre-wrap;
		}
		article {
			margin-bottom: 0.5rem;
			padding: 0.4rem 0.6rem;
			background: #2c2c2c;
			border-radius: 6px;
			box-shadow: 0 0 3px rgba(0,0,0,0.3);
		}
	</style>
</head>
<body>
	<main>
		<h1>Package <code>{{.PackageName}}</code> Documentation</h1>
		{{range .Functions}}
			<article>
				<h2>{{.Name}}</h2>
				{{if .Doc}}<p>{{.Doc}}</p>{{end}}
				<pre><code>{{.FullSig}}</code></pre>
			</article>
		{{end}}
	</main>
</body>
</html>`

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: arrow <path-to-go-source>")
		return
	}

	srcPath := os.Args[1]
	fileset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fileset, srcPath, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	var pageData PageData
	for pkgName, pkg := range pkgs {
		pageData.PackageName = pkgName

		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				if fn, isFn := decl.(*ast.FuncDecl); isFn {
					params := extractFieldList(fn.Type.Params)
					results := extractFieldList(fn.Type.Results)

					fullSig := fmt.Sprintf("func %s(%s)", fn.Name.Name, params)
					if results != "" {
						if strings.Contains(results, ",") {
							fullSig += " (" + results + ")"
						} else {
							fullSig += " " + results
						}
					}

					doc := ""
					if fn.Doc != nil {
						doc = strings.TrimSpace(fn.Doc.Text())
					}

					pageData.Functions = append(pageData.Functions, Function{
						Name:    fn.Name.Name,
						Params:  params,
						Results: results,
						FullSig: fullSig,
						Doc:     doc,
					})
				}
			}
		}
	}

	output, err := os.Create("docs.html")
	if err != nil {
		panic(err)
	}
	defer output.Close()

	t := template.Must(template.New("doc").Parse(tmpl))
	if err := t.Execute(output, pageData); err != nil {
		panic(err)
	}

	fmt.Println("Generated docs.html")
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

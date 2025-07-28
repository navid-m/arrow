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
	"sync"
	"text/template"

	"github.com/navid-m/arrow/building"
	"github.com/navid-m/arrow/models"
)

//go:embed views/template.htmx
var tmpl string

//go:embed views/index.htmx
var indexTmpl string

const ver = "v1.3.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: arrow <path-to-go-src-dir> [--name=<name>] | -v | --version")
		return
	}

	var (
		srcPath    string
		customName string
		fset       = token.NewFileSet()
		docDir     = filepath.Join(".", "docs")
	)

	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println("arrow " + ver)
			return
		}

		if after, ok := strings.CutPrefix(arg, "--name="); ok {
			customName = after
			continue
		}

		if srcPath == "" && !strings.HasPrefix(arg, "--") {
			srcPath = arg
		}
	}

	if srcPath == "" {
		fmt.Println("usage: arrow <path-to-go-src-dir> [--name=<name>] | -v | --version")
		return
	}

	if err := os.MkdirAll(docDir, 0755); err != nil {
		fmt.Printf("Failed to create docs directory: %v\n", err)
		return
	}

	var (
		indexEntries []models.IndexEntry
		mu           sync.Mutex
		wg           sync.WaitGroup
	)

	err := filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return err
		}

		pkgs, err := parser.ParseDir(fset, path, func(fi os.FileInfo) bool {
			return strings.HasSuffix(fi.Name(), ".go") && !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ParseComments)

		if err != nil || len(pkgs) == 0 {
			return nil
		}

		filteredPkgs := make(map[string]*ast.Package)
		for pkgName, pkg := range pkgs {
			if !strings.HasSuffix(pkgName, "_test") {
				filteredPkgs[pkgName] = pkg
			}
		}

		if len(filteredPkgs) == 0 {
			return nil
		}

		relPath, _ := filepath.Rel(srcPath, path)
		docFileName := strings.ReplaceAll(relPath, string(filepath.Separator), "_")
		if docFileName == "." || docFileName == "" {
			docFileName = "main"
		}

		wg.Add(1)
		go func(docFileName, relPath string, pkgs map[string]*ast.Package) {
			defer wg.Done()
			entries := building.RenderDocs(tmpl, docFileName, relPath, pkgs, docDir, srcPath)
			mu.Lock()
			indexEntries = append(indexEntries, entries...)
			mu.Unlock()
		}(docFileName, relPath, filteredPkgs)

		return nil
	})

	wg.Wait()

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

	var workingDirName string
	if customName != "" {
		workingDirName = customName
	} else {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Printf("Failed to get working directory: %v\n", err)
			return
		}
		workingDirName = filepath.Base(wd)
	}

	data := struct {
		IndexEntries   []models.IndexEntry
		WorkingDirName string
	}{
		IndexEntries:   indexEntries,
		WorkingDirName: workingDirName,
	}

	if err := t.Execute(f, data); err != nil {
		fmt.Printf("Error creating index: %v\n", err)
	}
}

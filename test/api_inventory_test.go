package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestExportedSymbolInventory_Baselines(t *testing.T) {
	packages := []struct {
		name   string
		dir    string
		golden string
	}{
		{name: "pkg/discovery", dir: filepath.Join("..", "pkg", "discovery"), golden: "api_inventory/pkg_discovery.txt"},
		{name: "pkg/resource", dir: filepath.Join("..", "pkg", "resource"), golden: "api_inventory/pkg_resource.txt"},
		{name: "pkg/repo", dir: filepath.Join("..", "pkg", "repo"), golden: "api_inventory/pkg_repo.txt"},
	}

	for _, pkg := range packages {
		pkg := pkg
		t.Run(pkg.name, func(t *testing.T) {
			symbols := collectExportedSymbols(t, pkg.dir)
			assertGoldenTextForTestPackage(t, pkg.golden, strings.Join(symbols, "\n")+"\n")
		})
	}
}

func collectExportedSymbols(t *testing.T, dir string) []string {
	t.Helper()

	fset := token.NewFileSet()
	pkgMap, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		name := fi.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse dir %s: %v", dir, err)
	}

	symbols := map[string]struct{}{}

	for _, pkg := range pkgMap {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					switch d.Tok {
					case token.CONST:
						for _, spec := range d.Specs {
							vs, ok := spec.(*ast.ValueSpec)
							if !ok {
								continue
							}
							for _, name := range vs.Names {
								if name.IsExported() {
									symbols["const "+name.Name] = struct{}{}
								}
							}
						}
					case token.VAR:
						for _, spec := range d.Specs {
							vs, ok := spec.(*ast.ValueSpec)
							if !ok {
								continue
							}
							for _, name := range vs.Names {
								if name.IsExported() {
									symbols["var "+name.Name] = struct{}{}
								}
							}
						}
					case token.TYPE:
						for _, spec := range d.Specs {
							ts, ok := spec.(*ast.TypeSpec)
							if !ok {
								continue
							}
							if ts.Name.IsExported() {
								symbols["type "+ts.Name.Name] = struct{}{}
							}
						}
					}
				case *ast.FuncDecl:
					if !d.Name.IsExported() {
						continue
					}
					if d.Recv == nil {
						symbols["func "+d.Name.Name] = struct{}{}
						continue
					}

					recv := receiverBaseName(d.Recv.List[0].Type)
					if recv != "" {
						symbols["method "+recv+"."+d.Name.Name] = struct{}{}
					}
				}
			}
		}
	}

	out := make([]string, 0, len(symbols))
	for symbol := range symbols {
		out = append(out, symbol)
	}
	slices.Sort(out)

	return out
}

func receiverBaseName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return e.Name
	}
	return ""
}

func assertGoldenTextForTestPackage(t *testing.T, relPath, actual string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", "golden", relPath)
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	if os.Getenv("AIMGR_UPDATE_BASELINES") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0644); err != nil {
			t.Fatalf("failed to write golden file %s: %v", goldenPath, err)
		}
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v", goldenPath, err)
	}

	if string(expected) != actual {
		t.Fatalf("golden mismatch for %s\nset AIMGR_UPDATE_BASELINES=1 to refresh", goldenPath)
	}
}

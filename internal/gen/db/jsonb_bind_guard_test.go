package db_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedParams_NoRawByteMachinePricingSnapshot(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, root, func(info fs.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse gen/db: %v", err)
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(ts.Name.Name, "Params") {
					return true
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 || field.Names[0].Name != "MachinePricingSnapshot" {
						continue
					}
					if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "[]byte" {
						t.Fatalf("%s: MachinePricingSnapshot must not be []byte (use string + ::text::jsonb cast)", ts.Name.Name)
					}
				}
				return true
			})
		}
	}
}

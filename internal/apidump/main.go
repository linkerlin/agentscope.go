// Command apidump prints a canonical listing of every exported symbol
// (types, funcs, methods, consts, vars with signatures) of every non-internal
// package in the module — the input to scripts/check_api_diff.sh (23.7).
// Run from the module root; each output line is "<pkg>\t<decl>".
package main

import (
	"bufio"
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	modBytes, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	module := ""
	for _, line := range strings.Split(string(modBytes), "\n") {
		if strings.HasPrefix(line, "module ") {
			module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	if module == "" {
		fmt.Fprintln(os.Stderr, "apidump: no module directive")
		os.Exit(1)
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	// Walk every directory that holds .go files; skip non-library trees.
	type pkgDir struct{ rel, importPath string }
	var dirs []pkgDir
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		base := filepath.Base(rel)
		switch {
		case rel == ".":
		case strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_"):
			return filepath.SkipDir
		case base == "internal", base == "examples", base == "tests",
			base == "docs", base == "cmd", base == "vendor", base == "testdata",
			base == "quality", base == "scripts":
			return filepath.SkipDir
		}
		m, _ := filepath.Glob(filepath.Join(path, "*.go"))
		if len(m) == 0 {
			return nil
		}
		importPath := module
		if rel != "." {
			importPath += "/" + filepath.ToSlash(rel)
		}
		dirs = append(dirs, pkgDir{rel: rel, importPath: importPath})
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].importPath < dirs[j].importPath })

	for _, d := range dirs {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, d.rel, func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ParseComments)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(pkgs))
		for n := range pkgs {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if n == "main" { // examples that slipped through / main packages
				continue
			}
			p := doc.New(pkgs[n], d.importPath, doc.Mode(0))
			var lines []string
			for _, c := range p.Consts {
				for _, name := range c.Names {
					lines = append(lines, "const "+name)
				}
			}
			for _, v := range p.Vars {
				for _, name := range v.Names {
					lines = append(lines, "var "+name)
				}
			}
			for _, f := range p.Funcs {
				lines = append(lines, "func "+f.Name+" "+funcSig(f.Decl))
			}
			for _, t := range p.Types {
				lines = append(lines, "type "+t.Name)
				for _, f := range t.Funcs {
					lines = append(lines, "func "+f.Name+" "+funcSig(f.Decl))
				}
				for _, m := range t.Methods {
					recv := ""
					if m.Decl.Recv != nil && len(m.Decl.Recv.List) > 0 {
						recv = exprString(m.Decl.Recv.List[0].Type)
					}
					lines = append(lines, "method "+t.Name+" "+strings.Trim(recv, "*")+"."+m.Name+" "+funcSig(m.Decl))
				}
			}
			sort.Strings(lines)
			for _, l := range lines {
				fmt.Fprintf(out, "%s\t%s\n", d.importPath, l)
			}
		}
	}
}

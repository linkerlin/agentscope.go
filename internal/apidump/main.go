// Command apidump prints a canonical listing of every exported symbol
// (types, funcs, methods, consts, vars with signatures) of every non-internal
// package in the module — the input to scripts/check_api_diff.sh (23.7).
// Run from the module root; each output line is "<pkg>\t<decl>".
//
// Method faces include promoted methods: an exported struct embedding a type
// from this or another package exposes that type's exported methods, and an
// exported alias (`type X = pkg.Y`) carries Y's whole method face. Since
// 16.2 the gateway root keeps assembly wrappers of exactly these shapes, so
// counting only receiver-declared methods would report their inherited
// method faces as breaking even though every caller still compiles. Promoted
// resolution is AST-level: embeds and alias targets are resolved through the
// declaring file's import table, then method faces propagate to a fixed
// point (local declarations shadow promoted ones, as in the language).
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// typeRef names a type: pkg is the import path ("" = the package currently
// being dumped) and name the type name.
type typeRef struct{ pkg, name string }

// typeDecl captures what a pass-1 dump needs to know about one exported type
// beyond its own method declarations.
type typeDecl struct {
	// embeds lists the types embedded (value or pointer) in an exported
	// struct; aliasTo is set for `type X = ...` declarations. Both use the
	// declaring file's import table to resolve package qualifiers.
	embeds   []typeRef
	aliasTo  *typeRef
	imports  map[string]string // file-local pkg name -> import path
	isStruct bool
}

// pkgDump is one package's pass-1 result.
type pkgDump struct {
	// methods holds the receiver-declared exported methods per type:
	// name -> method -> signature text.
	methods map[string]map[string]string
	// types holds declaration metadata for promoted-method resolution.
	types map[string]*typeDecl
}

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

	// Pass 1: parse every package once, collecting declarations, method
	// signatures, and type metadata (embeds / alias targets / import tables).
	dumps := make(map[string]*pkgDump) // importPath -> dump
	// lines per package holds the non-method declarations verbatim.
	declLines := make(map[string][]string)
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
			dump := &pkgDump{
				methods: make(map[string]map[string]string),
				types:   make(map[string]*typeDecl),
			}
			for _, t := range p.Types {
				lines = append(lines, "type "+t.Name)
				for _, f := range t.Funcs {
					lines = append(lines, "func "+f.Name+" "+funcSig(f.Decl))
				}
				for _, m := range t.Methods {
					if m.Decl.Recv != nil && len(m.Decl.Recv.List) > 0 {
						recv := exprString(m.Decl.Recv.List[0].Type)
						lines = append(lines, "method "+t.Name+" "+strings.Trim(recv, "*")+"."+m.Name+" "+funcSig(m.Decl))
					}
				}
				dump.methods[t.Name] = methodMap(t)
				dump.types[t.Name] = typeMeta(p, t, pkgs[n])
			}
			sort.Strings(lines)
			declLines[d.importPath] = append(declLines[d.importPath], lines...)
			if _, ok := dumps[d.importPath]; !ok {
				dumps[d.importPath] = dump
			} else {
				// multiple parsed packages under one import path (rare):
				// merge.
				for k, v := range dump.methods {
					dumps[d.importPath].methods[k] = v
				}
				for k, v := range dump.types {
					dumps[d.importPath].types[k] = v
				}
			}
		}
	}

	// Pass 2: emit declarations, with each exported type's method face
	// expanded to include promoted methods.
	for _, d := range dirs {
		dump := dumps[d.importPath]
		if dump == nil {
			continue
		}
		lines := append([]string(nil), declLines[d.importPath]...)
		var typeNames []string
		for name := range dump.types {
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)
		for _, name := range typeNames {
			local := dump.methods[name]
			for mname, sig := range promotedMethods(dumps, d.importPath, name, nil) {
				if _, isLocal := local[mname]; isLocal {
					continue // receiver-declared methods are already in declLines
				}
				lines = append(lines, "method "+name+" "+name+"."+mname+" "+sig)
			}
		}
		sort.Strings(lines)
		lines = dedup(lines)
		for _, l := range lines {
			fmt.Fprintf(out, "%s\t%s\n", d.importPath, l)
		}
	}
}

// dedup drops adjacent duplicate lines (the input must already be sorted).
func dedup(sorted []string) []string {
	out := sorted[:0]
	for i, l := range sorted {
		if i > 0 && sorted[i-1] == l {
			continue
		}
		out = append(out, l)
	}
	return out
}

// methodMap extracts the type's receiver-declared exported methods with
// rendered signatures.
func methodMap(t *doc.Type) map[string]string {
	mm := make(map[string]string)
	for _, m := range t.Methods {
		if m.Decl.Recv != nil && len(m.Decl.Recv.List) > 0 {
			mm[m.Name] = funcSig(m.Decl)
		}
	}
	return mm
}

// typeMeta inspects the type's declaration for embeds and alias targets.
// The import table of the declaring file resolves package qualifiers; doc
// gives us the AST spec, and the package's file set maps back to files.
func typeMeta(p *doc.Package, t *doc.Type, pkg *ast.Package) *typeDecl {
	td := &typeDecl{imports: map[string]string{}}
	ts, ok := t.Decl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return td
	}
	file := fileOf(pkg, t.Decl)
	if file != nil {
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			name := ""
			if imp.Name != nil {
				name = imp.Name.Name
			} else {
				name = path[strings.LastIndex(path, "/")+1:]
			}
			td.imports[name] = path
		}
	}
	switch spec := ts.Type.(type) {
	case *ast.StructType:
		td.isStruct = true
		if spec.Fields != nil {
			for _, f := range spec.Fields.List {
				if len(f.Names) != 0 { // named field, not an embed
					continue
				}
				if ref, ok := embedRef(f.Type, td.imports); ok {
					td.embeds = append(td.embeds, ref)
				}
			}
		}
	case *ast.Ident:
		// `type X = Y` (alias to a same-package type).
		td.aliasTo = &typeRef{name: spec.Name}
	case *ast.SelectorExpr:
		// `type X = pkg.Y` (alias to another package's type).
		if pkgIdent, ok := spec.X.(*ast.Ident); ok {
			if path, ok2 := td.imports[pkgIdent.Name]; ok2 {
				td.aliasTo = &typeRef{pkg: path, name: spec.Sel.Name}
			} else {
				td.aliasTo = &typeRef{name: spec.Sel.Name} // unresolved: assume same package
			}
		}
	case *ast.StarExpr:
		// `type X = *pkg.Y` — an alias to a pointer type still exposes Y's
		// method face on addressable values; count it.
		if ref, ok := embedRef(spec, td.imports); ok {
			td.aliasTo = &ref
		}
	}
	return td
}

// embedRef resolves an embedded field type (or an alias target) to a typeRef,
// unwrapping pointer qualifiers. Generic instantiations and other exotic
// forms are ignored (none are exported in this module).
func embedRef(e ast.Expr, imports map[string]string) (typeRef, bool) {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	switch v := e.(type) {
	case *ast.Ident:
		return typeRef{name: v.Name}, true
	case *ast.SelectorExpr:
		if pkgIdent, ok := v.X.(*ast.Ident); ok {
			if path, ok2 := imports[pkgIdent.Name]; ok2 {
				return typeRef{pkg: path, name: v.Sel.Name}, true
			}
			return typeRef{name: v.Sel.Name}, true
		}
	}
	return typeRef{}, false
}

// fileOf finds the *ast.File containing the declaration (best effort: any
// file whose declarations include the GenDecl).
func fileOf(pkg *ast.Package, decl ast.Decl) *ast.File {
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			if d == decl {
				return f
			}
		}
	}
	return nil
}

// promotedMethods returns the type's full exported method face: local
// declarations plus methods promoted through embeds and alias targets,
// recursively. Local declarations shadow promoted ones (language semantics).
// visiting guards cycles (type A embeds B embeds A is illegal in Go, but the
// guard keeps resolution total anyway).
func promotedMethods(dumps map[string]*pkgDump, pkgPath, typeName string, visiting map[typeRef]bool) map[string]string {
	ref := typeRef{pkg: pkgPath, name: typeName}
	if visiting[ref] {
		return map[string]string{}
	}
	if visiting == nil {
		visiting = map[typeRef]bool{}
	}
	visiting[ref] = true
	defer delete(visiting, ref)

	dump := dumps[pkgPath]
	if dump == nil {
		return map[string]string{}
	}
	td := dump.types[typeName]
	face := map[string]string{}
	for m, sig := range dump.methods[typeName] {
		face[m] = sig
	}
	if td == nil {
		return face
	}
	// Alias: the face is the target's face, without the local decls merged
	// twice (an alias has no decls of its own).
	if td.aliasTo != nil {
		targetPkg := td.aliasTo.pkg
		if targetPkg == "" {
			targetPkg = pkgPath
		}
		return promotedMethods(dumps, targetPkg, td.aliasTo.name, visiting)
	}
	for _, emb := range td.embeds {
		if !ast.IsExported(emb.name) {
			continue // unexported embeds do not promote across packages
		}
		embPkg := emb.pkg
		if embPkg == "" {
			embPkg = pkgPath
		}
		for m, sig := range promotedMethods(dumps, embPkg, emb.name, visiting) {
			if _, shadowed := face[m]; !shadowed {
				face[m] = sig
			}
		}
	}
	return face
}

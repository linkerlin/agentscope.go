// Command quality computes per-package statement coverage from a Go
// coverage profile — the arithmetic behind scripts/check_coverage.sh (23.6).
// A package's coverage is sum(numStmts where count>0) / sum(numStmts).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
)

func main() {
	profile := flag.String("profile", "", "coverage profile path (go test -coverprofile output)")
	pkg := flag.String("pkg", "", "print coverage for this import path only (empty = all)")
	flag.Parse()

	f, err := os.Open(*profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	// per package: total and covered statement counts.
	type acc struct{ total, covered float64 }
	byPkg := map[string]*acc{}
	order := []string{}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		// <file>:<startLine>.<startCol>,<endLine>.<endCol> <numStmts> <count>
		parts := strings.Fields(line)
		if len(parts) != 3 {
			continue
		}
		stmts, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}
		count, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			continue
		}
		file := parts[0]
		dir := path.Dir(file)
		// import path: strip a leading "./" that profiles sometimes carry.
		dir = strings.TrimPrefix(dir, "./")
		a := byPkg[dir]
		if a == nil {
			a = &acc{}
			byPkg[dir] = a
			order = append(order, dir)
		}
		a.total += stmts
		if count > 0 {
			a.covered += stmts
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	printPct := func(p string) {
		a := byPkg[p]
		if a == nil || a.total == 0 {
			fmt.Printf("0.0")
			return
		}
		fmt.Printf("%.1f", a.covered/a.total*100)
	}

	if *pkg != "" {
		printPct(*pkg)
		fmt.Println()
		return
	}
	for _, p := range order {
		a := byPkg[p]
		fmt.Printf("%s %.1f%% (%.0f/%.0f)\n", p, a.covered/a.total*100, a.covered, a.total)
	}
}

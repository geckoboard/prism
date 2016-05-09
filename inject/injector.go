package inject

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type parsedFile struct {
	filePath string
	pkgName  string
	fset     *token.FileSet
	astFile  *ast.File
	touched  bool
}

type Injector struct {
	pkgRootPath         string
	pkgRootPathOverride string

	hookedFuncs map[string]*Target

	parsedFiles []*parsedFile
}

func NewInjector(pkgRootPath, pkgRootPathOverride string) *Injector {
	return &Injector{
		parsedFiles:         make([]*parsedFile, 0),
		pkgRootPath:         pkgRootPath,
		pkgRootPathOverride: pkgRootPathOverride,
	}
}

// Apply profiler hooks to the supplied targets. Returns the number of modified files.
func (in *Injector) Hook(targets []*Target) (int, error) {
	// Parse package files
	err := filepath.Walk(in.pkgRootPath, in.buildAST)
	if err != nil {
		return 0, err
	}

	in.initCache(targets)

	touchedFiles := 0
	for _, parsedFile := range in.parsedFiles {
		injectProfiler(parsedFile, in)

		// Write modified ASTs to disk
		if parsedFile.touched {
			f, err := os.Create(parsedFile.filePath)
			if err != nil {
				return 0, err
			}
			printer.Fprint(f, parsedFile.fset, parsedFile.astFile)
			f.Close()
			touchedFiles++
		}
	}

	return touchedFiles, nil
}

// Initialize cache for accelerating lookups and ensuring that shared targets
// are only hooked once.
func (in *Injector) initCache(targets []*Target) {
	in.hookedFuncs = make(map[string]*Target, len(targets))

	for _, target := range targets {
		in.hookedFuncs[target.Name] = target
	}
}

// Parse a go file and store its AST representation.
func (in *Injector) buildAST(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	// Skip dirs and non-go files
	if info.IsDir() || !strings.HasSuffix(path, ".go") {
		return nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return fmt.Errorf("in: could not parse %s; %v", path, err)
	}

	pkgName, err := qualifiedPkgName(
		strings.Replace(path, in.pkgRootPath, in.pkgRootPathOverride, -1),
	)
	if err != nil {
		return err
	}

	in.parsedFiles = append(in.parsedFiles, &parsedFile{
		pkgName:  pkgName,
		filePath: path,
		fset:     fset,
		astFile:  f,
	})

	return nil
}

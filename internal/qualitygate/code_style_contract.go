package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const expectedStyleDescriptorCount = 35

type codeStyleContract struct {
	root string
}

func (contract codeStyleContract) check() error {
	repository, err := os.OpenRoot(contract.root)
	if err != nil {
		return fmt.Errorf("open repository root for canonical CODE_STYLE.md: %w", err)
	}
	content, readErr := repository.ReadFile("CODE_STYLE.md")
	if err = errors.Join(readErr, repository.Close()); err != nil {
		return fmt.Errorf("read canonical CODE_STYLE.md: %w", err)
	}
	for _, required := range []string{
		"**Status:** Normative for application code",
		"**Profile name:** `java-structured`",
		"# Part IX — Automated enforcement implementation",
		"\"schemaVersion\": 1",
		"\"packageFunctions\": \"error\"",
		"spice.style.file.one-primary-type",
		"spice.style.route.classification",
	} {
		if !bytes.Contains(content, []byte(required)) {
			return fmt.Errorf("CODE_STYLE.md is missing required contract %q", required)
		}
	}
	if bytes.Contains(content, []byte("\"packageFunctions\": \"deny\"")) {
		return fmt.Errorf("CODE_STYLE.md contains retired packageFunctions level deny")
	}
	table := styleDescriptorTable(content)
	if len(table) != expectedStyleDescriptorCount {
		return fmt.Errorf(
			"CODE_STYLE.md official descriptor inventory has %d rows, want %d",
			len(table),
			expectedStyleDescriptorCount,
		)
	}
	descriptors, err := contract.descriptorNames()
	if err != nil {
		return err
	}
	if len(descriptors) != expectedStyleDescriptorCount {
		return fmt.Errorf("official descriptor source has %d definitions, want %d", len(descriptors), expectedStyleDescriptorCount)
	}
	joined := strings.Join(table, "\n")
	for _, descriptor := range descriptors {
		if !strings.Contains(joined, "@"+descriptor+"`") &&
			!strings.Contains(joined, "."+descriptor+"`") {
			return fmt.Errorf("CODE_STYLE.md inventory omits official descriptor %s", descriptor)
		}
	}
	return nil
}

func styleDescriptorTable(content []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	inventory := false
	var rows []string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "## 21. Complete official annotation inventory"):
			inventory = true
		case inventory && strings.HasPrefix(line, "## 22."):
			return rows
		case inventory && strings.HasPrefix(line, "| `@"):
			rows = append(rows, line)
		}
	}
	return rows
}

func (contract codeStyleContract) descriptorNames() ([]string, error) {
	root := filepath.Join(contract.root, "annotation")
	names := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse descriptor source %s: %w", path, err)
		}
		for _, declaration := range file.Decls {
			if name, ok := descriptorName(declaration); ok {
				names[name] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect official descriptor source: %w", err)
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func descriptorName(declaration ast.Decl) (string, bool) {
	function, ok := declaration.(*ast.FuncDecl)
	if !ok || function.Recv != nil || !function.Name.IsExported() ||
		function.Type.Results == nil || len(function.Type.Results.List) != 1 {
		return "", false
	}
	selector, ok := function.Type.Results.List[0].Type.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Definition" {
		return "", false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Name != "sdk" {
		return "", false
	}
	return function.Name.Name, true
}

package server

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// The maintenance controller dispatches before ServeMux. Extract its actual
// method/path predicates rather than keeping a second list of business routes.
// Unknown predicate forms fail closed, requiring review when dispatch changes.
type routePredicate struct{ method, exact, prefix, suffix string }

func controllerOperations(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "../backup/controller.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var patterns []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "control" && fn.Name.Name != "ServeHTTP") {
			continue
		}
		for _, statement := range fn.Body.List {
			branch, ok := statement.(*ast.IfStmt)
			if !ok {
				if _, isSwitch := statement.(*ast.SwitchStmt); isSwitch && fn.Name.Name == "control" {
					t.Fatal("update controller coverage for switch dispatch")
				}
				continue
			}
			clauses, err := parseRoutePredicate(branch.Cond)
			if err != nil {
				if fn.Name.Name == "control" {
					t.Fatal(err)
				}
				continue // ServeHTTP also has lifecycle/auth predicates, not route dispatch.
			}
			for _, clause := range clauses {
				path := clause.exact
				if path == "" && clause.prefix != "" {
					path = clause.prefix + "{id}" + clause.suffix
				}
				if clause.method == "" || path == "" {
					continue
				}
				patterns = append(patterns, clause.method+" "+path)
			}
		}
	}
	if len(patterns) == 0 {
		t.Fatal("no controller operations discovered")
	}
	return patterns
}

func parseRoutePredicate(e ast.Expr) ([]routePredicate, error) {
	if p, ok := e.(*ast.ParenExpr); ok {
		return parseRoutePredicate(p.X)
	}
	literal := func(e ast.Expr) string {
		if s, ok := e.(*ast.BasicLit); ok && s.Kind == token.STRING {
			v, _ := strconv.Unquote(s.Value)
			return v
		}
		return ""
	}
	name := func(e ast.Expr) string {
		if n, ok := e.(*ast.Ident); ok {
			return n.Name
		}
		if s, ok := e.(*ast.SelectorExpr); ok {
			if n, ok := s.X.(*ast.Ident); ok && n.Name == "r" && s.Sel.Name == "Method" {
				return "method"
			}
		}
		return ""
	}
	if b, ok := e.(*ast.BinaryExpr); ok {
		if b.Op == token.EQL {
			value := literal(b.Y)
			switch name(b.X) {
			case "method":
				return []routePredicate{{method: value}}, nil
			case "path":
				return []routePredicate{{exact: value}}, nil
			}
		}
		if b.Op == token.LAND || b.Op == token.LOR {
			left, err := parseRoutePredicate(b.X)
			if err != nil {
				return nil, err
			}
			right, err := parseRoutePredicate(b.Y)
			if err != nil {
				return nil, err
			}
			if b.Op == token.LOR {
				return append(left, right...), nil
			}
			var result []routePredicate
			for _, a := range left {
				for _, b := range right {
					merge := func(x, y string) (string, error) {
						if x != "" && y != "" && x != y {
							return "", fmt.Errorf("conflicting route predicates")
						}
						if y != "" {
							return y, nil
						}
						return x, nil
					}
					var c routePredicate
					for _, pair := range []struct {
						x, y string
						out  *string
					}{{a.method, b.method, &c.method}, {a.exact, b.exact, &c.exact}, {a.prefix, b.prefix, &c.prefix}, {a.suffix, b.suffix, &c.suffix}} {
						v, e := merge(pair.x, pair.y)
						if e != nil {
							return nil, e
						}
						*pair.out = v
					}
					result = append(result, c)
				}
			}
			return result, nil
		}
	}
	if call, ok := e.(*ast.CallExpr); ok && len(call.Args) == 2 && name(call.Args[0]) == "path" {
		if fn, ok := call.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := fn.X.(*ast.Ident); ok && pkg.Name == "strings" {
				switch fn.Sel.Name {
				case "HasPrefix":
					return []routePredicate{{prefix: literal(call.Args[1])}}, nil
				case "HasSuffix":
					return []routePredicate{{suffix: literal(call.Args[1])}}, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("unsupported controller route predicate %T; update the contract extractor", e)
}

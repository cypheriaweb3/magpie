//go:build !nogui

package main

import (
	"encoding/json"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type openAPIDocument struct {
	OpenAPI string `json:"openapi"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Security   []map[string]any                      `json:"security"`
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		SecuritySchemes map[string]struct {
			Type   string `json:"type"`
			Scheme string `json:"scheme"`
		} `json:"securitySchemes"`
	} `json:"components"`
}

func readOpenAPI(t *testing.T) openAPIDocument {
	t.Helper()
	var doc openAPIDocument
	if err := json.Unmarshal(adminOpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestOpenAPIDocument(t *testing.T) {
	doc := readOpenAPI(t)
	if doc.OpenAPI != "3.1.0" {
		t.Fatalf("openapi = %q", doc.OpenAPI)
	}
	if len(doc.Servers) != 1 || doc.Servers[0].URL != "/" {
		t.Fatalf("servers = %#v", doc.Servers)
	}
	scheme, ok := doc.Components.SecuritySchemes["bearerAuth"]
	if !ok || scheme.Type != "http" || scheme.Scheme != "bearer" {
		t.Fatalf("bearerAuth = %#v", scheme)
	}
	if len(doc.Security) != 1 {
		t.Fatalf("global security = %#v", doc.Security)
	}
	if _, ok := doc.Paths["/openapi.json"]["get"]; !ok {
		t.Fatal("GET /openapi.json is missing")
	}

	operationIDs := map[string]string{}
	for path, item := range doc.Paths {
		for method, raw := range item {
			var operation struct {
				OperationID string                     `json:"operationId"`
				Responses   map[string]json.RawMessage `json:"responses"`
			}
			if err := json.Unmarshal(raw, &operation); err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			if operation.OperationID == "" {
				t.Errorf("%s %s has no operationId", method, path)
			} else if prior := operationIDs[operation.OperationID]; prior != "" {
				t.Errorf("operationId %q is shared by %s and %s %s", operation.OperationID, prior, method, path)
			} else {
				operationIDs[operation.OperationID] = method + " " + path
			}
			if _, ok := operation.Responses["401"]; !ok {
				t.Errorf("%s %s does not document authentication failure", method, path)
			}
		}
	}
}

func TestOpenAPIReferencesResolve(t *testing.T) {
	var root any
	if err := json.Unmarshal(adminOpenAPI, &root); err != nil {
		t.Fatal(err)
	}
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if key == "$ref" {
					ref, ok := child.(string)
					if !ok || !strings.HasPrefix(ref, "#/") {
						t.Errorf("unsupported reference %#v", child)
						continue
					}
					var at any = root
					for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
						part = strings.NewReplacer("~1", "/", "~0", "~").Replace(part)
						object, ok := at.(map[string]any)
						if !ok {
							t.Errorf("%s traverses a non-object at %q", ref, part)
							break
						}
						at, ok = object[part]
						if !ok {
							t.Errorf("%s does not resolve", ref)
							break
						}
					}
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(root)
}

// guiRoutes reads the route literals from the production GUI handler. This
// intentionally avoids a second hand-maintained route list: adding or removing
// a handler makes the OpenAPI coverage test fail until the document follows.
func guiRoutes(t *testing.T) map[string]map[string]bool {
	t.Helper()
	out := map[string]map[string]bool{}
	dir := filepath.Join("internal", "gui")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		matches, err := build.Default.MatchFile(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		if !matches {
			continue
		}
		file, err := parser.ParseFile(files, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc") {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			pattern, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Errorf("%s: invalid route literal: %v", name, err)
				return true
			}
			method, path, ok := strings.Cut(pattern, " ")
			if !ok || !strings.HasPrefix(path, "/api/") {
				return true
			}
			method = strings.ToLower(method)
			if out[path] == nil {
				out[path] = map[string]bool{}
			}
			out[path][method] = true
			return true
		})
	}
	return out
}

func TestOpenAPICoversGUIRoutes(t *testing.T) {
	doc := readOpenAPI(t)
	routes := guiRoutes(t)
	for path, methods := range routes {
		for method := range methods {
			if _, ok := doc.Paths[path][method]; !ok {
				t.Errorf("OpenAPI is missing %s %s", strings.ToUpper(method), path)
			}
		}
	}
	for path, item := range doc.Paths {
		if path == "/openapi.json" {
			continue
		}
		for method := range item {
			if !routes[path][method] {
				t.Errorf("OpenAPI has %s %s, but the production GUI handler does not", strings.ToUpper(method), path)
			}
		}
	}
}

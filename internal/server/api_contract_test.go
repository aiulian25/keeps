package server

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	registeredRoute = regexp.MustCompile(`"(GET|POST|PUT|PATCH|DELETE) (/api/v1/[^"]*)"`)
	documentedRoute = regexp.MustCompile("(?m)^\\|\\s*(GET|POST|PUT|PATCH|DELETE)\\s*\\|\\s*`(/api/v1/[^`]*)`")
)

// TestEveryRouteIsDocumented keeps docs/api.md, the contract future clients
// rely on, in step with the router.
func TestEveryRouteIsDocumented(t *testing.T) {
	documentation, err := os.ReadFile(filepath.Join("..", "..", "docs", "api.md"))
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]bool{}
	for _, match := range documentedRoute.FindAllStringSubmatch(string(documentation), -1) {
		documented[match[1]+" "+match[2]] = true
	}

	registered := map[string]bool{}
	filepath.WalkDir(filepath.Join("..", ".."), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, "node_modules") {
			return nil
		}
		content, _ := os.ReadFile(path)
		for _, match := range registeredRoute.FindAllStringSubmatch(string(content), -1) {
			registered[match[1]+" "+match[2]] = true
		}
		return nil
	})

	if len(registered) == 0 {
		t.Fatal("no routes found")
	}
	for route := range registered {
		if !documented[route] {
			t.Errorf("route not documented in docs/api.md: %s", route)
		}
	}
	for route := range documented {
		if !registered[route] {
			t.Errorf("docs/api.md lists a route that does not exist: %s", route)
		}
	}
}

// Package internal_test enforces the layering contract from CLAUDE.md §2 as a
// test rather than as a convention people remember.
//
// Dependencies point inward only: adapter → app → domain, with platform
// available to all. The domain imports no framework, no driver, no net/http and
// no SQL.
package internal_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modulePath = "github.com/stevenwilliam/ruuma"

// forbidden lists import prefixes that must never appear in a layer.
var rules = []struct {
	layer     string // directory under internal/
	forbidden []string
	reason    string
}{
	{
		layer: "domain",
		forbidden: []string{
			"net/http", "database/sql", "github.com/gin-gonic", "gorm.io",
			"github.com/minio", "github.com/golang-jwt", "github.com/prometheus",
			modulePath + "/internal/adapter", modulePath + "/internal/app",
			modulePath + "/db",
		},
		reason: "the domain is pure: no framework, no driver, no HTTP, no SQL (CLAUDE.md §2)",
	},
	{
		layer: "app",
		forbidden: []string{
			"github.com/gin-gonic", "gorm.io", "net/http",
			modulePath + "/internal/adapter",
		},
		reason: "app orchestrates domain through ports; it must not know an adapter or a driver",
	},
	{
		layer: "platform",
		forbidden: []string{
			modulePath + "/internal/domain", modulePath + "/internal/app",
			modulePath + "/internal/adapter",
		},
		reason: "platform is business-agnostic and reusable across projects (CLAUDE.md §2)",
	},
}

func TestLayeringRule(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	for _, rule := range rules {
		layerDir := filepath.Join(root, rule.layer)
		if _, err := os.Stat(layerDir); os.IsNotExist(err) {
			continue // layer not built yet
		}

		err := filepath.Walk(layerDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				for _, bad := range rule.forbidden {
					if importPath == bad || strings.HasPrefix(importPath, bad+"/") {
						rel, _ := filepath.Rel(root, path)
						t.Errorf("internal/%s: %s imports %q — %s", rule.layer, rel, importPath, rule.reason)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", rule.layer, err)
		}
	}
}

// The platform packages are meant to be carried between projects (CLAUDE.md
// §2), so nothing in them may mention ruuma's business nouns.
func TestPlatformIsBusinessAgnostic(t *testing.T) {
	businessNouns := []string{"MenuItem", "OrderLine", "KitchenUnit", "PromoCode"}

	root, err := filepath.Abs("platform")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("platform not built yet")
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path) //nolint:gosec // test walks its own tree
		if err != nil {
			return err
		}
		for _, noun := range businessNouns {
			if strings.Contains(string(body), noun) {
				t.Errorf("platform/%s mentions %q — platform must stay business-agnostic",
					filepath.Base(path), noun)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

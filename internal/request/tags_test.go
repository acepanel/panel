package request

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/libtnb/validator"

	"github.com/acepanel/panel/v3/internal/rule"
)

// tagValidator mirrors bootstrap.NewValidator's rule set. db is nil: CheckType and
// AddRules only compile expressions, they never invoke a rule's Validate or touch
// the database.
func tagValidator() *validator.Validator {
	return validator.MustNew(
		validator.WithStrictRequired(),
		validator.WithRules(rule.Rules()...),
		validator.WithFallibleRules(rule.FallibleRules(nil)...),
	)
}

// TestCheckRulesKeyRequests runs the validator's own tag checker over the
// requests with the richest rules (custom rules, dive, cross-field, and the
// v0.2 additions unique / after_or_equal), catching bad tags at test time.
func TestCheckRulesKeyRequests(t *testing.T) {
	v := tagValidator()
	for _, req := range []any{
		&WebsiteCreate{}, &WebsiteUpdate{}, &WebsiteDefaultConfig{},
		&CertCreate{}, &CertUpdate{},
		&FileCompress{}, &FilePermission{},
		&SettingPanel{}, &UserTokenCreate{}, &FirewallScanSetting{},
		&WebsiteStatDateRange{}, &BackupCreate{},
	} {
		if err := v.CheckType(reflect.TypeOf(req)); err != nil {
			t.Errorf("%T: %v", req, err)
		}
	}
}

// TestValidateTagsCompile is the whole-package backstop: every validate tag in
// this package must compile against the real validator, so a typo'd rule name, a
// DSL syntax error, or an unregistered custom rule fails the build. This is the
// engine-backed successor to the ad-hoc checktag used during the gookit migration
// (which, being a plain tag scanner, could not see the Rules() maps that hid the
// Domains.* bug, nor tell whether a custom rule was registered).
func TestValidateTagsCompile(t *testing.T) {
	v := tagValidator()
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			field, ok := n.(*ast.Field)
			if !ok || field.Tag == nil {
				return true
			}
			expr := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("validate")
			if strings.TrimSpace(expr) == "" {
				return true
			}
			name := "field"
			if len(field.Names) > 0 {
				name = field.Names[0].Name
			}
			seen++
			vd, err := v.Map(map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = vd.AddRules(name, expr); err != nil {
				t.Errorf("%s: %s `validate:%q`: %v", file, name, expr, err)
			}
			return true
		})
	}
	if seen == 0 {
		t.Fatal("no validate tags found — the scanner is broken")
	}
	t.Logf("compiled %d validate tags", seen)
}

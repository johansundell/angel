package main

import (
	"html/template"
	"strings"
	"testing"
)

// TestBaseTitleWithoutPageTitle keeps a page rendered without a title from
// showing a dangling "Angel – " in the tab.
func TestBaseTitleWithoutPageTitle(t *testing.T) {
	tmpl, err := template.ParseFS(embeddedTemplates, "tmpl/base.html")
	if err != nil {
		t.Fatalf("parse base template: %v", err)
	}
	if _, err := tmpl.New("content").Parse(""); err != nil {
		t.Fatalf("define content: %v", err)
	}
	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, "base", map[string]any{}); err != nil {
		t.Fatalf("execute base: %v", err)
	}
	if !strings.Contains(b.String(), "<title>Angel</title>") {
		t.Errorf("untitled page head lacks <title>Angel</title>:\n%s", b.String())
	}
}

package main

import (
	"html/template"
	"strings"
	"testing"
)

// footerLinks are the links every page's open-source footer must carry. Both
// open in a new tab without handing the new page a reference to this one.
var footerLinks = []string{
	`<a href="https://github.com/johansundell/angel/blob/main/LICENSE" target="_blank" rel="noopener noreferrer">MIT-licens</a>`,
	`<a href="https://github.com/johansundell/angel" target="_blank" rel="noopener noreferrer">GitHub</a>`,
}

// TestBaseFooter keeps the open-source footer on every page rendered through
// the base template.
func TestBaseFooter(t *testing.T) {
	tmpl, err := template.ParseFS(embeddedTemplates, "tmpl/base.html")
	if err != nil {
		t.Fatalf("parse base template: %v", err)
	}
	if _, err := tmpl.New("content").Parse(`<main class="app"></main>`); err != nil {
		t.Fatalf("define content: %v", err)
	}
	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, "base", map[string]any{}); err != nil {
		t.Fatalf("execute base: %v", err)
	}
	checkFooter(t, "tmpl/base.html", b.String())
}

// TestOfflineFooter keeps the same footer on the offline page, which is served
// as a static file and so does not use the base template.
func TestOfflineFooter(t *testing.T) {
	page, err := embeddedAssets.ReadFile("assets/offline.html")
	if err != nil {
		t.Fatalf("read offline page: %v", err)
	}
	checkFooter(t, "assets/offline.html", string(page))
}

func checkFooter(t *testing.T, name, page string) {
	t.Helper()
	start := strings.Index(page, `<footer class="app-footer">`)
	end := strings.Index(page, `</footer>`)
	if start < 0 || end < start {
		t.Fatalf("%s: no <footer class=\"app-footer\">:\n%s", name, page)
	}
	footer := page[start:end]
	if !strings.Contains(footer, "© 2026 Johan Sundell") {
		t.Errorf("%s: footer lacks copyright notice:\n%s", name, footer)
	}
	for _, link := range footerLinks {
		if !strings.Contains(footer, link) {
			t.Errorf("%s: footer lacks %s:\n%s", name, link, footer)
		}
	}
	if main := strings.LastIndex(page, "</main>"); main > start {
		t.Errorf("%s: footer comes before the end of <main>", name)
	}
}

package router

import (
	"html/template"
	"io/fs"
	"strings"
	"testing"

	"github.com/v03413/bepusdt/static"
)

func TestLangGeCheckoutTemplateIsEmbedded(t *testing.T) {
	checkout, err := readCheckoutInfoFromFS(static.Checkout, "checkout/langge")
	if err != nil {
		t.Fatalf("read langge checkout info: %v", err)
	}

	if checkout.Name != "LangGe design" {
		t.Fatalf("unexpected checkout name: %q", checkout.Name)
	}
	if checkout.Author == "" {
		t.Fatal("checkout author is required")
	}
	if checkout.Desc == "" {
		t.Fatal("checkout desc is required")
	}

	view, err := fs.ReadFile(static.Checkout, "checkout/langge/views/checkout.html")
	if err != nil {
		t.Fatalf("read langge checkout template: %v", err)
	}
	if !strings.Contains(string(view), "{{ .trade_id }}") {
		t.Fatal("langge checkout template must only depend on trade_id server injection")
	}

	tmpl := template.New("default")
	if !registerTemplatesFromFS(tmpl, static.Checkout, "checkout/langge", "langge") {
		t.Fatal("langge checkout template was not registered")
	}
	if tmpl.Lookup("langge/checkout.html") == nil {
		t.Fatal("langge checkout template was not registered under expected name")
	}
}

func TestOfficialCheckoutUsesPipioBrand(t *testing.T) {
	view, err := fs.ReadFile(static.Checkout, "checkout/official/views/checkout.html")
	if err != nil {
		t.Fatalf("read official checkout template: %v", err)
	}

	content := string(view)
	if strings.Contains(strings.ToLower(content), "veilx") {
		t.Fatal("official checkout template must not expose the VeilX brand")
	}
	for _, expected := range []string{
		"Pipio USDT 收银台",
		`/checkout/official/assets/img/pipio.png`,
		`<span class="footer-brand">pipio</span>`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("official checkout template is missing Pipio branding %q", expected)
		}
	}

	asset, err := fs.ReadFile(static.Checkout, "checkout/official/assets/img/pipio.png")
	if err != nil {
		t.Fatalf("read Pipio checkout mark: %v", err)
	}
	if len(asset) == 0 {
		t.Fatal("Pipio checkout mark must not be empty")
	}

	for _, localePath := range []string{
		"checkout/official/assets/locales/en.json",
		"checkout/official/assets/locales/zh.json",
	} {
		locale, err := fs.ReadFile(static.Checkout, localePath)
		if err != nil {
			t.Fatalf("read official checkout locale %s: %v", localePath, err)
		}
		localeContent := strings.ToLower(string(locale))
		if strings.Contains(localeContent, "veilx") {
			t.Fatalf("official checkout locale %s must not expose the VeilX brand", localePath)
		}
		if !strings.Contains(localeContent, "pipio") {
			t.Fatalf("official checkout locale %s must expose the Pipio brand", localePath)
		}
	}
}

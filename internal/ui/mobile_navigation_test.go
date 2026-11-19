package ui

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devSealWare/LightIPAM/internal/store"
)

func TestMobileNavigation(t *testing.T) {
	for _, admin := range []bool{false, true} {
		rendered := httptest.NewRecorder()
		err := Render(rendered, "dashboard.html", PageData{CSRF: "test-token", User: store.User{IsAdmin: admin}})
		if err != nil {
			t.Fatal(err)
		}
		html := rendered.Body.String()
		start := strings.Index(html, `<details data-mobile-menu`)
		if start < 0 {
			t.Fatalf("admin=%v: mobile menu missing", admin)
		}
		end := strings.Index(html[start:], `</details>`)
		if end < 0 {
			t.Fatal("mobile menu does not close")
		}
		menu := html[start : start+end]
		for _, want := range []string{`<summary`, `href="/"`, `href="/subnets"`, `href="/devices"`, `href="/search"`, `href="/discoveries"`, `href="/scans"`, `href="/agents"`, `href="/schedules"`, `href="/account"`, `href="/import"`, `href="/policy"`, `href="/audit"`, `method="post" action="/logout"`, `name="csrf_token" value="test-token"`} {
			if !strings.Contains(menu, want) {
				t.Errorf("admin=%v: mobile menu missing %s", admin, want)
			}
		}
		if got := strings.Contains(menu, `href="/settings"`); got != admin {
			t.Errorf("admin=%v: settings presence=%v", admin, got)
		}
	}
}

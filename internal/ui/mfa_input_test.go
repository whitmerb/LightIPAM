package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestMFACodeKeyboard(t *testing.T) {
	for _, tc := range []struct {
		name        string
		template    string
		data        PageData
		numericOnly bool
	}{
		{name: "sign in accepts recovery codes", template: "mfa_challenge.html"},
		{name: "disable accepts recovery codes", template: "mfa_settings.html", data: PageData{MFAEnabled: true}},
		{name: "enrollment uses TOTP only", template: "mfa_settings.html", numericOnly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := renderToString(t, tc.template, tc.data)
			inputs := regexp.MustCompile(`<input\b[^>]*\bname="code"[^>]*>`).FindAllString(body, -1)
			if len(inputs) != 1 {
				t.Fatalf("got %d code inputs, want 1", len(inputs))
			}
			input := inputs[0]
			if tc.numericOnly {
				if !strings.Contains(input, `inputmode="numeric"`) {
					t.Error("TOTP enrollment should offer a numeric keyboard")
				}
			} else {
				if strings.Contains(input, `inputmode="numeric"`) || strings.Contains(input, `type="number"`) || strings.Contains(input, `inputmode="decimal"`) {
					t.Error("recovery-code entry must offer letters and the separator")
				}
			}
			if !strings.Contains(input, `autocomplete="one-time-code"`) || !strings.Contains(input, "required") {
				t.Error("code entry must retain one-time-code completion and be required")
			}
		})
	}
}

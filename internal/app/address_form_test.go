package app

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/devSealWare/LightIPAM/internal/store"
	"github.com/devSealWare/LightIPAM/internal/ui"
)

func TestAddressFormErrorRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, address, state string
		invalid              bool
	}{
		{"invalid address", "bad<ip", "assigned", true},
		{"invalid state", "192.0.2.5", "unknown", true},
		{"store failure", " 192.0.2.5 ", "assigned", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := url.Values{"address": {tc.address}, "state": {tc.state}, "device_id": {"d1"}, "hostname": {`host"name`}, "notes": {"notes<text"}, "cf_f1": {"new<owner"}}
			r := httptest.NewRequest("POST", "/addresses/a1", strings.NewReader(values.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			_, err := addressInputFromRequest(r)
			if (err != nil) != tc.invalid {
				t.Fatalf("validation error = %v", err)
			}
			form := addressFormFromRequest(r)
			for _, key := range []string{"address", "state", "device_id", "hostname", "notes"} {
				if form[key] != values.Get(key) {
					t.Errorf("%s = %q, want %q", key, form[key], values.Get(key))
				}
			}
			fields := submittedAddressCustomFields([]store.CustomFieldValue{{Def: store.CustomFieldDef{ID: "f1", Name: "Owner"}, Value: "old"}}, r)
			data := ui.PageData{Error: "Unable to save", Form: form, Address: store.IPAddress{ID: "a1"}, Subnet: store.Subnet{ID: "s1"}, AddressStates: addressStates(), Devices: []store.Device{{ID: "d1", Name: "Printer"}}, CustomFields: fields}
			for _, template := range []string{"subnet_detail.html", "address_form.html"} {
				w := httptest.NewRecorder()
				if err := ui.Render(w, template, data); err != nil {
					t.Fatal(err)
				}
				body := w.Body.String()
				for _, want := range []string{`value="d1" selected`, `value="host&#34;name"`, `notes&lt;text`} {
					if !strings.Contains(body, want) {
						t.Errorf("%s missing %q", template, want)
					}
				}
				if tc.state == "assigned" && !strings.Contains(body, `value="assigned" selected`) {
					t.Errorf("%s did not select submitted state", template)
				}
				if strings.Contains(body, `value="unknown"`) {
					t.Errorf("%s introduced invalid state", template)
				}
				if template == "address_form.html" && !strings.Contains(body, `value="new&lt;owner"`) {
					t.Error("custom field edit lost")
				}
			}
		})
	}
}

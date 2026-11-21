package main

import (
	"reflect"
	"testing"
)

func TestParseFields(t *testing.T) {
	strFields := []string{"cidr", "name", "site_id", "description"}
	intFields := []string{"vlan"}

	t.Run("create includes set fields with types", func(t *testing.T) {
		body, err := parseFields("subnets create", []string{"--cidr", "10.0.0.0/24", "--name", "Core", "--vlan", "100"}, strFields, intFields)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := map[string]any{"cidr": "10.0.0.0/24", "name": "Core", "vlan": 100}
		if !reflect.DeepEqual(body, want) {
			t.Fatalf("body = %#v, want %#v", body, want)
		}
	})

	t.Run("update is partial — only set flags", func(t *testing.T) {
		body, err := parseFields("subnets update", []string{"--name", "Renamed"}, strFields, intFields)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(body, map[string]any{"name": "Renamed"}) {
			t.Fatalf("body = %#v, want only name", body)
		}
	})

	t.Run("no flags is an error", func(t *testing.T) {
		if _, err := parseFields("subnets create", nil, strFields, intFields); err == nil {
			t.Fatal("expected an error when no field flags are set")
		}
	})

	t.Run("unknown flag is an error", func(t *testing.T) {
		if _, err := parseFields("subnets create", []string{"--bogus", "x"}, strFields, intFields); err == nil {
			t.Fatal("expected an error for an unknown flag")
		}
	})
}

func TestParseFieldsIDFlagAliases(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		args        []string
		want        string
	}{
		{"site hyphen", "site_id", []string{"--site-id", "site-1"}, "site-1"},
		{"site underscore", "site_id", []string{"--site_id", "site-1"}, "site-1"},
		{"device hyphen", "device_id", []string{"--device-id", "device-1"}, "device-1"},
		{"device underscore", "device_id", []string{"--device_id", "device-1"}, "device-1"},
		{"empty value", "site_id", []string{"--site-id", ""}, ""},
		{"last alias wins", "device_id", []string{"--device-id", "first", "--device_id", "last"}, "last"},
		{"last hyphen wins", "device_id", []string{"--device_id", "first", "--device-id", "last"}, "last"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := parseFields("test", tc.args, []string{tc.field}, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := map[string]any{tc.field: tc.want}; !reflect.DeepEqual(body, want) {
				t.Fatalf("body = %#v, want %#v", body, want)
			}
		})
	}
}

func TestAPIErrorMessage(t *testing.T) {
	if got := apiErrorMessage([]byte(`{"error":"nope"}`)); got != "nope" {
		t.Fatalf("apiErrorMessage = %q", got)
	}
	if got := apiErrorMessage([]byte(`plain text`)); got != "plain text" {
		t.Fatalf("apiErrorMessage fallback = %q", got)
	}
}

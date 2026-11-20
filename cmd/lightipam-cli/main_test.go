package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestUpdatePreservesUnspecifiedFields(t *testing.T) {
	cases := []struct {
		name, resource, current, want string
		flags                         []string
	}{
		{
			name: "subnet description", resource: "subnets",
			current: `{"id":"record","cidr":"10.0.0.0/24","name":"Core","site_id":"hq","vlan":100,"description":"old","address_count":2}`,
			flags:   []string{"--description", "new"},
			want:    `{"cidr":"10.0.0.0/24","name":"Core","site_id":"hq","vlan":100,"description":"new"}`,
		},
		{
			name: "address state", resource: "addresses",
			current: `{"id":"record","subnet_id":"subnet","address":"10.0.0.5","state":"assigned","hostname":"router","device_id":"device","notes":"keep"}`,
			flags:   []string{"--state", "reserved"},
			want:    `{"address":"10.0.0.5","state":"reserved","hostname":"router","device_id":"device","notes":"keep"}`,
		},
		{
			name: "device description", resource: "devices",
			current: `{"id":"record","name":"router","description":"old"}`,
			flags:   []string{"--description", "new"},
			want:    `{"name":"router","description":"new"}`,
		},
		{
			name: "device name", resource: "devices",
			current: `{"id":"record","name":"router","description":"keep"}`,
			flags:   []string{"--name", "renamed"},
			want:    `{"name":"renamed","description":"keep"}`,
		},
		{
			name: "clear device description", resource: "devices",
			current: `{"id":"record","name":"router","description":"old"}`,
			flags:   []string{"--description", ""},
			want:    `{"name":"router","description":""}`,
		},
		{
			name: "clear address fields", resource: "addresses",
			current: `{"id":"record","subnet_id":"subnet","address":"10.0.0.5","state":"reserved","hostname":"router","device_id":"device","notes":"old"}`,
			flags:   []string{"--hostname", "", "--device_id", "", "--notes", ""},
			want:    `{"address":"10.0.0.5","state":"reserved","hostname":"","device_id":"","notes":""}`,
		},
		{
			name: "preserve unset VLAN and clear description", resource: "subnets",
			current: `{"id":"record","cidr":"10.0.0.0/24","name":"Core","site_id":"default","vlan":null,"description":"old","address_count":2}`,
			flags:   []string{"--description", ""},
			want:    `{"cidr":"10.0.0.0/24","name":"Core","site_id":"default","vlan":null,"description":""}`,
		},
		{
			name: "change VLAN", resource: "subnets",
			current: `{"id":"record","cidr":"10.0.0.0/24","name":"Core","site_id":"hq","vlan":100,"description":"keep"}`,
			flags:   []string{"--vlan", "200"},
			want:    `{"cidr":"10.0.0.0/24","name":"Core","site_id":"hq","vlan":200,"description":"keep"}`,
		},
		{
			name: "choose site for unassigned subnet", resource: "subnets",
			current: `{"id":"record","cidr":"10.0.0.0/24","name":"Core","site_id":"","vlan":null,"description":"keep"}`,
			flags:   []string{"--site_id", "hq"},
			want:    `{"cidr":"10.0.0.0/24","name":"Core","site_id":"hq","vlan":null,"description":"keep"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan string, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Method
				if r.URL.Path != "/api/v1/"+tc.resource+"/record" || r.Header.Get("Authorization") != "Bearer secret" {
					t.Errorf("unexpected request: %s %s, authorization %q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
				}
				switch r.Method {
				case http.MethodGet:
					w.Write([]byte(tc.current))
				case http.MethodPut:
					var got, want map[string]any
					if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
						t.Errorf("decode PUT: %v", err)
					}
					if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
						t.Error(err)
						return
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("PUT body = %#v, want %#v", got, want)
						http.Error(w, `{"error":"incomplete replacement"}`, http.StatusBadRequest)
						return
					}
					w.Write([]byte(tc.want))
				default:
					t.Errorf("unexpected method %s", r.Method)
				}
			}))
			defer server.Close()
			c := &client{baseURL: server.URL, token: "secret", http: server.Client()}
			if err := updateResource(c, tc.resource, tc.flags); err != nil {
				t.Fatalf("update: %v", err)
			}
			for _, want := range []string{http.MethodGet, http.MethodPut} {
				select {
				case got := <-requests:
					if got != want {
						t.Errorf("request method = %s, want %s", got, want)
					}
				default:
					t.Errorf("missing %s request", want)
				}
			}
		})
	}
}

func updateResource(c *client, resource string, flags []string) error {
	args := append([]string{"update", "record"}, flags...)
	switch resource {
	case "subnets":
		return c.resource(args, resource, []string{"cidr", "name", "site_id", "description"}, []string{"vlan"})
	case "addresses":
		return c.addresses(args)
	default:
		return c.resource(args, resource, []string{"name", "description"}, nil)
	}
}

func TestUpdateDoesNotWriteAfterReadFailure(t *testing.T) {
	cases := []struct {
		name, resource, body string
		status               int
	}{
		{"unauthorized", "devices", `{"error":"invalid token"}`, http.StatusUnauthorized},
		{"forbidden", "devices", `{"error":"forbidden"}`, http.StatusForbidden},
		{"not found", "devices", `{"error":"not found"}`, http.StatusNotFound},
		{"server error", "devices", `{"error":"unavailable"}`, http.StatusInternalServerError},
		{"malformed JSON", "devices", `{"name":`, http.StatusOK},
		{"trailing JSON", "devices", `{"name":"router","description":"keep"} {}`, http.StatusOK},
		{"null object", "devices", `null`, http.StatusOK},
		{"array", "devices", `[]`, http.StatusOK},
		{"missing field", "devices", `{"name":"router"}`, http.StatusOK},
		{"null string", "devices", `{"name":null,"description":"keep"}`, http.StatusOK},
		{"wrong string type", "devices", `{"name":12,"description":"keep"}`, http.StatusOK},
		{"wrong VLAN type", "subnets", `{"cidr":"10.0.0.0/24","name":"Core","site_id":"default","vlan":"100","description":"keep"}`, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("read failure must not send %s", r.Method)
				}
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer server.Close()
			c := &client{baseURL: server.URL, http: server.Client()}
			if err := updateResource(c, tc.resource, []string{"--description", "new"}); err == nil {
				t.Fatal("expected read error")
			}
		})
	}
	t.Run("transport failure", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()
		c := &client{baseURL: server.URL, http: server.Client()}
		if err := updateResource(c, "devices", []string{"--description", "new"}); err == nil {
			t.Fatal("expected transport error")
		}
	})
}

func TestUpdateReportsWriteFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"name":"router","description":"keep"}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"read-only token"}`))
	}))
	defer server.Close()
	c := &client{baseURL: server.URL, http: server.Client()}
	err := updateResource(c, "devices", []string{"--name", "renamed"})
	if err == nil || err.Error() != "PUT /api/v1/devices/record: HTTP 403: read-only token" {
		t.Fatalf("update error = %v, want API write error", err)
	}
}

func TestUpdateRequiresExplicitSiteForUnassignedSubnet(t *testing.T) {
	for _, flags := range [][]string{
		{"--description", "new"},
		{"--site_id", ""},
		{"--site_id", " "},
	} {
		t.Run(flags[0]+"="+flags[1], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("must not send %s when unset site cannot be preserved", r.Method)
				}
				w.Write([]byte(`{"cidr":"10.0.0.0/24","name":"Core","site_id":"","vlan":null,"description":"keep"}`))
			}))
			defer server.Close()
			c := &client{baseURL: server.URL, http: server.Client()}
			err := updateResource(c, "subnets", flags)
			if err == nil || err.Error() != "GET /api/v1/subnets/record: the API cannot preserve an unset site; choose a site with --site_id before updating" {
				t.Fatalf("update error = %v, want explicit-site guidance", err)
			}
		})
	}
}

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

func TestAPIErrorMessage(t *testing.T) {
	if got := apiErrorMessage([]byte(`{"error":"nope"}`)); got != "nope" {
		t.Fatalf("apiErrorMessage = %q", got)
	}
	if got := apiErrorMessage([]byte(`plain text`)); got != "plain text" {
		t.Fatalf("apiErrorMessage fallback = %q", got)
	}
}

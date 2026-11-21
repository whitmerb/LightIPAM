package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devSealWare/LightIPAM/internal/auth"
	"github.com/devSealWare/LightIPAM/internal/db"
	"github.com/devSealWare/LightIPAM/internal/secret"
	"github.com/devSealWare/LightIPAM/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLoginMFAEnrollmentLookup(t *testing.T) {
	if os.Getenv("LIGHTIPAM_TEST_DATABASE_URL") == "" {
		t.Skip("set LIGHTIPAM_TEST_DATABASE_URL to run PostgreSQL login integration tests")
	}
	passwordHash, err := auth.HashPassword("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"absent", "pending", "enabled", "unavailable", "no-sealer"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			pool := newLoginTestPool(t, ctx)
			if _, err := pool.Exec(ctx, `INSERT INTO users (id, username, display_name, password_hash, role) VALUES ('login-user', 'login-user', 'Login User', $1, 'admin')`, passwordHash); err != nil {
				t.Fatal(err)
			}
			st := store.New(pool)
			if mode != "absent" {
				if err := st.StartTOTPEnrollment(ctx, "login-user", "sealed-fixture"); err != nil {
					t.Fatal(err)
				}
				if mode != "pending" {
					if err := st.EnableTOTP(ctx, "login-user", nil); err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "unavailable" {
				// Break only the MFA lookup; password, session and audit writes
				// remain available, reproducing a fail-open login deterministically.
				if _, err := pool.Exec(ctx, `ALTER TABLE user_totp RENAME TO unavailable_totp`); err != nil {
					t.Fatal(err)
				}
			}
			sealer, err := secret.NewSealer(secret.DeriveKey([]byte("login-fixture")))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "no-sealer" {
				sealer = nil
			}
			a := &App{store: st, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sealer: sealer,
				settings: SecuritySettings{LoginMaxAttempts: 5, LoginWindow: time.Minute, LoginLockout: time.Minute, SessionAbsoluteTimeout: time.Hour}}
			form := url.Values{"username": {"login-user"}, "password": {"fixture-password"}, "csrf_token": {"login-csrf"}}
			r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.AddCookie(&http.Cookie{Name: csrfCookie, Value: "login-csrf"})
			w := httptest.NewRecorder()
			a.loginSubmit(w, r)

			wantSessions, wantAudit, wantLocation := 0, 0, ""
			switch mode {
			case "absent", "pending":
				wantSessions, wantAudit, wantLocation = 1, 1, "/"
			case "enabled":
				wantLocation = "/login/mfa"
			default:
				if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Unable to start two-factor verification. Please try again.") {
					t.Errorf("login must fail closed: status=%d location=%q", w.Code, w.Header().Get("Location"))
				}
			}
			if got := w.Header().Get("Location"); got != wantLocation {
				t.Errorf("redirect = %q, want %q", got, wantLocation)
			}
			if wantLocation != "" && w.Code != http.StatusSeeOther {
				t.Errorf("redirect status = %d", w.Code)
			}
			cookies := map[string]*http.Cookie{}
			for _, c := range w.Result().Cookies() {
				cookies[c.Name] = c
			}
			if (cookies[sessionCookie] != nil) != (wantSessions == 1) {
				t.Errorf("session cookie present = %v", cookies[sessionCookie] != nil)
			}
			if (cookies[mfaCookie] != nil) != (mode == "enabled") {
				t.Errorf("pending MFA cookie present = %v", cookies[mfaCookie] != nil)
			}
			for _, check := range []struct {
				query string
				want  int
			}{{`SELECT count(*) FROM sessions`, wantSessions}, {`SELECT count(*) FROM audit_logs WHERE action = 'auth.login'`, wantAudit}} {
				var got int
				if err := pool.QueryRow(ctx, check.query).Scan(&got); err != nil {
					t.Fatal(err)
				}
				if got != check.want {
					t.Errorf("%s = %d, want %d", check.query, got, check.want)
				}
			}
		})
	}
}

func newLoginTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	admin, err := pgxpool.New(ctx, os.Getenv("LIGHTIPAM_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("lightipam_login_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop login test schema: %v", err)
		}
	})
	cfg, err := pgxpool.ParseConfig(os.Getenv("LIGHTIPAM_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

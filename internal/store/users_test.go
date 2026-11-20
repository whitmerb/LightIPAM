package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLastAdminWrites(t *testing.T) {
	for _, kind := range []string{"demote", "delete"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			st, pool := newIntegrationStore(t, ctx)
			mustExec(t, ctx, pool, `INSERT INTO users (id, username, display_name, password_hash, role, is_admin) VALUES ('admin', 'admin', 'Admin', 'hash', 'admin', true), ('viewer', 'viewer', 'Viewer', 'hash', 'viewer', false)`)
			if err := removeAdmin(ctx, st, kind, "admin"); !errors.Is(err, ErrLastAdmin) {
				t.Fatalf("removing the sole admin: got %v, want ErrLastAdmin", err)
			}
			assertCount(t, ctx, pool, `SELECT count(*) FROM users WHERE is_admin AND role='admin'`, 1)
			if err := st.SetUserRole(ctx, "viewer", RoleAdmin); err != nil {
				t.Fatal(err)
			}
			if err := removeAdmin(ctx, st, kind, "admin"); err != nil {
				t.Fatalf("removal after promotion: %v", err)
			}
			if err := st.SetUserRole(ctx, "viewer", RoleAdmin); err != nil {
				t.Fatalf("idempotent admin role: %v", err)
			}
			if err := removeAdmin(ctx, st, kind, "missing"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("missing target: got %v, want ErrNotFound", err)
			}
			assertCount(t, ctx, pool, `SELECT count(*) FROM users WHERE is_admin <> (role='admin')`, 0)
		})
	}
}

func TestConcurrentLastAdminWrites(t *testing.T) {
	for _, first := range []string{"demote", "delete"} {
		for _, second := range []string{"demote", "delete"} {
			t.Run(first+"/"+second, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				st, pool := newIntegrationStore(t, ctx)
				mustExec(t, ctx, pool, `INSERT INTO users (id, username, display_name, password_hash, role, is_admin) VALUES ('a', 'a', 'A', 'hash', 'admin', true), ('b', 'b', 'B', 'hash', 'admin', true)`)
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1279872333, 2)`); err != nil {
					t.Fatal(err)
				}
				results := make(chan error, 2)
				go func() { results <- removeAdmin(ctx, st, first, "a") }()
				go func() { results <- removeAdmin(ctx, st, second, "b") }()
				waitForAdminWriters(t, ctx, pool, results, 2)
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				successes, protected := 0, 0
				for range 2 {
					if err := <-results; err == nil {
						successes++
					} else if errors.Is(err, ErrLastAdmin) {
						protected++
					} else {
						t.Fatalf("unexpected removal error: %v", err)
					}
				}
				if successes != 1 || protected != 1 {
					t.Fatalf("successful removals = %d, protected = %d; want 1 each", successes, protected)
				}
				assertCount(t, ctx, pool, `SELECT count(*) FROM users WHERE is_admin`, 1)
				assertCount(t, ctx, pool, `SELECT count(*) FROM users WHERE is_admin <> (role='admin')`, 0)
			})
		}
	}
}

func TestLastAdminFreshRoleAndCancellation(t *testing.T) {
	for _, kind := range []string{"demote", "delete", "promote"} {
		for _, cancelWrite := range []bool{false, true} {
			name := kind + "/fresh"
			if cancelWrite {
				name = kind + "/cancel"
			}
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				st, pool := newIntegrationStore(t, ctx)
				mustExec(t, ctx, pool, `INSERT INTO users (id, username, display_name, password_hash, role, is_admin) VALUES ('target', 'target', 'Target', 'hash', 'viewer', false)`)
				// Exercise a pool whose default would otherwise keep a snapshot
				// from before the lock holder committed its promotion.
				config := pool.Config()
				config.ConnConfig.RuntimeParams["default_transaction_isolation"] = "repeatable read"
				repeatable, err := pgxpool.NewWithConfig(ctx, config)
				if err != nil {
					t.Fatal(err)
				}
				defer repeatable.Close()
				st = New(repeatable)
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1279872333, 2)`); err != nil {
					t.Fatal(err)
				}
				writeCtx, cancelOp := context.WithCancel(ctx)
				defer cancelOp()
				results := make(chan error, 1)
				go func() { results <- removeAdmin(writeCtx, st, kind, "target") }()
				waitForAdminWriters(t, ctx, pool, results, 1)
				if cancelWrite {
					cancelOp()
					if err := <-results; !errors.Is(err, context.Canceled) {
						t.Fatalf("cancelled operation: got %v, want context.Canceled", err)
					}
					if err := tx.Rollback(ctx); err != nil {
						t.Fatal(err)
					}
					assertCount(t, ctx, pool, `SELECT count(*) FROM users WHERE id='target' AND role='viewer' AND NOT is_admin`, 1)
					if err := st.SetUserRole(ctx, "target", RoleAdmin); err != nil {
						t.Fatalf("write after cancellation: %v", err)
					}
					return
				}
				if _, err := tx.Exec(ctx, `UPDATE users SET role='admin', is_admin=true WHERE id='target'`); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				err = <-results
				if kind == "promote" && err != nil {
					t.Fatalf("promotion after waiting: %v", err)
				} else if kind != "promote" && !errors.Is(err, ErrLastAdmin) {
					t.Fatalf("removing the newly promoted sole admin: got %v, want ErrLastAdmin", err)
				}
				assertCount(t, ctx, pool, `SELECT count(*) FROM users WHERE id='target' AND role='admin' AND is_admin`, 1)
			})
		}
	}
}

func TestUserRoleAndViewerDeletion(t *testing.T) {
	ctx := context.Background()
	st, pool := newIntegrationStore(t, ctx)
	mustExec(t, ctx, pool, `INSERT INTO users (id, username, display_name, password_hash, role, is_admin) VALUES ('admin', 'admin', 'Admin', 'hash', 'admin', true), ('viewer', 'viewer', 'Viewer', 'hash', 'viewer', false)`)
	if err := st.SetUserRole(ctx, "viewer", "invalid"); err == nil {
		t.Fatal("invalid role accepted")
	}
	assertCount(t, ctx, pool, `SELECT count(*) FROM users WHERE id='viewer' AND role='viewer' AND NOT is_admin`, 1)
	if err := st.SetUserRole(ctx, "viewer", RoleViewer); err != nil {
		t.Fatalf("idempotent viewer role: %v", err)
	}
	if err := st.DeleteUser(ctx, "viewer"); err != nil {
		t.Fatalf("delete viewer: %v", err)
	}
	assertCount(t, ctx, pool, `SELECT count(*) FROM users WHERE id='admin' AND is_admin`, 1)
}

func removeAdmin(ctx context.Context, st *Store, kind, id string) error {
	if kind == "delete" {
		return st.DeleteUser(ctx, id)
	}
	role := RoleViewer
	if kind == "promote" {
		role = RoleAdmin
	}
	return st.SetUserRole(ctx, id, role)
}

func waitForAdminWriters(t *testing.T, ctx context.Context, pool *pgxpool.Pool, results <-chan error, want int) {
	t.Helper()
	for {
		var waiting int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND classid=1279872333 AND objid=2 AND NOT granted AND database=(SELECT oid FROM pg_database WHERE datname=current_database())`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting >= want {
			return
		}
		select {
		case err := <-results:
			t.Fatalf("operation bypassed admin lock: %v", err)
		case <-ctx.Done():
			t.Fatal("timed out waiting for admin lock")
		case <-time.After(time.Millisecond):
		}
	}
}

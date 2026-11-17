package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConsumeRecoveryCodeConcurrentSingleUse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	st, pool := newIntegrationStore(t, ctx)
	user, err := st.CreateUser(ctx, "recovery-test", "Recovery Test", "unused", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, ctx, pool, `INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, 'recovery-hash')`, user.ID)

	// Keep the first consumption uncommitted so the competing statement sees
	// an unused code in its snapshot, then waits for this row's write lock.
	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback(context.Background())
	var blockerPID uint32
	if err := first.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Exec(ctx, `UPDATE user_recovery_codes SET used_at = now() WHERE user_id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}

	// A dedicated one-connection pool identifies exactly the backend running
	// ConsumeRecoveryCode, without changing the production store interface.
	config := pool.Config()
	config.MaxConns = 1
	contender, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer contender.Close()
	var contenderPID uint32
	if err := contender.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&contenderPID); err != nil {
		t.Fatal(err)
	}
	type result struct {
		consumed bool
		err      error
	}
	done := make(chan result, 1)
	go func() {
		consumed, err := New(contender).ConsumeRecoveryCode(ctx, user.ID, "recovery-hash")
		done <- result{consumed, err}
	}()

	// Observe the actual database wait before committing; elapsed time alone
	// cannot establish that both statements have seen the unused version.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := pool.QueryRow(ctx, `SELECT $1::integer = ANY(pg_blocking_pids($2::integer))`, blockerPID, contenderPID).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case got := <-done:
			t.Fatalf("competing consumption finished before waiting: %+v", got)
		case <-ctx.Done():
			t.Fatal("competing consumption never blocked on the first transaction")
		case <-ticker.C:
		}
	}
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil {
		t.Fatalf("competing consumption: %v", got.err)
	}
	if got.consumed {
		t.Fatal("recovery code was consumed twice by concurrent transactions")
	}
	if consumed, err := st.ConsumeRecoveryCode(ctx, user.ID, "recovery-hash"); err != nil || consumed {
		t.Fatalf("subsequent consumption = %v, %v; want false, nil", consumed, err)
	}
	mustExec(t, ctx, pool, `INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, 'fresh-hash')`, user.ID)
	for _, attempt := range []struct{ userID, hash string }{
		{"another-user", "fresh-hash"},
		{user.ID, "wrong-hash"},
	} {
		if consumed, err := st.ConsumeRecoveryCode(ctx, attempt.userID, attempt.hash); err != nil || consumed {
			t.Fatalf("mismatched consumption = %v, %v; want false, nil", consumed, err)
		}
	}
	if consumed, err := st.ConsumeRecoveryCode(ctx, user.ID, "fresh-hash"); err != nil || !consumed {
		t.Fatalf("fresh code consumption = %v, %v; want true, nil", consumed, err)
	}
	if consumed, err := st.ConsumeRecoveryCode(ctx, user.ID, "fresh-hash"); err != nil || consumed {
		t.Fatalf("repeated consumption = %v, %v; want false, nil", consumed, err)
	}
}

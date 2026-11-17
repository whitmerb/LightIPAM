package store

import (
	"context"
	"sync"
	"testing"
)

func TestLinkOIDCSubject(t *testing.T) {
	ctx := context.Background()
	st, pool := newIntegrationStore(t, ctx)
	mustExec(t, ctx, pool, `INSERT INTO users (id, username, display_name, password_hash) VALUES ('oidc-user', 'oidc-user', 'OIDC User', 'hash'), ('race-user', 'race-user', 'Race User', 'hash')`)

	if err := st.LinkOIDCSubject(ctx, "oidc-user", "subject-a"); err != nil {
		t.Fatalf("first link: %v", err)
	}
	if err := st.LinkOIDCSubject(ctx, "oidc-user", "subject-a"); err != nil {
		t.Fatalf("same subject link: %v", err)
	}
	if err := st.LinkOIDCSubject(ctx, "oidc-user", "subject-b"); err == nil {
		t.Error("different subject link succeeded")
	}
	var subject string
	if err := pool.QueryRow(ctx, `SELECT oidc_subject FROM users WHERE id = 'oidc-user'`).Scan(&subject); err != nil || subject != "subject-a" {
		t.Errorf("subject after rejected link = %q, err = %v; want subject-a", subject, err)
	}
	if err := st.LinkOIDCSubject(ctx, "missing-user", "subject-c"); err == nil {
		t.Error("link to missing user succeeded")
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, candidate := range []string{"race-a", "race-b"} {
		wg.Add(1)
		go func(candidate string) {
			defer wg.Done()
			<-start
			errs <- st.LinkOIDCSubject(ctx, "race-user", candidate)
		}(candidate)
	}
	close(start)
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("competing links: %d successes, want 1", successes)
	}
}

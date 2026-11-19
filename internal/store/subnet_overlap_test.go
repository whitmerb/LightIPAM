package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSubnetImportRechecksOverlap(t *testing.T) {
	for _, cidr := range []string{"10.0.0.0/23", "10.0.0.0/25"} {
		t.Run(cidr, func(t *testing.T) {
			ctx := context.Background()
			st, pool := newIntegrationStore(t, ctx)
			// A preview can see no conflict before another writer creates a subnet.
			if _, err := st.ListSubnets(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := st.CreateSubnet(ctx, SubnetInput{SiteID: "default", CIDR: "10.0.0.0/24", Name: "Original"}); err != nil {
				t.Fatal(err)
			}
			err := st.ImportSubnets(ctx, []SubnetImport{
				{SiteID: "default", CIDR: "10.0.0.0/24", Name: "Changed"},
				{SiteID: "default", CIDR: "192.0.2.0/24", Name: "Safe"},
				{SiteID: "default", CIDR: cidr, Name: "Stale"},
			})
			if !errors.Is(err, ErrOverlap) {
				t.Fatalf("stale import returned %v, want ErrOverlap", err)
			}
			assertCount(t, ctx, pool, `SELECT count(*) FROM subnets`, 1)
			assertCount(t, ctx, pool, `SELECT count(*) FROM subnets WHERE cidr='10.0.0.0/24' AND name='Original'`, 1)
		})
	}
}

func TestSubnetImportRejectsBatchOverlap(t *testing.T) {
	ctx := context.Background()
	st, pool := newIntegrationStore(t, ctx)
	err := st.ImportSubnets(ctx, []SubnetImport{
		{SiteID: "default", CIDR: "10.0.0.0/24", Name: "First"},
		{SiteID: "default", CIDR: "10.0.0.0/25", Name: "Overlap"},
	})
	if !errors.Is(err, ErrOverlap) {
		t.Fatalf("overlapping batch returned %v, want ErrOverlap", err)
	}
	assertCount(t, ctx, pool, `SELECT count(*) FROM subnets`, 0)
}

func TestSubnetImportPreservesExactCIDRUpsert(t *testing.T) {
	ctx := context.Background()
	st, pool := newIntegrationStore(t, ctx)
	mustExec(t, ctx, pool, `INSERT INTO subnets (id, site_id, cidr, name) VALUES ('original', 'default', '10.0.0.0/24', 'Original')`)
	mustExec(t, ctx, pool, `INSERT INTO ip_addresses (id, subnet_id, address, state) VALUES ('ip', 'original', '10.0.0.10', 'assigned')`)
	vlan := 42
	err := st.ImportSubnets(ctx, []SubnetImport{
		{SiteID: "default", CIDR: "10.0.0.0/24", Name: "Updated", VLAN: &vlan, Description: "Updated description"},
		{SiteID: "default", CIDR: "10.0.1.0/24", Name: "Adjacent"},
		{SiteID: "default", CIDR: "192.0.2.0/31", Name: "Point to point"},
		{SiteID: "default", CIDR: "192.0.2.2/32", Name: "Host"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, ctx, pool, `SELECT count(*) FROM subnets`, 4)
	assertCount(t, ctx, pool, `SELECT count(*) FROM subnets WHERE id='original' AND name='Updated' AND vlan=42 AND description='Updated description'`, 1)
	assertCount(t, ctx, pool, `SELECT count(*) FROM ip_addresses WHERE id='ip' AND subnet_id='original'`, 1)
	assertCount(t, ctx, pool, `SELECT count(*) FROM ip_addresses`, 1)
}

func TestSubnetOverlapLockOrdering(t *testing.T) {
	for _, first := range []string{"create", "update", "import"} {
		for _, second := range []string{"create", "update", "import"} {
			t.Run(first+"/"+second, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				st, pool := newIntegrationStore(t, ctx)
				mustExec(t, ctx, pool, `INSERT INTO subnets (id, site_id, cidr, name) VALUES ('first', 'default', '10.1.0.0/24', 'First'), ('second', 'default', '10.2.0.0/24', 'Second')`)
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				// Queue both writers behind a reader, in a known order. Neither
				// may check overlap until it owns the exclusive transaction lock.
				if err := lockSubnetContainment(ctx, tx, false); err != nil {
					t.Fatal(err)
				}
				var holderPID int
				if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
					t.Fatal(err)
				}
				winner := make(chan error, 1)
				go func() { winner <- writeOverlapSubnet(ctx, st, first, "first", "10.0.0.0/24") }()
				waitForSubnetOverlapWriters(t, ctx, pool, holderPID, 1, winner)
				loser := make(chan error, 1)
				go func() { loser <- writeOverlapSubnet(ctx, st, second, "second", "10.0.0.0/25") }()
				waitForSubnetOverlapWriters(t, ctx, pool, holderPID, 2, winner, loser)
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				if err := <-winner; err != nil {
					t.Fatalf("first writer: %v", err)
				}
				if err := <-loser; !errors.Is(err, ErrOverlap) {
					t.Fatalf("second writer returned %v, want ErrOverlap", err)
				}
				assertCount(t, ctx, pool, `SELECT count(*) FROM subnets a JOIN subnets b ON a.id < b.id AND a.cidr && b.cidr`, 0)
				assertCount(t, ctx, pool, `SELECT count(*) FROM subnets WHERE cidr='10.0.0.0/24'`, 1)
				assertCount(t, ctx, pool, `SELECT count(*) FROM ip_addresses`, 0)
			})
		}
	}
}

func TestSubnetOverlapLockCancellation(t *testing.T) {
	for _, kind := range []string{"create", "update", "import"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			st, pool := newIntegrationStore(t, ctx)
			mustExec(t, ctx, pool, `INSERT INTO subnets (id, site_id, cidr, name) VALUES ('subnet', 'default', '10.1.0.0/24', 'Original')`)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if err := lockSubnetContainment(ctx, tx, false); err != nil {
				t.Fatal(err)
			}
			var holderPID int
			if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			writerCtx, cancelWriter := context.WithCancel(ctx)
			defer cancelWriter()
			result := make(chan error, 1)
			go func() { result <- writeOverlapSubnet(writerCtx, st, kind, "subnet", "10.0.0.0/24") }()
			waitForSubnetOverlapWriters(t, ctx, pool, holderPID, 1, result)
			cancelWriter()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled writer returned %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("canceled writer did not return while the lock was held")
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			assertCount(t, ctx, pool, `SELECT count(*) FROM subnets WHERE id='subnet' AND cidr='10.1.0.0/24' AND name='Original'`, 1)
			assertCount(t, ctx, pool, `SELECT count(*) FROM subnets`, 1)
			if err := writeOverlapSubnet(ctx, st, kind, "subnet", "10.0.0.0/24"); err != nil {
				t.Fatalf("writer after cancellation: %v", err)
			}
		})
	}
}

func writeOverlapSubnet(ctx context.Context, st *Store, kind, id, cidr string) error {
	input := SubnetInput{SiteID: "default", CIDR: cidr, Name: "Changed"}
	switch kind {
	case "create":
		_, err := st.CreateSubnet(ctx, input)
		return err
	case "update":
		_, err := st.UpdateSubnet(ctx, id, input)
		return err
	default:
		return st.ImportSubnets(ctx, []SubnetImport{{SiteID: "default", CIDR: cidr, Name: input.Name}})
	}
}

func waitForSubnetOverlapWriters(t *testing.T, ctx context.Context, pool *pgxpool.Pool, holderPID, want int, results ...chan error) {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND classid=1279872333 AND objid=1 AND NOT granted AND $1 = ANY(pg_blocking_pids(pid))`, holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting == want {
			return
		}
		for _, result := range results {
			select {
			case err := <-result:
				t.Fatalf("writer did not wait for exclusive advisory lock: %v", err)
			default:
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %d subnet writers", want)
		case <-ticker.C:
		}
	}
}

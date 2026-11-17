package store

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestSubnetEditRejectsStrandedAddresses(t *testing.T) {
	ctx := context.Background()
	st, pool := newIntegrationStore(t, ctx)
	mustExec(t, ctx, pool, `INSERT INTO subnets (id, site_id, cidr, name) VALUES ('subnet', 'default', '10.0.0.0/24', 'LAN')`)
	mustExec(t, ctx, pool, `INSERT INTO ip_addresses (id, subnet_id, address, state) VALUES ('ip', 'subnet', '10.0.0.200', 'assigned')`)
	_, err := st.UpdateSubnet(ctx, "subnet", SubnetInput{SiteID: "default", CIDR: "10.0.0.0/25", Name: "Changed", Description: "Changed"})
	if !errors.Is(err, ErrSubnetContainsAddresses) {
		t.Fatalf("subnet edit returned %v, want stranded-address rejection", err)
	}
	assertCount(t, ctx, pool, `SELECT count(*) FROM subnets WHERE id='subnet' AND cidr='10.0.0.0/24' AND name='LAN' AND description=''`, 1)
}

func TestSubnetContainmentAndStaleWriters(t *testing.T) {
	ctx := context.Background()
	st, pool := newIntegrationStore(t, ctx)
	mustExec(t, ctx, pool, `INSERT INTO subnets (id, site_id, cidr, name) VALUES ('subnet', 'default', '10.0.0.0/24', 'LAN'), ('empty', 'default', '10.1.0.0/24', 'Empty')`)
	mustExec(t, ctx, pool, `INSERT INTO ip_addresses (id, subnet_id, address, state) VALUES ('ip', 'subnet', '10.0.0.10', 'assigned')`)
	update := func(id, cidr string) error {
		_, err := st.UpdateSubnet(ctx, id, SubnetInput{SiteID: "default", CIDR: cidr, Name: "LAN"})
		return err
	}
	for _, cidr := range []string{"10.0.0.0/23", "10.0.0.0/25"} {
		if err := update("subnet", cidr); err != nil {
			t.Fatalf("valid edit %s: %v", cidr, err)
		}
	}
	if err := update("empty", "10.2.0.0/24"); err != nil {
		t.Fatalf("empty move: %v", err)
	}
	stale := Subnet{ID: "subnet", CIDR: "10.0.0.0/24"}
	if _, err := st.CreateAddress(ctx, stale, AddressInput{Address: "10.0.0.200", State: "assigned"}); !errors.Is(err, ErrAddressOutOfCIDR) {
		t.Fatalf("stale create: %v", err)
	}
	if _, err := st.UpdateAddress(ctx, "ip", stale, AddressInput{Address: "10.0.0.200", State: "assigned"}); !errors.Is(err, ErrAddressOutOfCIDR) {
		t.Fatalf("stale update: %v", err)
	}
	mustExec(t, ctx, pool, `UPDATE ip_addresses SET subnet_id='empty', address='10.2.0.10' WHERE id='ip'`)
	if _, err := st.UpdateAddress(ctx, "ip", stale, AddressInput{Address: "10.0.0.10", State: "assigned"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reassigned update: %v", err)
	}
	err := st.ImportAddresses(ctx, []AddressImport{{Address: "10.0.0.20", SubnetID: "subnet", State: "assigned"}, {Address: "10.0.0.200", SubnetID: "subnet", State: "assigned"}})
	if !errors.Is(err, ErrAddressOutOfCIDR) {
		t.Fatalf("stale import: %v", err)
	}
	assertCount(t, ctx, pool, `SELECT count(*) FROM ip_addresses WHERE address='10.0.0.20'`, 0)
	mustExec(t, ctx, pool, `INSERT INTO scan_discoveries (id, ip, status, reconcile_status) VALUES ('discovery', '10.0.0.200', 'pending', 'new')`)
	if _, err := st.ImportDiscovery(ctx, "discovery", ""); !errors.Is(err, ErrNoContainingSubnet) {
		t.Fatalf("discovery: %v", err)
	}
	assertCount(t, ctx, pool, `SELECT count(*) FROM scan_discoveries WHERE id='discovery' AND status='pending'`, 1)
	assertCount(t, ctx, pool, `SELECT count(*) FROM ip_addresses WHERE address='10.0.0.200'`, 0)
	assertCount(t, ctx, pool, `SELECT count(*) FROM devices`, 0)
}

func TestContainmentLockOrdering(t *testing.T) {
	for _, kind := range []string{"create", "update", "csv", "discovery", "editor"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			st, pool := newIntegrationStore(t, ctx)
			mustExec(t, ctx, pool, `INSERT INTO subnets (id, site_id, cidr, name) VALUES ('subnet', 'default', '10.0.0.0/24', 'LAN')`)
			mustExec(t, ctx, pool, `INSERT INTO ip_addresses (id, subnet_id, address, state) VALUES ('ip', 'subnet', '10.0.0.10', 'assigned')`)
			mustExec(t, ctx, pool, `INSERT INTO scan_discoveries (id, ip, status, reconcile_status) VALUES ('discovery', '10.0.0.200', 'pending', 'new')`)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if err := lockSubnetContainment(ctx, tx, kind != "editor"); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() {
				subnet := Subnet{ID: "subnet", CIDR: "10.0.0.0/24"}
				var err error
				switch kind {
				case "create":
					_, err = st.CreateAddress(ctx, subnet, AddressInput{Address: "10.0.0.200", State: "assigned"})
				case "update":
					_, err = st.UpdateAddress(ctx, "ip", subnet, AddressInput{Address: "10.0.0.200", State: "assigned"})
				case "csv":
					err = st.ImportAddresses(ctx, []AddressImport{{Address: "10.0.0.200", SubnetID: "subnet", State: "assigned"}})
				case "discovery":
					_, err = st.ImportDiscovery(ctx, "discovery", "")
				case "editor":
					_, err = st.UpdateSubnet(ctx, "subnet", SubnetInput{SiteID: "default", CIDR: "10.0.0.0/25", Name: "LAN"})
				}
				result <- err
			}()
			for {
				var waiting bool
				if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_locks WHERE locktype='advisory' AND classid=1279872333 AND objid=1 AND NOT granted)`).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case err := <-result:
					t.Fatalf("operation did not wait: %v", err)
				case <-ctx.Done():
					t.Fatal("timed out waiting for advisory lock")
				default:
					runtime.Gosched()
				}
			}
			if kind == "editor" {
				if _, err := tx.Exec(ctx, `INSERT INTO ip_addresses (id, subnet_id, address, state) VALUES ('racing', 'subnet', '10.0.0.200', 'assigned')`); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := tx.Exec(ctx, `UPDATE subnets SET cidr='10.0.0.0/25' WHERE id='subnet'`); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			err = <-result
			want := ErrAddressOutOfCIDR
			if kind == "editor" {
				want = ErrSubnetContainsAddresses
			}
			if kind == "discovery" {
				want = ErrNoContainingSubnet
			}
			if !errors.Is(err, want) {
				t.Fatalf("operation returned %v, want %v", err, want)
			}
			assertCount(t, ctx, pool, `SELECT count(*) FROM ip_addresses ip JOIN subnets s ON s.id=ip.subnet_id WHERE NOT (ip.address <<= s.cidr)`, 0)
			if kind == "discovery" {
				assertCount(t, ctx, pool, `SELECT count(*) FROM devices`, 0)
				assertCount(t, ctx, pool, `SELECT count(*) FROM scan_discoveries WHERE id='discovery' AND status='pending'`, 1)
			}
		})
	}
}

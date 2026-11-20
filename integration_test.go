//go:build integration

package fastnear_test

// Integration smoke tests against the live FastNear API. They run in
// CI (workflow integration.yml) or locally with:
//
//	FASTNEAR_API_KEY=... FASTNEAR_INTEGRATION=1 go test -tags=integration -v
//
// They use only stable public data (root.near, wrap.near) and never
// hardcode rotating access keys or growing block heights.

import (
	"context"
	"os"
	"testing"
	"time"

	fastnear "github.com/tigusigalpa/fastnear-go"
	"github.com/tigusigalpa/fastnear-go/tx"
)

func liveClient(t *testing.T) *fastnear.Client {
	t.Helper()
	if os.Getenv("FASTNEAR_INTEGRATION") == "" {
		t.Skip("set FASTNEAR_INTEGRATION=1 to run live tests")
	}
	c, err := fastnear.NewClient(
		fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")),
		fastnear.WithTimeout(20*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestLiveHealth(t *testing.T) {
	c := liveClient(t)
	h, err := c.Accounts().Health(context.Background())
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if h == nil || h.Status == "" {
		t.Fatalf("health = %+v", h)
	}
}

func TestLiveAccountFull(t *testing.T) {
	c := liveClient(t)
	full, err := c.Accounts().AccountFull(context.Background(), "root.near")
	if err != nil {
		t.Fatalf("account_full: %v", err)
	}
	if full == nil {
		t.Fatal("nil account")
	}
}

func TestLiveViewAccount(t *testing.T) {
	c := liveClient(t)
	acct, err := c.RPC().ViewAccount(context.Background(), "root.near", nil)
	if err != nil {
		t.Fatalf("view_account: %v", err)
	}
	if acct == nil || acct.Amount == "" {
		t.Fatalf("acct = %+v", acct)
	}
	if _, err := fastnear.FromYocto(acct.Amount); err != nil {
		t.Fatalf("amount not yocto: %v", err)
	}
}

func TestLiveLastFinalHeight(t *testing.T) {
	c := liveClient(t)
	h, err := c.Data().LastFinalHeight(context.Background())
	if err != nil {
		t.Fatalf("last_final: %v", err)
	}
	if h == 0 {
		t.Fatal("height = 0")
	}
}

func TestLiveAccountHistoryPage(t *testing.T) {
	c := liveClient(t)
	desc, limit := true, uint32(2)
	q := tx.NewAccountHistoryQuery("intents.near")
	q.Desc, q.Limit = &desc, &limit
	page, err := c.Tx().AccountHistory(context.Background(), q)
	if err != nil {
		t.Fatalf("account history: %v", err)
	}
	if page == nil {
		t.Fatal("nil page")
	}
}

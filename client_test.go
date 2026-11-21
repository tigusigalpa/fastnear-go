package fastnear

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tigusigalpa/fastnear-go/transfers"
)

func TestSharedPoolAndTestnetConfigurationError(t *testing.T) {
	c, err := NewClient(WithConnectTimeout(123 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Accounts()
	pool := c.httpClient
	_ = c.RPC()
	if pool != c.httpClient {
		t.Fatal("families created separate root pools")
	}
	if pool.Transport.(*http.Transport).TLSHandshakeTimeout != 123*time.Millisecond {
		t.Fatal("root ignored connection timeout")
	}
	testnet, _ := NewClient(WithTestnet())
	_, err = testnet.Transfers().Query(context.Background(), transfers.Query{})
	if err == nil || !strings.Contains(err.Error(), "mainnet-only") {
		t.Fatalf("configuration cause lost: %v", err)
	}
}

func TestRejectUnsafeOptionsAndMaskConfig(t *testing.T) {
	for _, option := range []Option{
		WithBaseURL(FamilyRPC, "ftp://example.test"),
		WithBaseURL(FamilyRPC, "https://user:password@example.test"),
		WithBaseURL(FamilyRPC, "https://example.test?apiKey=secret"),
		WithBaseURL(FamilyRPC, "https://example.test#part"),
		WithBaseURL("unknown", "https://example.test"),
		WithAPIKey("key\x00bad"), WithUserAgent("agent\r\nInjected: x"),
	} {
		if _, err := NewClient(option); err == nil {
			t.Fatal("unsafe option accepted")
		}
	}
	c, _ := NewClient(WithAPIKey("private-secret-1234"))
	for _, value := range []string{fmt.Sprintf("%v", c.Config()), fmt.Sprintf("%+v", c.Config()), fmt.Sprintf("%#v", c.Config())} {
		if strings.Contains(value, "private-secret") {
			t.Fatalf("config leaked key: %s", value)
		}
	}
}

func TestNewClientDefaults(t *testing.T) {
	c, err := NewClient(WithAPIKey("key-1234"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := c.Config()
	if cfg.Network() != Mainnet || cfg.AuthMode() != AuthBearer || cfg.Archival() {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.Timeout() != 30*time.Second || cfg.MaxRetries() != 2 {
		t.Fatalf("unexpected defaults")
	}
	if cfg.MaskedAPIKey() != "***1234" {
		t.Fatalf("masked = %q", cfg.MaskedAPIKey())
	}
}

func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient(WithAPIKey("bad key")); err == nil {
		t.Fatal("key with space must be rejected")
	}
	if _, err := NewClient(WithTimeout(0)); err == nil {
		t.Fatal("zero timeout must be rejected")
	}
	if _, err := NewClient(WithNetwork("betanet")); err == nil {
		t.Fatal("unknown network must be rejected")
	}
	if _, err := NewClient(WithBaseURL(FamilyRPC, "not-a-url")); err == nil {
		t.Fatal("invalid base URL must be rejected")
	}
}

func TestFamilyBaseURL(t *testing.T) {
	u, err := FamilyRPC.BaseURL(Mainnet, false)
	if err != nil || u != "https://rpc.mainnet.fastnear.com" {
		t.Fatalf("rpc mainnet = %q %v", u, err)
	}
	u, err = FamilyRPC.BaseURL(Testnet, true)
	if err != nil || u != "https://archival-rpc.testnet.fastnear.com" {
		t.Fatalf("rpc archival testnet = %q %v", u, err)
	}
	if FamilyTransfers.SupportsTestnet() {
		t.Fatal("transfers must not support testnet")
	}
	if _, err := FamilyTransfers.BaseURL(Testnet, false); err == nil {
		t.Fatal("transfers testnet must error")
	}
	u, err = FamilyData.BaseURL(Testnet, false)
	if err != nil || u != "https://testnet.neardata.xyz" {
		t.Fatalf("data testnet = %q %v", u, err)
	}
}

func TestNilClientSafe(t *testing.T) {
	var c *Client
	if err := c.Close(); err != nil {
		t.Fatal("Close on nil must be nil")
	}
	if c.Accounts() == nil {
		t.Fatal("Accounts on nil must return a client")
	}
}

func TestCloseIdempotent(t *testing.T) {
	c, _ := NewClient()
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal("second Close must be safe")
	}
}

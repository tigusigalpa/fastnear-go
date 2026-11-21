package fastnear

import (
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/tigusigalpa/fastnear-go/accounts"
	"github.com/tigusigalpa/fastnear-go/data"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
	"github.com/tigusigalpa/fastnear-go/kv"
	"github.com/tigusigalpa/fastnear-go/rpc"
	"github.com/tigusigalpa/fastnear-go/transfers"
	"github.com/tigusigalpa/fastnear-go/tx"
)

// Version is the SDK version sent in User-Agent.
const Version = "1.0.0"

const userAgent = "fastnear-go/" + Version + " (+https://github.com/tigusigalpa/fastnear-go)"

// Client is the entry point to all six FastNear API surfaces. Create it
// with NewClient; family clients are constructed lazily and cached.
// Client is safe for concurrent use.
type Client struct {
	cfg Config

	mu         sync.Mutex
	httpClient *http.Client
	transports map[Family]*transport.Client

	accountsC  *accounts.Client
	rpcC       *rpc.Client
	txC        *tx.Client
	dataC      *data.Client
	transfersC *transfers.Client
	kvC        *kv.Client
}

var errUninitialized = errors.New("fastnear: uninitialized client; use fastnear.NewClient")

// NewClient validates the options and builds a Client.
func NewClient(opts ...Option) (*Client, error) {
	cfg := Config{
		network:          Mainnet,
		authMode:         AuthBearer,
		timeout:          30 * time.Second,
		connectTimeout:   5 * time.Second,
		maxRetries:       2,
		retryBaseDelay:   200 * time.Millisecond,
		maxResponseBytes: 32 << 20,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.userAgent == "" {
		cfg.userAgent = userAgent
	}
	return &Client{cfg: cfg}, nil
}

// Config returns a copy of the client configuration.
func (c *Client) Config() Config {
	if c == nil {
		return Config{}
	}
	return c.cfg
}

// Accounts returns the FastNear API client (indexed account views).
// See https://docs.fastnear.com/api
func (c *Client) Accounts() *accounts.Client {
	if c == nil {
		return accounts.NewClient(nil, errUninitialized)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accountsC == nil {
		t, err := c.transportFor(FamilyAccounts)
		c.accountsC = accounts.NewClient(t, err)
	}
	return c.accountsC
}

// RPC returns the JSON-RPC client. With WithArchival it targets the
// archival node. See https://docs.fastnear.com/rpc
func (c *Client) RPC() *rpc.Client {
	if c == nil {
		return rpc.NewClient(nil, errUninitialized)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rpcC == nil {
		t, err := c.transportFor(FamilyRPC)
		c.rpcC = rpc.NewClient(t, err)
	}
	return c.rpcC
}

// Tx returns the Transactions API client (indexed history).
// See https://docs.fastnear.com/tx
func (c *Client) Tx() *tx.Client {
	if c == nil {
		return tx.NewClient(nil, errUninitialized)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.txC == nil {
		t, err := c.transportFor(FamilyTx)
		c.txC = tx.NewClient(t, err)
	}
	return c.txC
}

// Data returns the NEAR Data API client (raw block documents).
// See https://docs.fastnear.com/neardata
func (c *Client) Data() *data.Client {
	if c == nil {
		return data.NewClient(nil, errUninitialized)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dataC == nil {
		t, err := c.transportFor(FamilyData)
		c.dataC = data.NewClient(t, err)
	}
	return c.dataC
}

// Transfers returns the Transfers API client (mainnet only).
// See https://docs.fastnear.com/transfers
func (c *Client) Transfers() *transfers.Client {
	if c == nil {
		return transfers.NewClient(nil, errUninitialized)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.transfersC == nil {
		t, err := c.transportFor(FamilyTransfers)
		c.transfersC = transfers.NewClient(t, err)
	}
	return c.transfersC
}

// KV returns the KV FastData client (contract storage index).
// See https://docs.fastnear.com/fastdata/kv
func (c *Client) KV() *kv.Client {
	if c == nil {
		return kv.NewClient(nil, errUninitialized)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.kvC == nil {
		t, err := c.transportFor(FamilyKV)
		c.kvC = kv.NewClient(t, err)
	}
	return c.kvC
}

// Close releases idle HTTP connections. Repeated calls are safe.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}

// transportFor builds (or reuses) the per-family transport. A shared
// *http.Client is created lazily unless WithHTTPClient provided one.
func (c *Client) transportFor(f Family) (*transport.Client, error) {
	if c == nil {
		return nil, errUninitialized
	}
	if c.transports == nil {
		c.transports = make(map[Family]*transport.Client)
	}
	if t, ok := c.transports[f]; ok {
		return t, nil
	}
	baseURL, err := c.cfg.BaseURL(f)
	if err != nil {
		return nil, err
	}
	if c.httpClient == nil {
		c.httpClient = c.cfg.httpClient
		if c.httpClient == nil {
			c.httpClient = transport.NewHTTPClient(c.cfg.connectTimeout)
		}
	}
	// NEAR Data redirects to sibling mirror hosts (a0.<host>, …);
	// allow cross-host redirects within the configured host's domain.
	var redirHosts []string
	if f == FamilyData {
		if u, perr := url.Parse(baseURL); perr == nil && u.Hostname() != "" {
			redirHosts = []string{"." + u.Hostname()}
		}
	}
	t := transport.New(transport.Params{
		HTTPClient:     c.httpClient,
		BaseURL:        baseURL,
		APIKey:         c.cfg.apiKey,
		AuthMode:       transport.AuthMode(c.cfg.authMode),
		Family:         string(f),
		Timeout:        c.cfg.timeout,
		ConnectTimeout: c.cfg.connectTimeout,
		Retry: transport.RetryPolicy{
			MaxRetries:         c.cfg.maxRetries,
			BaseDelay:          c.cfg.retryBaseDelay,
			MaxDelay:           5 * time.Second,
			Budget:             15 * time.Second,
			RetryNonIdempotent: c.cfg.retryNonIdem,
		},
		Logger:               c.cfg.logger,
		UserAgent:            c.cfg.userAgent,
		MaxBytes:             c.cfg.maxResponseBytes,
		Middleware:           c.cfg.middleware,
		RedirectHostSuffixes: redirHosts,
	})
	c.transports[f] = t
	return t, nil
}

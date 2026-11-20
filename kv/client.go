package kv

import "github.com/tigusigalpa/fastnear-go/internal/transport"

// Client is the kv API client. Obtain it from the root fastnear.Client;
// it is safe for concurrent use.
type Client struct {
	t   *transport.Client
	err error
}

// NewClient is used internally by fastnear.Client. The err parameter
// carries a configuration error that every method returns.
func NewClient(t *transport.Client, err error) *Client {
	return &Client{t: t, err: err}
}

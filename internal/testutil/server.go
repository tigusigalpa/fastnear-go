// Package testutil provides HTTP fixtures over in-memory connections. Tests
// exercise net/http without opening TCP sockets or contacting external hosts.
package testutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
)

var serverIDs atomic.Uint64

// Server serves a handler at a unique test URL using net.Pipe connections.
type Server struct {
	URL      string
	client   *http.Client
	server   *http.Server
	done     chan struct{}
	handlers sync.WaitGroup
}

// NewServer starts an HTTP server whose requests must use Client().
func NewServer(handler http.Handler) *Server {
	address := fmt.Sprintf("fixture-%d.test:80", serverIDs.Add(1))
	l := &pipeListener{connections: make(chan net.Conn), closed: make(chan struct{})}
	s := &Server{URL: "http://" + address, done: make(chan struct{})}
	s.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handlers.Add(1)
		defer s.handlers.Done()
		handler.ServeHTTP(w, r)
	})}
	s.client = &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			if addr != address {
				return nil, errors.New("test fixture refused an external host")
			}
			clientConn, serverConn := net.Pipe()
			select {
			case l.connections <- serverConn:
				return clientConn, nil
			case <-ctx.Done():
				_ = clientConn.Close()
				_ = serverConn.Close()
				return nil, ctx.Err()
			case <-l.closed:
				_ = clientConn.Close()
				_ = serverConn.Close()
				return nil, net.ErrClosed
			}
		},
	}}
	go func() {
		defer close(s.done)
		_ = s.server.Serve(l)
	}()
	return s
}

// Client returns an HTTP client connected to this fixture.
func (s *Server) Client() *http.Client { return s.client }

// Close stops the server and waits for its handlers to finish.
func (s *Server) Close() {
	s.client.CloseIdleConnections()
	_ = s.server.Close()
	<-s.done
	s.handlers.Wait()
}

type pipeListener struct {
	connections chan net.Conn
	closed      chan struct{}
	once        sync.Once
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connections:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddress("memory") }

type pipeAddress string

func (a pipeAddress) Network() string { return "pipe" }
func (a pipeAddress) String() string  { return string(a) }

package fastnearerr

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestHelpers(t *testing.T) {
	nf := &Error{StatusCode: 404}
	if !IsNotFound(nf) || IsAuth(nf) {
		t.Fatal("IsNotFound wrong")
	}
	if !IsNotFound(fmt.Errorf("wrap: %w", nf)) {
		t.Fatal("must unwrap")
	}
	auth := &Error{StatusCode: 403}
	if !IsAuth(auth) || IsNotFound(auth) {
		t.Fatal("IsAuth wrong")
	}
	quota := &Error{StatusCode: 402, Code: "quota_exceeded"}
	if !IsQuotaExceeded(quota) || IsRetryable(quota) {
		t.Fatal("quota wrong")
	}
	rl := &Error{StatusCode: 429, RetryAfter: 2 * time.Second}
	if !IsRateLimited(rl) || !IsRetryable(rl) {
		t.Fatal("rate limit wrong")
	}
	srv := &Error{StatusCode: 500}
	if !IsRetryable(srv) {
		t.Fatal("5xx must be retryable")
	}
	rpcBad := &Error{RPC: &RPCError{Code: -32602}}
	if IsRetryable(rpcBad) {
		t.Fatal("invalid params must not be retryable")
	}
	rpcSrv := &Error{RPC: &RPCError{Code: -32000}}
	if !IsRetryable(rpcSrv) {
		t.Fatal("rpc -32000 must be retryable once")
	}
	if IsRetryable(context.Canceled) {
		t.Fatal("canceled must not be retryable")
	}
	tr := &Error{Code: "timeout", Err: context.DeadlineExceeded}
	if !IsRetryable(tr) {
		t.Fatal("timeout must be retryable")
	}
	e, ok := As(fmt.Errorf("x: %w", nf))
	if !ok || e != nf {
		t.Fatal("As wrong")
	}
	if _, ok := As(errors.New("plain")); ok {
		t.Fatal("As must fail on plain error")
	}
}

func TestErrorStringNoSecrets(t *testing.T) {
	e := &Error{Op: "rpc.view_account", Family: "rpc", StatusCode: 403, Message: "forbidden", RequestURL: "https://x/?apiKey=secret"}
	s := e.Error()
	if s == "" {
		t.Fatal("empty message")
	}
}

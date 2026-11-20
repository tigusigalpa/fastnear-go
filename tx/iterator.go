package tx

import (
	"context"
	"iter"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
)

// AccountHistoryIterator walks account-history pages using the
// resume_token cursor. It performs no request until the first Next.
type AccountHistoryIterator struct {
	client   *Client
	query    AccountHistoryQuery
	maxPages int

	pages     int
	lastToken *string // nil = not started
	done      bool
}

// Token returns the current cursor position.
func (it *AccountHistoryIterator) Token() string {
	if it.lastToken == nil {
		return ""
	}
	return *it.lastToken
}

// Next fetches the next page. It returns (rows, ok, err): ok=false at
// the end of the history; err is non-nil on failure.
func (it *AccountHistoryIterator) Next(ctx context.Context) ([]AccountTxRow, bool, error) {
	if it.done {
		return nil, false, nil
	}
	if it.pages >= it.maxPages {
		it.done = true
		return nil, false, &fastnearerr.Error{Op: family + ".account", Family: family, Code: "pagination_limit", Message: "max pages limit reached"}
	}
	if it.lastToken != nil && *it.lastToken == "" {
		it.done = true
		return nil, false, nil
	}
	page, err := it.client.AccountHistory(ctx, it.query)
	if err != nil {
		it.done = true
		return nil, false, err
	}
	if page == nil {
		it.done = true
		return nil, false, nil
	}
	it.pages++
	token := page.ResumeToken
	if it.lastToken != nil && token == *it.lastToken {
		it.done = true
		return nil, false, &fastnearerr.Error{Op: family + ".account", Family: family, Code: "pagination_loop", Message: "server returned a repeated resume_token"}
	}
	it.lastToken = &token
	it.query.ResumeToken = &token
	if token == "" {
		it.done = true
	}
	return page.AccountTxs, true, nil
}

// AccountHistorySeq streams account-history rows as an iter.Seq2,
// stopping on error, context cancellation or exhaustion.
func (c *Client) AccountHistorySeq(ctx context.Context, q AccountHistoryQuery) iter.Seq2[AccountTxRow, error] {
	return func(yield func(AccountTxRow, error) bool) {
		it := c.AccountHistoryIterator(q, 0)
		for {
			rows, ok, err := it.Next(ctx)
			if err != nil {
				var zero AccountTxRow
				yield(zero, err)
				return
			}
			if !ok {
				return
			}
			for _, row := range rows {
				if !yield(row, nil) {
					return
				}
			}
		}
	}
}

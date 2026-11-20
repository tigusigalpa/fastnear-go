package transfers

import (
	"context"
	"iter"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
)

// Iterator walks transfer pages using the resume_token cursor.
// It performs no request until the first Next.
type Iterator struct {
	client   *Client
	query    Query
	maxPages int

	pages     int
	lastToken *string
	done      bool
}

// Token returns the current cursor position.
func (it *Iterator) Token() string {
	if it.lastToken == nil {
		return ""
	}
	return *it.lastToken
}

// Next fetches the next page: (rows, ok, err).
func (it *Iterator) Next(ctx context.Context) ([]TransferRow, bool, error) {
	if it.done {
		return nil, false, nil
	}
	if it.pages >= it.maxPages {
		it.done = true
		return nil, false, &fastnearerr.Error{Op: family + ".query", Family: family, Code: "pagination_limit", Message: "max pages limit reached"}
	}
	if it.lastToken != nil && *it.lastToken == "" {
		it.done = true
		return nil, false, nil
	}
	page, err := it.client.Query(ctx, it.query)
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
		return nil, false, &fastnearerr.Error{Op: family + ".query", Family: family, Code: "pagination_loop", Message: "server returned a repeated resume_token"}
	}
	it.lastToken = &token
	it.query.ResumeToken = &token
	if token == "" {
		it.done = true
	}
	return page.Transfers, true, nil
}

// All streams all transfers matching q as an iter.Seq2.
func (c *Client) All(ctx context.Context, q Query) iter.Seq2[TransferRow, error] {
	return func(yield func(TransferRow, error) bool) {
		it := c.Iterator(q, 0)
		for {
			rows, ok, err := it.Next(ctx)
			if err != nil {
				var zero TransferRow
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

package kv

import (
	"context"
	"iter"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
)

// PageFunc fetches one page given a continuation token (nil = first
// page).
type PageFunc func(pageToken *string) (*List, error)

// Iterator walks KV list pages using the page_token cursor. It
// performs no request until the first Next.
type Iterator struct {
	fetch    PageFunc
	maxPages int

	pages     int
	lastToken *string
	done      bool
	started   bool
	seen      map[string]bool
}

// NewIterator builds an iterator over a page-fetching function.
// maxPages<=0 uses the default cap of 100.
func NewIterator(fetch PageFunc, maxPages int) *Iterator {
	if maxPages <= 0 {
		maxPages = 100
	}
	return &Iterator{fetch: fetch, maxPages: maxPages}
}

// Next fetches the next page: (entries, ok, err).
func (it *Iterator) Next(ctx context.Context) ([]Entry, bool, error) {
	if it.done {
		return nil, false, nil
	}
	if err := ctx.Err(); err != nil {
		it.done = true
		return nil, false, &fastnearerr.Error{Op: family + ".iterate", Family: family, Code: "canceled", Err: err}
	}
	if it.fetch == nil {
		it.done = true
		return nil, false, fastnearerr.Config(family+".iterate", family, "nil page fetch function")
	}
	if it.pages >= it.maxPages {
		it.done = true
		return nil, false, &fastnearerr.Error{Op: family + ".iterate", Family: family, Code: "pagination_limit", Message: "max pages limit reached"}
	}
	var token *string
	if it.started {
		token = it.lastToken
		if token == nil {
			it.done = true
			return nil, false, nil
		}
	}
	page, err := it.fetch(token)
	if err != nil {
		it.done = true
		return nil, false, err
	}
	if page == nil {
		it.done = true
		return nil, false, nil
	}
	it.pages++
	it.started = true
	if page.PageToken != nil && it.seen[*page.PageToken] {
		it.done = true
		return nil, false, &fastnearerr.Error{Op: family + ".iterate", Family: family, Code: "pagination_loop", Message: "server returned a repeated page_token"}
	}
	if page.PageToken != nil {
		if it.seen == nil {
			it.seen = make(map[string]bool)
		}
		it.seen[*page.PageToken] = true
	}
	it.lastToken = page.PageToken
	if page.PageToken == nil {
		it.done = true
	}
	return page.Entries, true, nil
}

// Token returns the current cursor position.
func (it *Iterator) Token() string {
	if it.lastToken == nil {
		return ""
	}
	return *it.lastToken
}

// Seq streams all entries fetched by fetch as an iter.Seq2.
func Seq(ctx context.Context, fetch PageFunc) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		it := NewIterator(fetch, 0)
		for {
			rows, ok, err := it.Next(ctx)
			if err != nil {
				var zero Entry
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

// LatestByAccountSeq streams all latest entries of a contract.
func (c *Client) LatestByAccountSeq(ctx context.Context, contractID string, q LatestQuery) iter.Seq2[Entry, error] {
	return Seq(ctx, func(token *string) (*List, error) {
		pageQuery := q
		if token != nil {
			pageQuery.PageToken = token
		}
		return c.LatestByAccount(ctx, contractID, pageQuery)
	})
}

// LatestByPredecessorSeq streams all latest entries written by a
// predecessor.
func (c *Client) LatestByPredecessorSeq(ctx context.Context, contractID, predecessorID string, q LatestQuery) iter.Seq2[Entry, error] {
	return Seq(ctx, func(token *string) (*List, error) {
		pageQuery := q
		if token != nil {
			pageQuery.PageToken = token
		}
		return c.LatestByPredecessor(ctx, contractID, predecessorID, pageQuery)
	})
}

// HistoryByAccountSeq streams the write history of a contract.
func (c *Client) HistoryByAccountSeq(ctx context.Context, contractID string, q HistoryQuery) iter.Seq2[Entry, error] {
	return Seq(ctx, func(token *string) (*List, error) {
		pageQuery := q
		if token != nil {
			pageQuery.PageToken = token
		}
		return c.HistoryByAccount(ctx, contractID, pageQuery)
	})
}

// HistoryByPredecessorSeq streams the write history of a contract by
// one predecessor.
func (c *Client) HistoryByPredecessorSeq(ctx context.Context, contractID, predecessorID string, q HistoryQuery) iter.Seq2[Entry, error] {
	return Seq(ctx, func(token *string) (*List, error) {
		pageQuery := q
		if token != nil {
			pageQuery.PageToken = token
		}
		return c.HistoryByPredecessor(ctx, contractID, predecessorID, pageQuery)
	})
}

// HistoryByKeySeq streams the write history of a key.
func (c *Client) HistoryByKeySeq(ctx context.Context, key string, q HistoryQuery) iter.Seq2[Entry, error] {
	return Seq(ctx, func(token *string) (*List, error) {
		pageQuery := q
		if token != nil {
			pageQuery.PageToken = token
		}
		return c.HistoryByKey(ctx, key, pageQuery)
	})
}

// AllByPredecessorSeq streams all entries written by a predecessor.
func (c *Client) AllByPredecessorSeq(ctx context.Context, predecessorID string, limit *uint32, includeMetadata bool) iter.Seq2[Entry, error] {
	return Seq(ctx, func(token *string) (*List, error) {
		return c.AllByPredecessor(ctx, predecessorID, limit, includeMetadata, token)
	})
}

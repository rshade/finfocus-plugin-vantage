package client

import (
	"context"
	"errors"
	"fmt"
)

// Pager provides cursor-based pagination for cost queries.
type Pager struct {
	client     Client
	query      Query
	logger     Logger
	hasStarted bool
}

// NewPager creates a new pager for the given query.
func NewPager(client Client, query Query, logger Logger) *Pager {
	return &Pager{
		client: client,
		query:  query,
		logger: logger,
	}
}

// NextPage fetches the next page of cost data.
func (p *Pager) NextPage(ctx context.Context) (Page, error) {
	// If we've already started and there's no cursor, we've exhausted all pages.
	if p.hasStarted && p.query.Cursor == "" {
		return Page{}, errors.New("no more pages available")
	}

	currentQuery := p.query
	if p.query.Cursor != "" {
		currentQuery.Cursor = p.query.Cursor
	}

	page, err := p.client.Costs(ctx, currentQuery)
	if err != nil {
		p.logger.Error(ctx, "Failed to fetch costs page", map[string]interface{}{
			"error":  err,
			"cursor": currentQuery.Cursor,
		})
		return Page{}, fmt.Errorf("fetching costs page: %w", err)
	}

	// Mark that we've started paging and update cursor for next page.
	p.hasStarted = true
	p.query.Cursor = page.NextCursor

	p.logger.Debug(ctx, "Fetched costs page", map[string]interface{}{
		logFieldRows:  len(page.Data),
		"next_cursor": page.NextCursor,
		"has_more":    page.HasMore,
	})

	return page, nil
}

// HasMore returns true if there are more pages to fetch.
func (p *Pager) HasMore() bool {
	return p.query.Cursor != ""
}

// AllPages fetches all pages and returns them as a single slice.
// Note: This can be memory-intensive for large datasets.
func (p *Pager) AllPages(ctx context.Context) ([]CostRow, error) {
	var allRows []CostRow

	for p.HasMore() || p.query.Cursor == "" {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		allRows = append(allRows, page.Data...)

		if !page.HasMore {
			break
		}
	}

	p.logger.Info(ctx, "Fetched all cost pages", map[string]interface{}{
		"total_rows": len(allRows),
	})
	return allRows, nil
}

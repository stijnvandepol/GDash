package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stijnvandepol/gdash/internal/webpage"
)

// maxWebPages bounds the list, so every read is bounded. internal/webpage
// enforces its own, lower limit.
const maxWebPages = 50

const webPageColumns = "id, name, url, refresh_minutes, zoom, allow_scripts, created_at, updated_at"

// WebPages returns every web page in the order they were added.
func (s *Store) WebPages(ctx context.Context) ([]webpage.Page, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+webPageColumns+" FROM web_pages ORDER BY id LIMIT ?", maxWebPages)
	if err != nil {
		return nil, fmt.Errorf("list web pages: %w", err)
	}
	defer rows.Close()
	var list []webpage.Page
	for rows.Next() {
		page, err := scanWebPage(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, page)
	}
	return list, rows.Err()
}

// WebPage returns one web page, or ErrNotFound.
func (s *Store) WebPage(ctx context.Context, id int64) (webpage.Page, error) {
	page, err := scanWebPage(s.db.QueryRowContext(ctx, "SELECT "+webPageColumns+" FROM web_pages WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return webpage.Page{}, ErrNotFound
	}
	return page, err
}

// AddWebPage stores a page and returns its ID. It returns ErrConflict when
// limit pages exist already.
func (s *Store) AddWebPage(ctx context.Context, page webpage.Page, limit int) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM web_pages").Scan(&count); err != nil {
		return 0, fmt.Errorf("count web pages: %w", err)
	}
	if count >= min(limit, maxWebPages) {
		return 0, fmt.Errorf("%w: limit of %d web pages reached", ErrConflict, limit)
	}
	res, err := tx.ExecContext(ctx,
		"INSERT INTO web_pages (name, url, refresh_minutes, zoom, allow_scripts, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		page.Name, page.URL, page.RefreshMinutes, page.Zoom, page.AllowScripts, formatTime(page.CreatedAt), formatTime(page.UpdatedAt))
	if err != nil {
		return 0, fmt.Errorf("add web page: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// UpdateWebPage replaces everything of a page except its ID and creation
// time, or returns ErrNotFound.
func (s *Store) UpdateWebPage(ctx context.Context, page webpage.Page) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE web_pages SET name = ?, url = ?, refresh_minutes = ?, zoom = ?, allow_scripts = ?, updated_at = ? WHERE id = ?",
		page.Name, page.URL, page.RefreshMinutes, page.Zoom, page.AllowScripts, formatTime(page.UpdatedAt), page.ID)
	if err != nil {
		return fmt.Errorf("update web page: %w", err)
	}
	return requireOneRow(res)
}

// DeleteWebPage removes a page, or returns ErrNotFound.
func (s *Store) DeleteWebPage(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM web_pages WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete web page: %w", err)
	}
	return requireOneRow(res)
}

func scanWebPage(row interface{ Scan(...any) error }) (webpage.Page, error) {
	var page webpage.Page
	var created, updated string
	if err := row.Scan(&page.ID, &page.Name, &page.URL, &page.RefreshMinutes, &page.Zoom, &page.AllowScripts, &created, &updated); err != nil {
		return webpage.Page{}, err
	}
	var err error
	if page.CreatedAt, err = parseTime(created); err != nil {
		return webpage.Page{}, fmt.Errorf("web page %d has invalid created_at: %w", page.ID, err)
	}
	if page.UpdatedAt, err = parseTime(updated); err != nil {
		return webpage.Page{}, fmt.Errorf("web page %d has invalid updated_at: %w", page.ID, err)
	}
	return page, nil
}

package settings

import (
	"context"

	"github.com/stijnvandepol/gdash/internal/webpage"
)

// WebPages returns every web page for the Web page widget, never nil.
func (s *Service) WebPages(ctx context.Context) ([]webpage.Page, error) {
	pages, err := s.Store.WebPages(ctx)
	if pages == nil {
		pages = []webpage.Page{}
	}
	return pages, err
}

// AddWebPage stores a new page. ownOrigin is the origin the admin reaches
// GDash on; a page there is refused. It returns a *validation.Error for
// invalid input and store.ErrConflict when webpage.MaxPages exist.
func (s *Service) AddWebPage(ctx context.Context, in webpage.Input, ownOrigin string) (webpage.Page, error) {
	page, err := webpage.Check(in, ownOrigin)
	if err != nil {
		return webpage.Page{}, err
	}
	page.CreatedAt = s.Now().UTC()
	page.UpdatedAt = page.CreatedAt
	if page.ID, err = s.Store.AddWebPage(ctx, page, webpage.MaxPages); err != nil {
		return webpage.Page{}, err
	}
	return page, nil
}

// UpdateWebPage changes a page, or returns store.ErrNotFound.
func (s *Service) UpdateWebPage(ctx context.Context, id int64, in webpage.Input, ownOrigin string) (webpage.Page, error) {
	stored, err := s.Store.WebPage(ctx, id)
	if err != nil {
		return webpage.Page{}, err
	}
	page, err := webpage.Check(in, ownOrigin)
	if err != nil {
		return webpage.Page{}, err
	}
	page.ID, page.CreatedAt, page.UpdatedAt = id, stored.CreatedAt, s.Now().UTC()
	if err := s.Store.UpdateWebPage(ctx, page); err != nil {
		return webpage.Page{}, err
	}
	return page, nil
}

// DeleteWebPage removes a page, or returns store.ErrNotFound. Widgets that
// show it then show that it is not available.
func (s *Service) DeleteWebPage(ctx context.Context, id int64) error {
	return s.Store.DeleteWebPage(ctx, id)
}

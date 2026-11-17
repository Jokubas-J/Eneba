package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/home"
)

// fakeHome: deals fail, everything else returns nothing.
type fakeHome struct{}

func (fakeHome) Blocks(context.Context) ([]home.Block, error)   { return nil, nil }
func (fakeHome) Banners(context.Context) ([]home.Banner, error) { return nil, nil }
func (fakeHome) BestSellers(context.Context) ([]catalog.ProductCard, error) {
	return []catalog.ProductCard{{Slug: "a"}}, nil
}
func (fakeHome) Deals(context.Context) ([]catalog.ProductCard, error) {
	return nil, errors.New("database is down")
}
func (fakeHome) NewReleases(context.Context) ([]catalog.ProductCard, error) { return nil, nil }
func (fakeHome) Categories(context.Context) ([]home.Category, error)        { return nil, nil }

func TestHomeEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{Home: fakeHome{}, AllowedOrigin: testOrigin})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	t.Run("one failing block fails alone", func(t *testing.T) {
		if rec := get("/api/home/deals"); rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "database") {
			t.Errorf("deals: status %d, body %s; want 500 without internal details", rec.Code, rec.Body)
		}
		if rec := get("/api/home/best-sellers"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"slug":"a"`) {
			t.Errorf("best sellers: status %d, body %s", rec.Code, rec.Body)
		}
	})

	t.Run("empty blocks are [] not null, and cacheable for a minute", func(t *testing.T) {
		rec := get("/api/home/banners")
		if strings.TrimSpace(rec.Body.String()) != `{"items":[]}` {
			t.Errorf("body = %s", rec.Body)
		}
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
			t.Errorf("Cache-Control = %q", got)
		}
	})
}

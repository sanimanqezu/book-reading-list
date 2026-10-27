package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/sanimanqezu/book-reading-list/internal/models"
	"github.com/sanimanqezu/book-reading-list/internal/repository"
	"github.com/sanimanqezu/book-reading-list/internal/service"
)

// fakeRepo is a configurable stand-in for repository.BookRepository so handler
// tests can drive success and error paths without a real database.
type fakeRepo struct {
	createFn         func(ctx context.Context, book *models.Book) (*models.Book, error)
	getByIDFn        func(ctx context.Context, id int) (*models.Book, error)
	getAllFn         func(ctx context.Context, status string) ([]*models.Book, error)
	updateFn         func(ctx context.Context, id int, req *models.UpdateBookRequest) (*models.Book, error)
	updateProgressFn func(ctx context.Context, id int, req *models.UpdateProgressRequest) (*models.Book, error)
	deleteFn         func(ctx context.Context, id int) error
	getStatsFn       func(ctx context.Context) (*models.BookStats, error)
}

func (f *fakeRepo) Create(ctx context.Context, book *models.Book) (*models.Book, error) {
	return f.createFn(ctx, book)
}
func (f *fakeRepo) GetByID(ctx context.Context, id int) (*models.Book, error) {
	return f.getByIDFn(ctx, id)
}
func (f *fakeRepo) GetAll(ctx context.Context, status string) ([]*models.Book, error) {
	return f.getAllFn(ctx, status)
}
func (f *fakeRepo) Update(ctx context.Context, id int, req *models.UpdateBookRequest) (*models.Book, error) {
	return f.updateFn(ctx, id, req)
}
func (f *fakeRepo) UpdateProgress(ctx context.Context, id int, req *models.UpdateProgressRequest) (*models.Book, error) {
	return f.updateProgressFn(ctx, id, req)
}
func (f *fakeRepo) Delete(ctx context.Context, id int) error { return f.deleteFn(ctx, id) }
func (f *fakeRepo) GetStats(ctx context.Context) (*models.BookStats, error) {
	return f.getStatsFn(ctx)
}

var _ repository.BookRepository = (*fakeRepo)(nil)

// newTestRouter wires the given fake repo through the real service and handler
// into a chi router that mirrors the production routing in cmd/api/main.go.
func newTestRouter(repo *fakeRepo) http.Handler {
	h := NewBookHandler(service.NewBookService(repo))
	r := chi.NewRouter()
	r.Get("/health", HealthCheck)
	r.Route("/api/books", func(r chi.Router) {
		r.Get("/stats", h.GetStats)
		r.Post("/", h.AddBook)
		r.Get("/", h.GetAllBooks)
		r.Get("/{id}", h.GetBookByID)
		r.Put("/{id}", h.UpdateBook)
		r.Delete("/{id}", h.DeleteBook)
		r.Patch("/{id}/progress", h.UpdateProgress)
	})
	return r
}

func do(t *testing.T, router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHealthCheck(t *testing.T) {
	rec := do(t, newTestRouter(&fakeRepo{}), http.MethodGet, "/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want %q", body["status"], "ok")
	}
}

func TestAddBookHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		repo       *fakeRepo
		wantStatus int
	}{
		{
			name:       "invalid json",
			body:       "{not-json",
			repo:       &fakeRepo{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing title is rejected by service",
			body:       `{}`,
			repo:       &fakeRepo{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "valid book is created",
			body: `{"title":"Dune","author":"Frank Herbert"}`,
			repo: &fakeRepo{
				createFn: func(_ context.Context, b *models.Book) (*models.Book, error) {
					b.ID = 7
					return b, nil
				},
			},
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestRouter(tt.repo), http.MethodPost, "/api/books/", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusCreated {
				var b models.Book
				if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
					t.Fatalf("decoding body: %v", err)
				}
				if b.ID != 7 || b.Title != "Dune" {
					t.Errorf("book = %+v, want ID 7 / title Dune", b)
				}
				if b.Status != models.StatusWantToRead {
					t.Errorf("status = %q, want default %q", b.Status, models.StatusWantToRead)
				}
			}
		})
	}
}

func TestGetBookByIDHandler(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		repo       *fakeRepo
		wantStatus int
	}{
		{
			name:       "non-numeric id",
			target:     "/api/books/abc",
			repo:       &fakeRepo{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "not found",
			target: "/api/books/5",
			repo: &fakeRepo{getByIDFn: func(context.Context, int) (*models.Book, error) {
				return nil, repository.ErrNotFound
			}},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "internal error",
			target: "/api/books/5",
			repo: &fakeRepo{getByIDFn: func(context.Context, int) (*models.Book, error) {
				return nil, context.DeadlineExceeded
			}},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:   "found",
			target: "/api/books/5",
			repo: &fakeRepo{getByIDFn: func(_ context.Context, id int) (*models.Book, error) {
				return &models.Book{ID: id, Title: "Dune"}, nil
			}},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestRouter(tt.repo), http.MethodGet, tt.target, "")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestGetAllBooksHandler(t *testing.T) {
	t.Run("empty list serialises as array not null", func(t *testing.T) {
		repo := &fakeRepo{getAllFn: func(context.Context, string) ([]*models.Book, error) {
			return nil, nil
		}}
		rec := do(t, newTestRouter(repo), http.MethodGet, "/api/books/", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
			t.Errorf("body = %q, want %q", got, "[]")
		}
	})

	t.Run("status query is forwarded and upper-cased", func(t *testing.T) {
		var gotStatus string
		repo := &fakeRepo{getAllFn: func(_ context.Context, status string) ([]*models.Book, error) {
			gotStatus = status
			return []*models.Book{{ID: 1, Title: "Dune"}}, nil
		}}
		rec := do(t, newTestRouter(repo), http.MethodGet, "/api/books/?status=reading", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if gotStatus != "READING" {
			t.Errorf("status forwarded = %q, want %q", gotStatus, "READING")
		}
	})

	t.Run("internal error", func(t *testing.T) {
		repo := &fakeRepo{getAllFn: func(context.Context, string) ([]*models.Book, error) {
			return nil, context.DeadlineExceeded
		}}
		rec := do(t, newTestRouter(repo), http.MethodGet, "/api/books/", "")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestUpdateBookHandler(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		body       string
		repo       *fakeRepo
		wantStatus int
	}{
		{name: "invalid id", target: "/api/books/x", body: `{"title":"Dune"}`, repo: &fakeRepo{}, wantStatus: http.StatusBadRequest},
		{name: "invalid body", target: "/api/books/1", body: `{`, repo: &fakeRepo{}, wantStatus: http.StatusBadRequest},
		{name: "validation error (missing title)", target: "/api/books/1", body: `{}`, repo: &fakeRepo{}, wantStatus: http.StatusBadRequest},
		{
			name:   "not found",
			target: "/api/books/1",
			body:   `{"title":"Dune"}`,
			repo: &fakeRepo{updateFn: func(context.Context, int, *models.UpdateBookRequest) (*models.Book, error) {
				return nil, repository.ErrNotFound
			}},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "success",
			target: "/api/books/1",
			body:   `{"title":"Dune","status":"COMPLETED"}`,
			repo: &fakeRepo{updateFn: func(_ context.Context, id int, req *models.UpdateBookRequest) (*models.Book, error) {
				return &models.Book{ID: id, Title: req.Title, Status: req.Status}, nil
			}},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestRouter(tt.repo), http.MethodPut, tt.target, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestDeleteBookHandler(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		repo       *fakeRepo
		wantStatus int
	}{
		{name: "invalid id", target: "/api/books/x", repo: &fakeRepo{}, wantStatus: http.StatusBadRequest},
		{
			name:       "not found",
			target:     "/api/books/9",
			repo:       &fakeRepo{deleteFn: func(context.Context, int) error { return repository.ErrNotFound }},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "internal error",
			target:     "/api/books/9",
			repo:       &fakeRepo{deleteFn: func(context.Context, int) error { return context.DeadlineExceeded }},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "success",
			target:     "/api/books/9",
			repo:       &fakeRepo{deleteFn: func(context.Context, int) error { return nil }},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestRouter(tt.repo), http.MethodDelete, tt.target, "")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestUpdateProgressHandler(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		body       string
		repo       *fakeRepo
		wantStatus int
	}{
		{name: "invalid id", target: "/api/books/x/progress", body: `{"currentPage":10}`, repo: &fakeRepo{}, wantStatus: http.StatusBadRequest},
		{name: "invalid body", target: "/api/books/1/progress", body: `{`, repo: &fakeRepo{}, wantStatus: http.StatusBadRequest},
		{name: "validation error (negative page)", target: "/api/books/1/progress", body: `{"currentPage":-5}`, repo: &fakeRepo{}, wantStatus: http.StatusBadRequest},
		{
			name:   "not found",
			target: "/api/books/1/progress",
			body:   `{"currentPage":10}`,
			repo: &fakeRepo{updateProgressFn: func(context.Context, int, *models.UpdateProgressRequest) (*models.Book, error) {
				return nil, repository.ErrNotFound
			}},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "success",
			target: "/api/books/1/progress",
			body:   `{"currentPage":120,"status":"READING"}`,
			repo: &fakeRepo{updateProgressFn: func(_ context.Context, id int, req *models.UpdateProgressRequest) (*models.Book, error) {
				return &models.Book{ID: id, CurrentPage: req.CurrentPage, Status: req.Status}, nil
			}},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestRouter(tt.repo), http.MethodPatch, tt.target, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestGetStatsHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &fakeRepo{getStatsFn: func(context.Context) (*models.BookStats, error) {
			return &models.BookStats{WantToRead: 1, Reading: 2, Completed: 3, TotalBooks: 6}, nil
		}}
		rec := do(t, newTestRouter(repo), http.MethodGet, "/api/books/stats", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var stats models.BookStats
		if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		if stats.TotalBooks != 6 {
			t.Errorf("totalBooks = %d, want 6", stats.TotalBooks)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		repo := &fakeRepo{getStatsFn: func(context.Context) (*models.BookStats, error) {
			return nil, context.DeadlineExceeded
		}}
		rec := do(t, newTestRouter(repo), http.MethodGet, "/api/books/stats", "")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

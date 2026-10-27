package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sanimanqezu/book-reading-list/internal/models"
	"github.com/sanimanqezu/book-reading-list/internal/repository"
)

// fakeRepo is a configurable in-memory stand-in for repository.BookRepository.
// Each method delegates to a function field so individual tests can control
// behaviour; unset fields return zero values.
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
	if f.createFn != nil {
		return f.createFn(ctx, book)
	}
	return book, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, id int) (*models.Book, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *fakeRepo) GetAll(ctx context.Context, status string) ([]*models.Book, error) {
	if f.getAllFn != nil {
		return f.getAllFn(ctx, status)
	}
	return nil, nil
}

func (f *fakeRepo) Update(ctx context.Context, id int, req *models.UpdateBookRequest) (*models.Book, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, id, req)
	}
	return nil, nil
}

func (f *fakeRepo) UpdateProgress(ctx context.Context, id int, req *models.UpdateProgressRequest) (*models.Book, error) {
	if f.updateProgressFn != nil {
		return f.updateProgressFn(ctx, id, req)
	}
	return nil, nil
}

func (f *fakeRepo) Delete(ctx context.Context, id int) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}

func (f *fakeRepo) GetStats(ctx context.Context) (*models.BookStats, error) {
	if f.getStatsFn != nil {
		return f.getStatsFn(ctx)
	}
	return nil, nil
}

// compile-time assurance that fakeRepo satisfies the interface.
var _ repository.BookRepository = (*fakeRepo)(nil)

func intPtr(i int) *int { return &i }

func TestAddBook(t *testing.T) {
	tests := []struct {
		name       string
		req        *models.AddBookRequest
		wantErr    bool
		wantStatus models.BookStatus
	}{
		{
			name:    "missing title without isbn returns error",
			req:     &models.AddBookRequest{},
			wantErr: true,
		},
		{
			name:       "valid book defaults to WANT_TO_READ",
			req:        &models.AddBookRequest{Title: "Dune"},
			wantStatus: models.StatusWantToRead,
		},
		{
			name:       "explicit status is preserved",
			req:        &models.AddBookRequest{Title: "Dune", Status: models.StatusReading},
			wantStatus: models.StatusReading,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{
				createFn: func(_ context.Context, book *models.Book) (*models.Book, error) {
					return book, nil
				},
			}
			svc := NewBookService(repo)

			book, err := svc.AddBook(context.Background(), tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if book.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", book.Status, tt.wantStatus)
			}
		})
	}
}

func TestAddBook_CreateErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	svc := NewBookService(&fakeRepo{
		createFn: func(context.Context, *models.Book) (*models.Book, error) {
			return nil, wantErr
		},
	})

	_, err := svc.AddBook(context.Background(), &models.AddBookRequest{Title: "Dune"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestUpdateBook(t *testing.T) {
	tests := []struct {
		name       string
		req        *models.UpdateBookRequest
		wantErr    bool
		wantStatus models.BookStatus
	}{
		{
			name:    "missing title returns error",
			req:     &models.UpdateBookRequest{},
			wantErr: true,
		},
		{
			name:    "rating below range returns error",
			req:     &models.UpdateBookRequest{Title: "Dune", Rating: intPtr(0)},
			wantErr: true,
		},
		{
			name:    "rating above range returns error",
			req:     &models.UpdateBookRequest{Title: "Dune", Rating: intPtr(6)},
			wantErr: true,
		},
		{
			name:       "valid update defaults blank status to WANT_TO_READ",
			req:        &models.UpdateBookRequest{Title: "Dune"},
			wantStatus: models.StatusWantToRead,
		},
		{
			name:       "valid rating within range is accepted",
			req:        &models.UpdateBookRequest{Title: "Dune", Status: models.StatusCompleted, Rating: intPtr(5)},
			wantStatus: models.StatusCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotStatus models.BookStatus
			repo := &fakeRepo{
				updateFn: func(_ context.Context, id int, req *models.UpdateBookRequest) (*models.Book, error) {
					gotStatus = req.Status
					return &models.Book{ID: id, Title: req.Title, Status: req.Status, Rating: req.Rating}, nil
				},
			}
			svc := NewBookService(repo)

			_, err := svc.UpdateBook(context.Background(), 1, tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotStatus != tt.wantStatus {
				t.Errorf("status passed to repo = %q, want %q", gotStatus, tt.wantStatus)
			}
		})
	}
}

func TestUpdateProgress(t *testing.T) {
	tests := []struct {
		name       string
		req        *models.UpdateProgressRequest
		wantErr    bool
		wantStatus models.BookStatus
	}{
		{
			name:    "negative current page returns error",
			req:     &models.UpdateProgressRequest{CurrentPage: -1},
			wantErr: true,
		},
		{
			name:    "invalid status returns error",
			req:     &models.UpdateProgressRequest{CurrentPage: 10, Status: "DONE"},
			wantErr: true,
		},
		{
			name:       "blank status defaults to READING",
			req:        &models.UpdateProgressRequest{CurrentPage: 10},
			wantStatus: models.StatusReading,
		},
		{
			name:       "valid completed status is preserved",
			req:        &models.UpdateProgressRequest{CurrentPage: 300, Status: models.StatusCompleted},
			wantStatus: models.StatusCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotStatus models.BookStatus
			repo := &fakeRepo{
				updateProgressFn: func(_ context.Context, id int, req *models.UpdateProgressRequest) (*models.Book, error) {
					gotStatus = req.Status
					return &models.Book{ID: id, CurrentPage: req.CurrentPage, Status: req.Status}, nil
				},
			}
			svc := NewBookService(repo)

			_, err := svc.UpdateProgress(context.Background(), 1, tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotStatus != tt.wantStatus {
				t.Errorf("status passed to repo = %q, want %q", gotStatus, tt.wantStatus)
			}
		})
	}
}

func TestGetAllBooks_NormalisesStatusToUpper(t *testing.T) {
	var gotStatus string
	repo := &fakeRepo{
		getAllFn: func(_ context.Context, status string) ([]*models.Book, error) {
			gotStatus = status
			return []*models.Book{}, nil
		},
	}
	svc := NewBookService(repo)

	if _, err := svc.GetAllBooks(context.Background(), "reading"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotStatus != "READING" {
		t.Errorf("status passed to repo = %q, want %q", gotStatus, "READING")
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		name string
		raw  interface{}
		want string
	}{
		{name: "nil", raw: nil, want: ""},
		{name: "plain string", raw: "a great book", want: "a great book"},
		{
			name: "typed value object",
			raw:  map[string]interface{}{"type": "/type/text", "value": "nested description"},
			want: "nested description",
		},
		{
			name: "object without value key",
			raw:  map[string]interface{}{"type": "/type/text"},
			want: "",
		},
		{name: "unexpected type", raw: 42, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDescription(tt.raw); got != tt.want {
				t.Errorf("extractDescription(%v) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestGetStats_Passthrough(t *testing.T) {
	want := &models.BookStats{WantToRead: 1, Reading: 2, Completed: 3, TotalBooks: 6}
	svc := NewBookService(&fakeRepo{
		getStatsFn: func(context.Context) (*models.BookStats, error) { return want, nil },
	})

	got, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *got != *want {
		t.Errorf("stats = %+v, want %+v", got, want)
	}
}

func TestDeleteBook_PropagatesNotFound(t *testing.T) {
	svc := NewBookService(&fakeRepo{
		deleteFn: func(context.Context, int) error { return repository.ErrNotFound },
	})

	if err := svc.DeleteBook(context.Background(), 99); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

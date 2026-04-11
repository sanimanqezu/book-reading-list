package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sanimanqezu/book-reading-list/internal/models"
	"github.com/sanimanqezu/book-reading-list/internal/repository"
)

const openLibraryURL = "https://openlibrary.org/api/books"

// openLibraryBook is used to decode the Open Library API response.
type openLibraryBook struct {
	Title   string `json:"title"`
	Authors []struct {
		Name string `json:"name"`
	} `json:"authors"`
	Description interface{} `json:"description"` // can be string or {"type","value"} object
	Cover       struct {
		Large string `json:"large"`
	} `json:"cover"`
	NumberOfPages int `json:"number_of_pages"`
}

// BookService handles business logic for the reading list.
type BookService struct {
	repo       repository.BookRepository
	httpClient *http.Client
}

// NewBookService returns a new BookService.
func NewBookService(repo repository.BookRepository) *BookService {
	return &BookService{
		repo: repo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// AddBook creates a new book. If an ISBN is provided and title is empty,
// it first fetches metadata from the Open Library API.
func (s *BookService) AddBook(ctx context.Context, req *models.AddBookRequest) (*models.Book, error) {
	if req.ISBN != "" && req.Title == "" {
		olBook, err := s.fetchFromOpenLibrary(req.ISBN)
		if err == nil {
			// Populate fields only when they are not already set by the caller.
			if req.Title == "" {
				req.Title = olBook.Title
			}
			if req.Author == "" && len(olBook.Authors) > 0 {
				req.Author = olBook.Authors[0].Name
			}
			if req.Description == "" {
				req.Description = extractDescription(olBook.Description)
			}
			if req.CoverURL == "" {
				req.CoverURL = olBook.Cover.Large
			}
			if req.TotalPages == 0 {
				req.TotalPages = olBook.NumberOfPages
			}
		}
		// If Open Library is unreachable we fall through with whatever the caller supplied.
	}

	if req.Title == "" {
		return nil, fmt.Errorf("title is required when ISBN lookup yields no result")
	}

	status := req.Status
	if status == "" {
		status = models.StatusWantToRead
	}

	book := &models.Book{
		Title:       req.Title,
		Author:      req.Author,
		ISBN:        req.ISBN,
		Description: req.Description,
		CoverURL:    req.CoverURL,
		TotalPages:  req.TotalPages,
		Status:      status,
		CurrentPage: 0,
		Notes:       req.Notes,
	}

	return s.repo.Create(ctx, book)
}

// GetBookByID returns a single book by ID.
func (s *BookService) GetBookByID(ctx context.Context, id int) (*models.Book, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAllBooks returns all books, optionally filtered by status.
func (s *BookService) GetAllBooks(ctx context.Context, status string) ([]*models.Book, error) {
	// Normalise the filter to upper-case so callers can pass "reading" etc.
	return s.repo.GetAll(ctx, strings.ToUpper(status))
}

// UpdateBook updates the editable fields of a book.
func (s *BookService) UpdateBook(ctx context.Context, id int, req *models.UpdateBookRequest) (*models.Book, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if req.Status == "" {
		req.Status = models.StatusWantToRead
	}
	if req.Rating != nil && (*req.Rating < 1 || *req.Rating > 5) {
		return nil, fmt.Errorf("rating must be between 1 and 5")
	}
	return s.repo.Update(ctx, id, req)
}

// UpdateProgress updates only the reading progress (current page + status).
func (s *BookService) UpdateProgress(ctx context.Context, id int, req *models.UpdateProgressRequest) (*models.Book, error) {
	if req.CurrentPage < 0 {
		return nil, fmt.Errorf("currentPage cannot be negative")
	}
	validStatuses := map[models.BookStatus]bool{
		models.StatusWantToRead: true,
		models.StatusReading:    true,
		models.StatusCompleted:  true,
	}
	if req.Status != "" && !validStatuses[req.Status] {
		return nil, fmt.Errorf("invalid status: %s", req.Status)
	}
	if req.Status == "" {
		req.Status = models.StatusReading
	}
	return s.repo.UpdateProgress(ctx, id, req)
}

// DeleteBook removes a book from the reading list.
func (s *BookService) DeleteBook(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

// GetStats returns aggregate counts per status.
func (s *BookService) GetStats(ctx context.Context) (*models.BookStats, error) {
	return s.repo.GetStats(ctx)
}

// fetchFromOpenLibrary calls the Open Library API and returns structured book data.
func (s *BookService) fetchFromOpenLibrary(isbn string) (*openLibraryBook, error) {
	url := fmt.Sprintf("%s?bibkeys=ISBN:%s&format=json&jscmd=data", openLibraryURL, isbn)

	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("open library request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading open library response: %w", err)
	}

	// The response is a JSON object keyed by "ISBN:<isbn>".
	var raw map[string]openLibraryBook
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding open library response: %w", err)
	}

	key := "ISBN:" + isbn
	book, ok := raw[key]
	if !ok {
		return nil, fmt.Errorf("ISBN %s not found in Open Library", isbn)
	}
	return &book, nil
}

// extractDescription handles the fact that Open Library sometimes returns
// the description as a plain string and sometimes as {"type":"...","value":"..."}.
func extractDescription(raw interface{}) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case map[string]interface{}:
		if val, ok := v["value"].(string); ok {
			return val
		}
	}
	return ""
}

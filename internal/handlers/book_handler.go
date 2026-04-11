package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sanimanqezu/book-reading-list/internal/models"
	"github.com/sanimanqezu/book-reading-list/internal/repository"
	"github.com/sanimanqezu/book-reading-list/internal/service"
)

// BookHandler holds the HTTP handler methods for the books resource.
type BookHandler struct {
	svc *service.BookService
}

// NewBookHandler returns a new BookHandler.
func NewBookHandler(svc *service.BookService) *BookHandler {
	return &BookHandler{svc: svc}
}

// ------------------------------------------------------------------
// Helper utilities
// ------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseIDParam(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}

// ------------------------------------------------------------------
// Health check
// ------------------------------------------------------------------

// HealthCheck handles GET /health.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ------------------------------------------------------------------
// Book handlers
// ------------------------------------------------------------------

// AddBook handles POST /api/books.
// If an ISBN is provided without a title, book details are auto-fetched
// from the Open Library API.
func (h *BookHandler) AddBook(w http.ResponseWriter, r *http.Request) {
	var req models.AddBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	book, err := h.svc.AddBook(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, book)
}

// GetAllBooks handles GET /api/books with optional ?status= query param.
func (h *BookHandler) GetAllBooks(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	books, err := h.svc.GetAllBooks(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve books")
		return
	}

	// Return an empty array instead of null when there are no books.
	if books == nil {
		books = []*models.Book{}
	}

	writeJSON(w, http.StatusOK, books)
}

// GetBookByID handles GET /api/books/{id}.
func (h *BookHandler) GetBookByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book id")
		return
	}

	book, err := h.svc.GetBookByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "book not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve book")
		return
	}

	writeJSON(w, http.StatusOK, book)
}

// UpdateBook handles PUT /api/books/{id}.
func (h *BookHandler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book id")
		return
	}

	var req models.UpdateBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	book, err := h.svc.UpdateBook(r.Context(), id, &req)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "book not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, book)
}

// DeleteBook handles DELETE /api/books/{id}.
func (h *BookHandler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book id")
		return
	}

	if err := h.svc.DeleteBook(r.Context(), id); errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "book not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete book")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateProgress handles PATCH /api/books/{id}/progress.
func (h *BookHandler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book id")
		return
	}

	var req models.UpdateProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	book, err := h.svc.UpdateProgress(r.Context(), id, &req)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "book not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, book)
}

// GetStats handles GET /api/books/stats.
func (h *BookHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

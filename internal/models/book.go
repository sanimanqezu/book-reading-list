package models

import "time"

// BookStatus represents the reading status of a book.
type BookStatus string

const (
	StatusWantToRead BookStatus = "WANT_TO_READ"
	StatusReading    BookStatus = "READING"
	StatusCompleted  BookStatus = "COMPLETED"
)

// Book represents a book in the reading list.
type Book struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Author      string     `json:"author,omitempty"`
	ISBN        string     `json:"isbn,omitempty"`
	Description string     `json:"description,omitempty"`
	CoverURL    string     `json:"coverUrl,omitempty"`
	TotalPages  int        `json:"totalPages,omitempty"`
	Status      BookStatus `json:"status"`
	CurrentPage int        `json:"currentPage"`
	Rating      *int       `json:"rating,omitempty"`
	Notes       string     `json:"notes,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// AddBookRequest is the payload for creating a new book entry.
// If ISBN is provided, book details are auto-fetched from Open Library.
type AddBookRequest struct {
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	ISBN        string     `json:"isbn"`
	Description string     `json:"description"`
	CoverURL    string     `json:"coverUrl"`
	TotalPages  int        `json:"totalPages"`
	Status      BookStatus `json:"status"`
	Notes       string     `json:"notes"`
}

// UpdateBookRequest is the payload for updating an existing book entry.
type UpdateBookRequest struct {
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	ISBN        string     `json:"isbn"`
	Description string     `json:"description"`
	CoverURL    string     `json:"coverUrl"`
	TotalPages  int        `json:"totalPages"`
	Status      BookStatus `json:"status"`
	Rating      *int       `json:"rating"`
	Notes       string     `json:"notes"`
}

// UpdateProgressRequest is the payload for updating reading progress.
type UpdateProgressRequest struct {
	CurrentPage int        `json:"currentPage"`
	Status      BookStatus `json:"status"`
}

// BookStats holds aggregate counts of books by status.
type BookStats struct {
	WantToRead int `json:"wantToRead"`
	Reading    int `json:"reading"`
	Completed  int `json:"completed"`
	TotalBooks int `json:"totalBooks"`
}

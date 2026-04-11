package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sanimanqezu/book-reading-list/internal/models"
)

// ErrNotFound is returned when a requested book does not exist.
var ErrNotFound = errors.New("book not found")

// BookRepository defines the data access operations for books.
type BookRepository interface {
	Create(ctx context.Context, book *models.Book) (*models.Book, error)
	GetByID(ctx context.Context, id int) (*models.Book, error)
	GetAll(ctx context.Context, status string) ([]*models.Book, error)
	Update(ctx context.Context, id int, req *models.UpdateBookRequest) (*models.Book, error)
	UpdateProgress(ctx context.Context, id int, req *models.UpdateProgressRequest) (*models.Book, error)
	Delete(ctx context.Context, id int) error
	GetStats(ctx context.Context) (*models.BookStats, error)
}

// PostgresBookRepository is the PostgreSQL implementation of BookRepository.
type PostgresBookRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresBookRepository returns a new PostgresBookRepository.
func NewPostgresBookRepository(pool *pgxpool.Pool) *PostgresBookRepository {
	return &PostgresBookRepository{pool: pool}
}

// Create inserts a new book record into the database.
func (r *PostgresBookRepository) Create(ctx context.Context, book *models.Book) (*models.Book, error) {
	query := `
		INSERT INTO books (title, author, isbn, description, cover_url, total_pages, status, current_page, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, title, author, isbn, description, cover_url, total_pages, status,
		          current_page, rating, notes, started_at, finished_at, created_at`

	created := &models.Book{}
	err := r.pool.QueryRow(ctx, query,
		book.Title,
		book.Author,
		book.ISBN,
		book.Description,
		book.CoverURL,
		book.TotalPages,
		book.Status,
		book.CurrentPage,
		book.Notes,
	).Scan(
		&created.ID,
		&created.Title,
		&created.Author,
		&created.ISBN,
		&created.Description,
		&created.CoverURL,
		&created.TotalPages,
		&created.Status,
		&created.CurrentPage,
		&created.Rating,
		&created.Notes,
		&created.StartedAt,
		&created.FinishedAt,
		&created.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("repository.Create: %w", err)
	}
	return created, nil
}

// GetByID retrieves a single book by its primary key.
func (r *PostgresBookRepository) GetByID(ctx context.Context, id int) (*models.Book, error) {
	query := `
		SELECT id, title, author, isbn, description, cover_url, total_pages, status,
		       current_page, rating, notes, started_at, finished_at, created_at
		FROM books
		WHERE id = $1`

	book := &models.Book{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&book.ID,
		&book.Title,
		&book.Author,
		&book.ISBN,
		&book.Description,
		&book.CoverURL,
		&book.TotalPages,
		&book.Status,
		&book.CurrentPage,
		&book.Rating,
		&book.Notes,
		&book.StartedAt,
		&book.FinishedAt,
		&book.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.GetByID: %w", err)
	}
	return book, nil
}

// GetAll retrieves all books, optionally filtered by status.
func (r *PostgresBookRepository) GetAll(ctx context.Context, status string) ([]*models.Book, error) {
	var (
		rows pgx.Rows
		err  error
	)

	baseQuery := `
		SELECT id, title, author, isbn, description, cover_url, total_pages, status,
		       current_page, rating, notes, started_at, finished_at, created_at
		FROM books`

	if status != "" {
		rows, err = r.pool.Query(ctx, baseQuery+" WHERE status = $1 ORDER BY created_at DESC", status)
	} else {
		rows, err = r.pool.Query(ctx, baseQuery+" ORDER BY created_at DESC")
	}
	if err != nil {
		return nil, fmt.Errorf("repository.GetAll: %w", err)
	}
	defer rows.Close()

	var books []*models.Book
	for rows.Next() {
		book := &models.Book{}
		if err := rows.Scan(
			&book.ID,
			&book.Title,
			&book.Author,
			&book.ISBN,
			&book.Description,
			&book.CoverURL,
			&book.TotalPages,
			&book.Status,
			&book.CurrentPage,
			&book.Rating,
			&book.Notes,
			&book.StartedAt,
			&book.FinishedAt,
			&book.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("repository.GetAll scan: %w", err)
		}
		books = append(books, book)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository.GetAll rows: %w", err)
	}
	return books, nil
}

// Update replaces the editable fields of an existing book.
func (r *PostgresBookRepository) Update(ctx context.Context, id int, req *models.UpdateBookRequest) (*models.Book, error) {
	query := `
		UPDATE books
		SET title       = $1,
		    author      = $2,
		    isbn        = $3,
		    description = $4,
		    cover_url   = $5,
		    total_pages = $6,
		    status      = $7,
		    rating      = $8,
		    notes       = $9
		WHERE id = $10
		RETURNING id, title, author, isbn, description, cover_url, total_pages, status,
		          current_page, rating, notes, started_at, finished_at, created_at`

	book := &models.Book{}
	err := r.pool.QueryRow(ctx, query,
		req.Title,
		req.Author,
		req.ISBN,
		req.Description,
		req.CoverURL,
		req.TotalPages,
		req.Status,
		req.Rating,
		req.Notes,
		id,
	).Scan(
		&book.ID,
		&book.Title,
		&book.Author,
		&book.ISBN,
		&book.Description,
		&book.CoverURL,
		&book.TotalPages,
		&book.Status,
		&book.CurrentPage,
		&book.Rating,
		&book.Notes,
		&book.StartedAt,
		&book.FinishedAt,
		&book.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.Update: %w", err)
	}
	return book, nil
}

// UpdateProgress updates the current_page and status of a book.
// It automatically sets started_at when status becomes READING and
// finished_at when status becomes COMPLETED.
func (r *PostgresBookRepository) UpdateProgress(ctx context.Context, id int, req *models.UpdateProgressRequest) (*models.Book, error) {
	query := `
		UPDATE books
		SET current_page = $1,
		    status       = $2,
		    started_at   = CASE
		                       WHEN $2 = 'READING' AND started_at IS NULL THEN NOW()
		                       ELSE started_at
		                   END,
		    finished_at  = CASE
		                       WHEN $2 = 'COMPLETED' AND finished_at IS NULL THEN NOW()
		                       ELSE finished_at
		                   END
		WHERE id = $3
		RETURNING id, title, author, isbn, description, cover_url, total_pages, status,
		          current_page, rating, notes, started_at, finished_at, created_at`

	book := &models.Book{}
	err := r.pool.QueryRow(ctx, query, req.CurrentPage, req.Status, id).Scan(
		&book.ID,
		&book.Title,
		&book.Author,
		&book.ISBN,
		&book.Description,
		&book.CoverURL,
		&book.TotalPages,
		&book.Status,
		&book.CurrentPage,
		&book.Rating,
		&book.Notes,
		&book.StartedAt,
		&book.FinishedAt,
		&book.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.UpdateProgress: %w", err)
	}
	return book, nil
}

// Delete removes a book from the database by its ID.
func (r *PostgresBookRepository) Delete(ctx context.Context, id int) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM books WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("repository.Delete: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetStats returns aggregate counts of books grouped by status.
func (r *PostgresBookRepository) GetStats(ctx context.Context) (*models.BookStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'WANT_TO_READ') AS want_to_read,
			COUNT(*) FILTER (WHERE status = 'READING')      AS reading,
			COUNT(*) FILTER (WHERE status = 'COMPLETED')    AS completed,
			COUNT(*)                                         AS total_books
		FROM books`

	stats := &models.BookStats{}
	err := r.pool.QueryRow(ctx, query).Scan(
		&stats.WantToRead,
		&stats.Reading,
		&stats.Completed,
		&stats.TotalBooks,
	)
	if err != nil {
		return nil, fmt.Errorf("repository.GetStats: %w", err)
	}
	return stats, nil
}

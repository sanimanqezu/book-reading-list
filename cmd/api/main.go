package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"github.com/sanimanqezu/book-reading-list/internal/handlers"
	"github.com/sanimanqezu/book-reading-list/internal/repository"
	"github.com/sanimanqezu/book-reading-list/internal/service"
)

func main() {
	// Load .env file if present (ignored in production containers).
	_ = godotenv.Load()

	dsn := buildDSN()

	// ------------------------------------------------------------------
	// Database connection pool (pgx/v5)
	// ------------------------------------------------------------------
	pool, err := connectWithRetry(dsn, 10, 3*time.Second)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	log.Println("database connection established")

	// ------------------------------------------------------------------
	// Run migrations via golang-migrate using the pgx stdlib adapter
	// ------------------------------------------------------------------
	if err := runMigrations(dsn); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations applied")

	// ------------------------------------------------------------------
	// Wire up layers
	// ------------------------------------------------------------------
	bookRepo := repository.NewPostgresBookRepository(pool)
	bookSvc := service.NewBookService(bookRepo)
	bookHandler := handlers.NewBookHandler(bookSvc)

	// ------------------------------------------------------------------
	// Router
	// ------------------------------------------------------------------
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(30 * time.Second))

	// Health check
	r.Get("/health", handlers.HealthCheck)

	// Books API — note: /stats must be registered before /{id} so Chi
	// does not interpret "stats" as an integer ID.
	r.Route("/api/books", func(r chi.Router) {
		r.Get("/stats", bookHandler.GetStats)
		r.Post("/", bookHandler.AddBook)
		r.Get("/", bookHandler.GetAllBooks)
		r.Get("/{id}", bookHandler.GetBookByID)
		r.Put("/{id}", bookHandler.UpdateBook)
		r.Delete("/{id}", bookHandler.DeleteBook)
		r.Patch("/{id}/progress", bookHandler.UpdateProgress)
	})

	addr := envOrDefault("SERVER_ADDR", ":8080")
	log.Printf("server starting on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// buildDSN constructs the PostgreSQL connection string from environment
// variables with sensible defaults for local development.
func buildDSN() string {
	host := envOrDefault("DB_HOST", "localhost")
	port := envOrDefault("DB_PORT", "5432")
	user := envOrDefault("DB_USER", "postgres")
	password := envOrDefault("DB_PASSWORD", "postgres")
	dbname := envOrDefault("DB_NAME", "booklist")
	sslmode := envOrDefault("DB_SSLMODE", "disable")

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)
}

// connectWithRetry attempts to connect to Postgres, retrying up to maxRetries
// times with a delay between attempts. This handles the case where the app
// container starts before the database is fully ready.
func connectWithRetry(dsn string, maxRetries int, delay time.Duration) (*pgxpool.Pool, error) {
	ctx := context.Background()
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			log.Printf("db connect attempt %d/%d failed: %v – retrying in %s", attempt, maxRetries, err, delay)
			time.Sleep(delay)
			continue
		}
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			log.Printf("db ping attempt %d/%d failed: %v – retrying in %s", attempt, maxRetries, err, delay)
			time.Sleep(delay)
			continue
		}
		return pool, nil
	}
	return nil, fmt.Errorf("could not connect to database after %d attempts", maxRetries)
}

// runMigrations applies all pending up-migrations from the migrations/ directory.
func runMigrations(dsn string) error {
	// golang-migrate requires the standard library *sql.DB, not pgxpool.
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("opening sql.DB for migrations: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

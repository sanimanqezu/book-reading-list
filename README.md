# Book Reading List API

[![CI](https://github.com/sanimanqezu/book-reading-list/actions/workflows/ci.yml/badge.svg)](https://github.com/sanimanqezu/book-reading-list/actions/workflows/ci.yml)

A production-quality REST API, written in Go, for managing a personal book reading list. Add books manually or by ISBN — when only an ISBN is supplied the API auto-fetches the title, author, description, cover image, and page count from the free [Open Library](https://openlibrary.org/) database. Track reading progress, rate finished books, and pull aggregate reading statistics.

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.23 |
| Router | [Chi v5](https://github.com/go-chi/chi) (with stdlib `net/http`) |
| Database | PostgreSQL |
| Driver | [pgx/v5](https://github.com/jackc/pgx) (`pgxpool` connection pool) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Config | [godotenv](https://github.com/joho/godotenv) + environment variables |
| External data | Open Library Books API |
| Containers | Docker & Docker Compose |

## Key design points

- **Clean layered architecture** — `handlers` (HTTP) → `service` (business logic) → `repository` (data access). Each layer depends only on the one below it.
- **Repository interface** — the service depends on a `BookRepository` interface, not a concrete type. The PostgreSQL implementation is one adapter; tests substitute an in-memory fake, so the HTTP and business logic are verified without a database.
- **Validation in the service layer** — title requirement, rating bounds (1–5), non-negative page counts, and status whitelisting live in the service, keeping handlers thin and the rules reusable.
- **Sentinel errors** — the repository returns `repository.ErrNotFound`, which handlers map to `404` via `errors.Is`, cleanly separating "not found" from genuine `500` failures.
- **Graceful external dependency** — Open Library enrichment only runs when an ISBN is given without a title; if the lookup fails the request falls back to the caller-supplied fields rather than erroring.
- **Automatic progress timestamps** — moving a book to `READING` sets `started_at` and to `COMPLETED` sets `finished_at`, handled in SQL so the values are consistent.
- **Resilient startup** — the server retries the database connection on boot and runs migrations automatically, so it tolerates the DB coming up slightly later than the app.
- **Centralized PostgreSQL** — the service joins a shared `portfolio-net` Docker network and talks to a central Postgres instance rather than bundling its own.

## Running locally

### 1. Start the centralized PostgreSQL (portfolio root)

This service expects the shared Postgres from the portfolio root. From the portfolio root directory:

```bash
docker-compose up -d        # starts the centralized postgres + portfolio-net network
```

Ensure a database named `booklist` exists on that instance.

### 2. Configure environment

Copy the sample env file and adjust as needed:

```bash
cp .env.example .env
```

> **Note:** this Go service uses **`DB_USER`** (not `DB_USERNAME`).

| Variable | Default | Description |
|---|---|---|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `booklist` | Database name |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | `postgres` | Database password |
| `DB_SSLMODE` | `disable` | SSL mode (`disable` / `require`) |
| `SERVER_ADDR` | `:8080` | Address the HTTP server listens on |

### 3. Run the API

```bash
go run ./cmd/api
```

Migrations are applied automatically on startup. The API is then available at `http://localhost:8080`.

### Run the whole thing in Docker

Once the centralized Postgres and `portfolio-net` network are up, you can also run this service as a container:

```bash
docker compose up --build
```

## Tests

The suite is self-contained — it uses `net/http/httptest` and an in-memory fake repository, so **no running database or network access is required**.

```bash
go test ./...            # run everything
go test ./... -v         # verbose, per-case output
go vet ./...             # static checks
```

Coverage focuses on the layers with real logic: service-layer validation and the Open Library description parsing, plus every HTTP handler path (valid input, invalid input, not-found, and internal-error responses) routed through a Chi router that mirrors production.

## API Endpoints

### Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Returns `{"status":"ok"}` |

### Books

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/books` | Add a book (manually or by ISBN) |
| `GET` | `/api/books` | List all books (optional `?status=READING`) |
| `GET` | `/api/books/{id}` | Get a single book |
| `PUT` | `/api/books/{id}` | Update a book |
| `DELETE` | `/api/books/{id}` | Delete a book |
| `PATCH` | `/api/books/{id}/progress` | Update reading progress |
| `GET` | `/api/books/stats` | Reading statistics |

### Book status values

- `WANT_TO_READ` — on the reading list (default)
- `READING` — currently being read
- `COMPLETED` — finished

## Example requests

### Add a book by ISBN (Open Library auto-fill)

```bash
curl -X POST http://localhost:8080/api/books \
  -H "Content-Type: application/json" \
  -d '{"isbn": "9780261102354"}'
```

### Add a book manually

```bash
curl -X POST http://localhost:8080/api/books \
  -H "Content-Type: application/json" \
  -d '{
    "title": "The Pragmatic Programmer",
    "author": "David Thomas, Andrew Hunt",
    "status": "WANT_TO_READ"
  }'
```

### Update reading progress

```bash
curl -X PATCH http://localhost:8080/api/books/1/progress \
  -H "Content-Type: application/json" \
  -d '{"currentPage": 120, "status": "READING"}'
```

### Filter books by status

```bash
curl "http://localhost:8080/api/books?status=READING"
```

### Get reading statistics

```bash
curl http://localhost:8080/api/books/stats
```

Response:

```json
{
  "wantToRead": 5,
  "reading": 2,
  "completed": 10,
  "totalBooks": 17
}
```

### Rate a completed book

```bash
curl -X PUT http://localhost:8080/api/books/1 \
  -H "Content-Type: application/json" \
  -d '{
    "title": "The Lord of the Rings",
    "author": "J.R.R. Tolkien",
    "status": "COMPLETED",
    "rating": 5,
    "notes": "A timeless masterpiece."
  }'
```

## Open Library integration

When a `POST /api/books` request includes an `isbn` field **and no `title`**, the API queries the Open Library Books API:

```
GET https://openlibrary.org/api/books?bibkeys=ISBN:{isbn}&format=json&jscmd=data
```

These fields are populated from the response when not explicitly provided: `title`, `author` (first listed author), `description`, `coverUrl` (large cover image), and `totalPages`. If Open Library is unreachable the request falls back to the manually supplied fields, in which case a `title` is required.

## Project structure

```
book-reading-list/
├── cmd/api/main.go                          # Entry point — DB pool, migrations, router wiring
├── internal/
│   ├── handlers/
│   │   ├── book_handler.go                   # HTTP handlers (Chi)
│   │   └── book_handler_test.go              # httptest handler tests (routed via Chi)
│   ├── models/book.go                        # Domain structs & request/response types
│   ├── repository/book_repository.go         # BookRepository interface + pgx implementation
│   └── service/
│       ├── book_service.go                   # Business logic & Open Library client
│       └── book_service_test.go              # Service validation & parsing unit tests
├── migrations/
│   ├── 000001_create_books.up.sql
│   └── 000001_create_books.down.sql
├── Dockerfile
├── docker-compose.yml                        # Joins shared portfolio-net network
├── .env.example
└── README.md
```

## License

MIT

# Book Reading List API

A production-quality REST API built with Go that lets you manage a personal book reading list. Add books manually or by ISBN — when an ISBN is provided the API auto-fetches the book's title, author, description, cover image, and page count from the free [Open Library](https://openlibrary.org/) database.

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.21 |
| Router | [Chi v5](https://github.com/go-chi/chi) |
| Database | PostgreSQL 16 |
| Driver | [pgx/v5](https://github.com/jackc/pgx) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Containers | Docker & Docker Compose |

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose **or** Go 1.21 + a running PostgreSQL instance.

### Run with Docker (recommended)

```bash
git clone https://github.com/sanimanqezu/book-reading-list.git
cd book-reading-list
docker compose up --build
```

The API will be available at `http://localhost:8080`.

### Run locally (without Docker)

1. Start a PostgreSQL instance and create a database named `booklist`.
2. Copy the sample environment file and adjust values:

```bash
cp .env.example .env   # edit DB_* variables as needed
```

3. Install dependencies and run:

```bash
go mod tidy
go run ./cmd/api
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | `postgres` | Database password |
| `DB_NAME` | `booklist` | Database name |
| `DB_SSLMODE` | `disable` | SSL mode (`disable` / `require`) |
| `SERVER_ADDR` | `:8080` | Address the HTTP server listens on |

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

### Book Status Values

- `WANT_TO_READ` — on the reading list (default)
- `READING` — currently being read
- `COMPLETED` — finished

## Example Requests

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

## Open Library Integration

When a `POST /api/books` request includes an `isbn` field **and no `title`**, the API automatically queries the Open Library Books API:

```
GET https://openlibrary.org/api/books?bibkeys=ISBN:{isbn}&format=json&jscmd=data
```

The following fields are populated from the response when not explicitly provided:

- `title`
- `author` (first listed author)
- `description`
- `coverUrl` (large cover image URL)
- `totalPages`

If Open Library is unreachable the request falls back to the manually supplied fields. A `title` is required in that case.

## Project Structure

```
book-reading-list/
├── cmd/api/main.go                      # Entry point — DB, migrations, router
├── internal/
│   ├── handlers/book_handler.go         # HTTP handlers (Chi)
│   ├── models/book.go                   # Domain structs & request/response types
│   ├── repository/book_repository.go    # PostgreSQL data access layer (pgx)
│   └── service/book_service.go          # Business logic & Open Library client
├── migrations/
│   ├── 000001_create_books.up.sql
│   └── 000001_create_books.down.sql
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## License

MIT

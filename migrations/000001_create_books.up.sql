CREATE TABLE IF NOT EXISTS books (
    id           SERIAL PRIMARY KEY,
    title        VARCHAR(255) NOT NULL,
    author       VARCHAR(255),
    isbn         VARCHAR(20),
    description  TEXT,
    cover_url    VARCHAR(500),
    total_pages  INT,
    status       VARCHAR(20) NOT NULL DEFAULT 'WANT_TO_READ'
                     CHECK (status IN ('WANT_TO_READ', 'READING', 'COMPLETED')),
    current_page INT DEFAULT 0,
    rating       INT CHECK (rating >= 1 AND rating <= 5),
    notes        TEXT,
    started_at   TIMESTAMP,
    finished_at  TIMESTAMP,
    created_at   TIMESTAMP DEFAULT NOW()
);

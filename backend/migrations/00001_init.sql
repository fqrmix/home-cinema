-- +goose Up
CREATE TABLE downloads (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source                 TEXT NOT NULL CHECK (source IN ('rutracker', 'rutor')),
    source_title           TEXT NOT NULL,
    magnet_or_torrent_url  TEXT NOT NULL,
    transmission_hash      TEXT,
    status                 TEXT NOT NULL DEFAULT 'queued'
                               CHECK (status IN ('queued', 'downloading', 'seeding', 'completed', 'error')),
    progress_percent       NUMERIC(5,2) NOT NULL DEFAULT 0,
    download_dir           TEXT,
    error_message          TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_downloads_status ON downloads (status);
CREATE INDEX idx_downloads_transmission_hash ON downloads (transmission_hash);

CREATE TABLE movie_metadata (
    download_id     UUID PRIMARY KEY REFERENCES downloads (id) ON DELETE CASCADE,
    tmdb_id         INTEGER,
    title           TEXT,
    original_title  TEXT,
    overview        TEXT,
    poster_path     TEXT,
    release_year    SMALLINT,
    vote_average    NUMERIC(3,1),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE movie_metadata;
DROP TABLE downloads;

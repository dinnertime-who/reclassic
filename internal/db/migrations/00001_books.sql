-- 원본 5테이블. Gutenberg에서 온 불변 데이터.
-- 번역 테이블은 이 마이그레이션의 범위가 아니다 (SLICE_SKELETON 범위 밖).
-- +goose Up
-- +goose StatementBegin
CREATE TABLE books (
    id           BIGSERIAL PRIMARY KEY,
    gutenberg_id INTEGER     NOT NULL UNIQUE,
    title        TEXT        NOT NULL,
    author       TEXT,
    language     TEXT        NOT NULL DEFAULT 'en',
    status       TEXT        NOT NULL
                 CHECK (status IN ('pending','ready','needs_review','failed')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE book_sources (
    id           BIGSERIAL PRIMARY KEY,
    book_id      BIGINT      NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    s3_key       TEXT,                       -- R2 도입 전에는 NULL. FetchSource가 채운다
    content_hash TEXT        NOT NULL,       -- 원문 sha256 hex
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (book_id, content_hash)           -- 같은 원문을 두 번 저장하지 않는다
);

CREATE TABLE book_revisions (
    id             BIGSERIAL PRIMARY KEY,
    book_id        BIGINT      NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    source_id      BIGINT      NOT NULL REFERENCES book_sources(id),
    parser_version TEXT        NOT NULL,
    strategy       TEXT        NOT NULL,
    confidence     REAL        NOT NULL,
    coverage       REAL        NOT NULL,
    warnings       JSONB       NOT NULL DEFAULT '[]',
    is_active      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (book_id, source_id, parser_version)
);

-- 책당 활성 revision은 최대 하나. 애플리케이션이 아니라 스키마가 지킨다
CREATE UNIQUE INDEX book_revisions_one_active ON book_revisions (book_id) WHERE is_active;

CREATE TABLE chapters (
    id          BIGSERIAL PRIMARY KEY,
    revision_id BIGINT  NOT NULL REFERENCES book_revisions(id) ON DELETE CASCADE,
    idx         INTEGER NOT NULL,
    title       TEXT    NOT NULL DEFAULT '',
    anchor      TEXT    NOT NULL DEFAULT '',
    UNIQUE (revision_id, idx)
);

CREATE TABLE paragraphs (
    id          BIGSERIAL PRIMARY KEY,
    revision_id BIGINT  NOT NULL REFERENCES book_revisions(id) ON DELETE CASCADE,
    chapter_id  BIGINT  NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    idx         INTEGER NOT NULL,
    stable_id   TEXT    NOT NULL,
    text        TEXT    NOT NULL,
    html        TEXT    NOT NULL DEFAULT '',
    UNIQUE (chapter_id, idx),
    UNIQUE (revision_id, stable_id)          -- ADR-016이 보장하는 것을 스키마가 강제한다
);

CREATE INDEX paragraphs_stable_id ON paragraphs (stable_id);   -- 승계 매칭 조회용
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE paragraphs;
DROP TABLE chapters;
DROP TABLE book_revisions;
DROP TABLE book_sources;
DROP TABLE books;
-- +goose StatementEnd

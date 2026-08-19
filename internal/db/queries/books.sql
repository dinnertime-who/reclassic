-- 도서와 원본 스냅샷.

-- name: GetBookByGutenbergID :one
SELECT * FROM books WHERE gutenberg_id = $1;

-- name: CountBooks :one
SELECT count(*) FROM books;

-- 적재는 멱등이어야 하므로 upsert다.
-- status는 갱신하지 않는다 — 게이트 판정 결과로 뒤에서 따로 정한다.
-- name: UpsertBook :one
INSERT INTO books (gutenberg_id, title, author, language, status)
VALUES ($1, $2, $3, $4, 'pending')
ON CONFLICT (gutenberg_id) DO UPDATE
   SET title      = EXCLUDED.title,
       author     = COALESCE(EXCLUDED.author, books.author),
       language   = EXCLUDED.language,
       updated_at = now()
RETURNING *;

-- name: SetBookStatus :exec
UPDATE books SET status = $2, updated_at = now() WHERE id = $1;

-- 같은 원문을 두 번 저장하지 않는다. content_hash가 같으면 기존 행을 돌려준다.
-- name: UpsertBookSource :one
INSERT INTO book_sources (book_id, s3_key, content_hash)
VALUES ($1, $2, $3)
ON CONFLICT (book_id, content_hash) DO UPDATE
   SET s3_key = COALESCE(EXCLUDED.s3_key, book_sources.s3_key)
RETURNING *;

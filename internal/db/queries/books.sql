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

-- 목록. 상태 필터는 선택이다 — needs_review 큐와 전체를 한 쿼리로 본다 (D1).
-- 챕터·문단 수는 최신 revision 기준. 게이트에 걸린 책은 is_active가 아니므로
-- 활성을 기다리지 않는다. 화면이 ADR-014 임계값과 나란히 보여 줘야 한다.
-- name: ListBooks :many
SELECT
    b.id,
    b.gutenberg_id,
    b.title,
    b.author,
    b.language,
    b.status,
    COALESCE(stats.chapter_count, 0)::bigint    AS chapter_count,
    COALESCE(stats.paragraph_count, 0)::bigint  AS paragraph_count
FROM books b
LEFT JOIN LATERAL (
    SELECT
        (SELECT count(*) FROM chapters c WHERE c.revision_id = r.id)   AS chapter_count,
        (SELECT count(*) FROM paragraphs p WHERE p.revision_id = r.id) AS paragraph_count
    FROM book_revisions r
    WHERE r.book_id = b.id
    ORDER BY r.created_at DESC
    LIMIT 1
) stats ON true
WHERE sqlc.narg('status')::text IS NULL OR b.status = sqlc.narg('status')
ORDER BY b.gutenberg_id;

-- 공개 도서 목록. published 프로젝트만 행이 된다 (ADR-036).
-- 한 책에 대상 언어가 여럿이면 행이 여러 개다.
-- name: ListPublishedCatalog :many
SELECT
    b.gutenberg_id,
    b.title,
    b.author,
    p.id          AS project_id,
    p.target_lang
FROM translation_projects p
JOIN books b ON b.id = p.book_id
WHERE p.status = 'published'
ORDER BY b.title, p.target_lang;

-- 같은 원문을 두 번 저장하지 않는다. content_hash가 같으면 기존 행을 돌려준다.
-- name: UpsertBookSource :one
INSERT INTO book_sources (book_id, s3_key, content_hash)
VALUES ($1, $2, $3)
ON CONFLICT (book_id, content_hash) DO UPDATE
   SET s3_key = COALESCE(EXCLUDED.s3_key, book_sources.s3_key)
RETURNING *;

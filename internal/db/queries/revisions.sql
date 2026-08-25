-- 파싱 결과. 불변으로 쌓고 활성 revision만 읽기에 노출한다 (ADR-004).

-- name: FindRevision :one
SELECT * FROM book_revisions
WHERE book_id = $1 AND source_id = $2 AND parser_version = $3;

-- name: InsertRevision :one
INSERT INTO book_revisions (
    book_id, source_id, parser_version, strategy, confidence, coverage, warnings
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- 책당 활성 revision은 하나다. 부분 유니크 인덱스가 강제하므로
-- 새 revision을 켜기 전에 기존 것을 반드시 먼저 끈다.
-- name: DeactivateRevisions :exec
UPDATE book_revisions SET is_active = FALSE WHERE book_id = $1 AND is_active;

-- name: ActivateRevision :exec
UPDATE book_revisions SET is_active = TRUE WHERE id = $1;

-- name: GetActiveRevision :one
SELECT r.* FROM book_revisions r
JOIN books b ON b.id = r.book_id
WHERE b.gutenberg_id = $1 AND r.is_active;

-- name: InsertChapter :one
INSERT INTO chapters (revision_id, idx, title, anchor)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- 문단은 한 권에 수만 건이라 COPY로 넣는다.
-- name: InsertParagraphs :copyfrom
INSERT INTO paragraphs (revision_id, chapter_id, idx, stable_id, text, html)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: CountChapters :one
SELECT count(*) FROM chapters WHERE revision_id = $1;

-- name: GetChapter :one
SELECT * FROM chapters WHERE revision_id = $1 AND idx = $2;

-- name: ListParagraphsByChapter :many
SELECT stable_id, text FROM paragraphs WHERE chapter_id = $1 ORDER BY idx;

-- 승계 매칭률 측정용. 저장된 revision의 stable_id 전체를 읽는다.
-- name: ListStableIDs :many
SELECT stable_id FROM paragraphs WHERE revision_id = $1;

-- 고아 승계 목록. 부분 인덱스 revision_successions_orphaned (book_id)
-- WHERE orphaned > 0 를 탄다. 읽기만 한다 — 되살리는 조작은 ADR-016에 닿는다.
-- name: ListOrphanedSuccessions :many
SELECT
    s.id,
    s.book_id,
    b.gutenberg_id,
    b.title,
    s.orphaned,
    s.created_at
FROM revision_successions s
JOIN books b ON b.id = s.book_id
WHERE s.orphaned > 0
ORDER BY s.created_at DESC;

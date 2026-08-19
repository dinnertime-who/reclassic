-- 골격 슬라이스의 최소 쿼리. 적재 쿼리는 SLICE_READ_PATH.md에서 추가한다.

-- name: GetBookByGutenbergID :one
SELECT * FROM books WHERE gutenberg_id = $1;

-- name: CountBooks :one
SELECT count(*) FROM books;

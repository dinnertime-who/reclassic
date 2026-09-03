-- 번역 제안과 검수.
-- paragraph_stable_id는 FK가 아니다 (ADR-004). 재파싱해도 번역이 살아남게 하는 대가다.

-- name: GetUserByHandle :one
SELECT * FROM users WHERE handle = $1;

-- name: UpsertUser :one
INSERT INTO users (handle, display_name, role)
VALUES ($1, $2, $3)
ON CONFLICT (handle) DO UPDATE SET display_name = EXCLUDED.display_name
RETURNING *;

-- name: CreateProject :one
INSERT INTO translation_projects (book_id, target_lang)
VALUES ($1, $2)
ON CONFLICT (book_id, target_lang) DO UPDATE SET target_lang = EXCLUDED.target_lang
RETURNING *;

-- name: GetProject :one
SELECT * FROM translation_projects WHERE id = $1;

-- name: GetProjectWithBook :one
SELECT sqlc.embed(p), sqlc.embed(b)
FROM translation_projects p JOIN books b ON b.id = p.book_id
WHERE p.id = $1;

-- 번역 프로젝트 목록. 상태 필터는 선택이다 — 공개 목록은 published,
-- 관리자 목록은 필터 없이 open도 본다 (D4 · D5 · ADR-036).
-- name: ListProjects :many
SELECT
    p.id,
    p.book_id,
    p.target_lang,
    p.status,
    p.published_at,
    b.gutenberg_id,
    b.title,
    b.author
FROM translation_projects p
JOIN books b ON b.id = p.book_id
WHERE sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')
ORDER BY b.title, p.target_lang;

-- 공개 전이. published_at은 처음 published가 된 시각만 찍고,
-- open으로 내려올 때 비우지 않는다 (ADR-036).
-- name: SetProjectStatus :one
UPDATE translation_projects
   SET status = sqlc.arg('status'),
       published_at = CASE
           WHEN sqlc.arg('status') = 'published' AND published_at IS NULL THEN now()
           ELSE published_at
       END
 WHERE id = sqlc.arg('id')
RETURNING *;

-- name: CreateProposal :one
INSERT INTO translation_proposals (project_id, paragraph_stable_id, text, author_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetProposal :one
SELECT * FROM translation_proposals WHERE id = $1;

-- 제안 목록은 작성자 handle까지 조인해 내려준다.
-- 문단마다 사용자를 따로 조회하면 N+1이다.
-- name: ListProposals :many
SELECT p.*, u.handle AS author_handle
FROM translation_proposals p
JOIN users u ON u.id = p.author_id
WHERE p.project_id = $1 AND p.paragraph_stable_id = $2
ORDER BY p.created_at DESC;

-- name: WithdrawProposal :one
UPDATE translation_proposals
   SET status = 'withdrawn'
 WHERE id = $1 AND author_id = $2 AND status = 'pending'
RETURNING *;

-- name: RejectProposal :one
UPDATE translation_proposals
   SET status = 'rejected', reviewed_by = $2, reviewed_at = now(), review_note = $3
 WHERE id = $1 AND status = 'pending'
RETURNING *;

-- 승인 3단계 (ARCHITECTURE 불변식 3). 반드시 한 트랜잭션 안에서 순서대로 부른다.

-- 0. 같은 문단의 승인을 직렬화한다.
--
--    3단계만으로는 부족하다. 두 검수자가 같은 문단의 **서로 다른** 제안을 동시에 승인하면
--    둘 다 1단계에서 "기존 확정본 없음"을 보고 아무것도 supersede하지 않는다.
--    그 결과 확정본은 1행인데(복합 PK가 지킨다) approved 제안이 2개 남는다.
--    스트레스 테스트에서 40라운드 중 거의 전부가 그렇게 됐다. 근거는 ADR-024.
--
--    WHERE status='pending'은 **같은 제안**의 동시 승인만 막는다. 이건 다른 문제다.
-- name: LockParagraphForReview :exec
SELECT pg_advisory_xact_lock(
    hashtextextended(sqlc.arg(project_key)::text || ':' || sqlc.arg(stable_id)::text, 0));

-- 1. 기존 확정본의 제안을 superseded로.
-- name: SupersedeCurrentProposal :exec
UPDATE translation_proposals SET status = 'superseded'
 WHERE id = (SELECT pt.proposal_id FROM paragraph_translations pt
              WHERE pt.project_id = $1 AND pt.paragraph_stable_id = $2);

-- 2. 새 제안 승인. status 조건이 동시 승인을 막는다 — 빼지 말 것.
--    영향 행이 0이면 다른 검수자가 먼저 처리한 것이다. 롤백하고 409.
-- name: ApproveProposal :one
UPDATE translation_proposals
   SET status = 'approved', reviewed_by = $2, reviewed_at = now(), review_note = $3
 WHERE id = $1 AND status = 'pending'
RETURNING *;

-- 3. 확정본 교체.
-- name: UpsertParagraphTranslation :one
INSERT INTO paragraph_translations
       (project_id, paragraph_stable_id, text, proposal_id, approved_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id, paragraph_stable_id) DO UPDATE
   SET text = EXCLUDED.text,
       proposal_id = EXCLUDED.proposal_id,
       approved_by = EXCLUDED.approved_by,
       approved_at = now()
RETURNING *;

-- 읽기 화면. 확정 번역과 제안 수를 챕터 단위로 한 번에 조인해 내려준다 —
-- 문단마다 따로 조회하면 N+1이 그대로 나간다.
-- name: ListChapterParagraphsWithTranslation :many
SELECT p.stable_id,
       p.text AS source_text,
       pt.text AS approved_translation,
       (SELECT count(*) FROM translation_proposals tp
         WHERE tp.project_id = $2 AND tp.paragraph_stable_id = p.stable_id
           AND tp.status = 'pending') AS proposal_count
FROM paragraphs p
LEFT JOIN paragraph_translations pt
       ON pt.project_id = $2 AND pt.paragraph_stable_id = p.stable_id
WHERE p.chapter_id = $1
ORDER BY p.idx;

-- 색인 판정용 커버리지 (ADR-023). 챕터 단위다.
-- name: ChapterCoverage :one
SELECT count(*)                                   AS total,
       count(pt.paragraph_stable_id)              AS approved
FROM paragraphs p
LEFT JOIN paragraph_translations pt
       ON pt.project_id = $2 AND pt.paragraph_stable_id = p.stable_id
WHERE p.chapter_id = $1;

-- 승계 실행: 새 revision에 없는 stable_id 중 확정 번역이 붙어 있던 것.
-- 조용히 버리지 않는다 — 사람이 쓴 것이다.
-- name: FindOrphanedTranslations :many
SELECT pt.paragraph_stable_id
FROM paragraph_translations pt
JOIN translation_projects tp ON tp.id = pt.project_id
WHERE tp.book_id = $1
  AND NOT EXISTS (
      SELECT 1 FROM paragraphs p
       WHERE p.revision_id = $2 AND p.stable_id = pt.paragraph_stable_id)
ORDER BY pt.paragraph_stable_id;

-- name: RecordSuccession :one
INSERT INTO revision_successions
       (book_id, from_revision_id, to_revision_id, matched, added, lost, orphaned, orphan_ids)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- 사이트맵과 진행률용. 목차 화면도 같은 숫자를 쓴다 —
-- 장 단위 번역 커버리지(ADR-023)는 색인 판정과 목차 진행도가 공유하는 값이다.
-- title은 목차만 쓰지만 컬럼 하나를 위해 같은 조인을 한 번 더 하지 않는다.
-- name: ListProjectChapterCoverage :many
SELECT c.idx,
       c.title,
       count(p.id)                    AS total,
       count(pt.paragraph_stable_id)  AS approved
FROM chapters c
JOIN book_revisions r ON r.id = c.revision_id AND r.is_active
JOIN translation_projects tp ON tp.id = $1 AND tp.book_id = r.book_id
LEFT JOIN paragraphs p ON p.chapter_id = c.id
LEFT JOIN paragraph_translations pt
       ON pt.project_id = tp.id AND pt.paragraph_stable_id = p.stable_id
GROUP BY c.idx, c.title
ORDER BY c.idx;

-- 인증 (슬라이스 5).

-- google_sub으로 찾는다. 이메일은 바뀌므로 식별자로 쓰지 않는다.
-- name: GetUserByGoogleSub :one
SELECT * FROM users WHERE google_sub = $1;

-- name: CreateGoogleUser :one
INSERT INTO users (handle, display_name, role, email, google_sub)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateUserFromGoogle :one
UPDATE users
   SET display_name = $2, email = $3, role = $4
 WHERE id = $1
RETURNING *;

-- handle 충돌 확인용.
-- name: HandleExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE handle = $1);

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, expires_at, user_agent)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- 만료된 세션은 조회 단계에서 걸러낸다. 정리 잡을 기다리지 않는다.
-- name: GetSessionUser :one
SELECT sqlc.embed(u) FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id = $1 AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= now();

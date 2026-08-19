-- 번역 — 사용자가 만드는 가변 데이터.
-- 원본 테이블(00001)과 달리 여기는 사람이 쓴 것이 쌓인다. 조용히 버리지 않는다.
-- +goose Up
-- +goose StatementBegin

-- 세션은 다음 슬라이스다. 여기서는 신원의 뼈대만 만든다.
CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    handle       TEXT        NOT NULL UNIQUE,
    display_name TEXT        NOT NULL DEFAULT '',
    role         TEXT        NOT NULL DEFAULT 'member'
                 CHECK (role IN ('member','reviewer','admin')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE translation_projects (
    id           BIGSERIAL PRIMARY KEY,
    book_id      BIGINT      NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    target_lang  TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'open'
                 CHECK (status IN ('open','published','archived')),
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (book_id, target_lang)
);

-- 문단당 N개. 제안은 쌓인다.
CREATE TABLE translation_proposals (
    id                  BIGSERIAL PRIMARY KEY,
    project_id          BIGINT      NOT NULL REFERENCES translation_projects(id) ON DELETE CASCADE,
    paragraph_stable_id TEXT        NOT NULL,   -- FK가 아니다. ADR-004를 볼 것
    text                TEXT        NOT NULL,
    author_id           BIGINT      NOT NULL REFERENCES users(id),
    status              TEXT        NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','approved','rejected','superseded','withdrawn')),
    reviewed_by         BIGINT      REFERENCES users(id),
    reviewed_at         TIMESTAMPTZ,
    review_note         TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX translation_proposals_lookup
    ON translation_proposals (project_id, paragraph_stable_id, status);

-- 같은 사람이 같은 문단에 대기 중인 제안을 둘 이상 두지 않는다.
CREATE UNIQUE INDEX translation_proposals_one_pending_per_author
    ON translation_proposals (project_id, paragraph_stable_id, author_id)
    WHERE status = 'pending';

-- 문단당 0 또는 1. 공개되는 것.
-- 복합 PK가 불변식 2를 강제한다 — 애플리케이션이 아니라 스키마가 지킨다 (ADR-005).
CREATE TABLE paragraph_translations (
    project_id          BIGINT      NOT NULL REFERENCES translation_projects(id) ON DELETE CASCADE,
    paragraph_stable_id TEXT        NOT NULL,
    text                TEXT        NOT NULL,
    proposal_id         BIGINT      NOT NULL REFERENCES translation_proposals(id),
    approved_by         BIGINT      NOT NULL REFERENCES users(id),
    approved_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, paragraph_stable_id)
);

-- 인명·지명·호칭 일관성. 여러 사용자가 같은 책을 번역할 때 필요하다 (ADR-010).
CREATE TABLE book_glossary (
    id          BIGSERIAL PRIMARY KEY,
    book_id     BIGINT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    source_term TEXT   NOT NULL,
    target_term TEXT   NOT NULL,
    note        TEXT   NOT NULL DEFAULT '',
    UNIQUE (book_id, source_term)
);

-- revision 전환 로그. 번역이 갈 곳을 잃었는지 사람이 확인할 수 있어야 한다.
CREATE TABLE revision_successions (
    id               BIGSERIAL PRIMARY KEY,
    book_id          BIGINT      NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    from_revision_id BIGINT      REFERENCES book_revisions(id) ON DELETE SET NULL,
    to_revision_id   BIGINT      NOT NULL REFERENCES book_revisions(id) ON DELETE CASCADE,
    matched          INTEGER     NOT NULL,
    added            INTEGER     NOT NULL,
    lost             INTEGER     NOT NULL,
    -- 소실된 stable_id 중 확정 번역이 붙어 있던 것. 조용히 버리지 않는다.
    orphaned         INTEGER     NOT NULL,
    orphan_ids       JSONB       NOT NULL DEFAULT '[]',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX revision_successions_orphaned
    ON revision_successions (book_id) WHERE orphaned > 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE revision_successions;
DROP TABLE book_glossary;
DROP TABLE paragraph_translations;
DROP TABLE translation_proposals;
DROP TABLE translation_projects;
DROP TABLE users;
-- +goose StatementEnd

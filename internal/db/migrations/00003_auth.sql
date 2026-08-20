-- 세션 인증 (Google 로그인). 비밀번호를 다루지 않는다.
-- +goose Up
-- +goose StatementBegin

-- Google 식별자. sub이 안정 식별자이고 이메일은 바뀔 수 있다.
ALTER TABLE users ADD COLUMN email      TEXT;
ALTER TABLE users ADD COLUMN google_sub TEXT;

CREATE UNIQUE INDEX users_google_sub ON users (google_sub) WHERE google_sub IS NOT NULL;
CREATE UNIQUE INDEX users_email      ON users (email)      WHERE email IS NOT NULL;

CREATE TABLE sessions (
    -- 세션 토큰의 sha256이다. 토큰 원본이 아니다 —
    -- DB가 유출돼도 그 값으로 로그인할 수 없어야 한다.
    id         TEXT        PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_agent TEXT        NOT NULL DEFAULT ''
);

-- 사용자의 모든 세션을 한 번에 끊기 위한 조회.
CREATE INDEX sessions_user_id ON sessions (user_id);
-- 만료분 정리용.
CREATE INDEX sessions_expires_at ON sessions (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE sessions;
DROP INDEX users_email;
DROP INDEX users_google_sub;
ALTER TABLE users DROP COLUMN google_sub;
ALTER TABLE users DROP COLUMN email;
-- +goose StatementEnd

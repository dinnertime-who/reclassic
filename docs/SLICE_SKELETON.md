# 슬라이스 명세 — 프로젝트 골격 (walking skeleton)

**작업 종류:** 구현 슬라이스
**선행 문서:** `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/CONVENTIONS.md`,
`docs/DECISIONS.md` (특히 ADR-001 / 002 / 006 / 009 / 017 / 018 / 019)
**다음 슬라이스:** `docs/SLICE_PERSISTENCE.md`

---

## 1. 이 작업이 답해야 할 질문

파서는 검증됐지만 그 외에는 아무것도 서 있지 않다.
`ARCHITECTURE.md`가 예고한 디렉토리 8개가 없고, `make generate` / `migrate` / `dev`가 전부 죽어 있다.

> **한 저장소 안에서 도구 여섯이 실제로 맞물려 도는가?**
> goose · sqlc · oapi-codegen · orval · docker compose · TanStack Start

동시에 **기존 ADR 두 개의 전제**를 처음으로 실물 검증한다.

| 검증 대상 | 근거 ADR | 확인 방법 |
|---|---|---|
| "계약 변경이 양쪽에서 컴파일 에러로 드러난다" | ADR-009 | `openapi.yaml`을 고치고 Go·TS 양쪽이 깨지는지 본다 |
| "TanStack Start SSR로 검색 노출을 만든다" | ADR-006 | 브라우저 없이 받은 HTML에 API 데이터가 들어 있는지 본다 |

**TanStack Start v1.0은 2026-03 릴리스다.** 이 프로젝트에서 아직 한 줄도 쓰지 않았다.
프레임워크 선택이 틀렸다면 여기서 드러나야 한다. 기능을 얹은 뒤에 알면 비싸다.

**기능은 만들지 않는다. 배선만 한다.**

---

## 2. 범위

### 하는 것

- `ARCHITECTURE.md`가 예고한 디렉토리와 설정 파일 생성
- 원본 5테이블 마이그레이션 + goose 러너 → `make migrate` 복구
- sqlc 설정과 최소 쿼리 → `make generate` 복구
- `openapi.yaml`에 **엔드포인트 하나**(`GET /healthz`) → oapi-codegen(chi) + orval 양방향 생성
- `cmd/api` — chi 라우터, `[::]` 바인딩, `log/slog`, 미들웨어 3종
- `cmd/worker` — 기동과 graceful shutdown만. 잡 소비 없음
- `web/` — TanStack Start. `/healthz`를 **SSR로 호출해 화면에 표시**
- `docker-compose.yml` → `make dev` 복구

### 하지 않는 것 — 건드리면 안 됨

- **파서 결과 적재.** 테이블은 만들되 비어 있다. 적재는 `SLICE_PERSISTENCE.md`
- **River 잡 큐, R2, FetchSource**
- **번역 테이블 4개**
- **인증·세션.** 미들웨어 자리만 비워 둔다
- **화면 디자인.** 값이 보이면 된다
- 파서 로직 변경

**엔드포인트를 두 개 만들지 말 것.** 하나로 검증되지 않는 것은 열 개로도 검증되지 않는다.

### 의존성

```
github.com/go-chi/chi/v5       ADR-018
github.com/jackc/pgx/v5        ADR-017
github.com/pressly/goose/v3    ADR-017
```

Node 쪽은 ADR-019에서 정했다.

```
pnpm                           패키지 매니저. packageManager 필드로 버전 고정, corepack 활성화
web/package.json               저장소의 유일한 Node 패키지. 루트에 package.json을 만들지 않는다
orval                          web/의 devDependency. 설정에서 ../openapi.yaml 참조
```

빌드 타임 도구: `sqlc`, `oapi-codegen`(Go), `orval`(pnpm). **`make doctor`에 확인 항목을 추가한다.**
이 밖에 필요하면 추가 전에 `Proposed` ADR로 남기고 확인할 것.

---

## 3. 산출물

| 경로 | 내용 |
|---|---|
| `openapi.yaml` | API 계약 단일 원본. `GET /healthz` 하나 |
| `sqlc.yaml` / `oapi-codegen.yaml` | 생성 설정 |
| `docker-compose.yml` | 로컬 Postgres |
| `internal/db/migrations/00001_books.sql` | 원본 5테이블. **적용 후 수정 금지** |
| `internal/db/migrate.go` | goose 임베드 러너 |
| `internal/db/queries/*.sql` | sqlc 입력 |
| `internal/db/gen/` | sqlc 산출물. **손대지 않음** |
| `internal/api/gen/` | oapi-codegen 산출물. **손대지 않음** |
| `internal/api/server.go` | 핸들러 구현과 라우터 조립 |
| `cmd/api/main.go` | 얇게. 설정 읽고 서버 기동만 |
| `cmd/worker/main.go` | 기동과 종료만 |
| `web/package.json` · `web/pnpm-lock.yaml` | 저장소의 유일한 Node 패키지 (ADR-019) |
| `web/orval.config.ts` | `../openapi.yaml` → TS 클라이언트 |
| `web/` | TanStack Start 앱 |
| Makefile · `AGENTS.md` 명령어 표 | 함께 갱신 |

---

## 4. 구현 명세

### 4.1 디렉토리

`ARCHITECTURE.md`의 레이아웃을 따르되 **`internal/api/`를 추가한다.**
생성된 서버 인터페이스와 핸들러가 있을 곳이 레이아웃에 없었다.
`cmd/`는 진입점만 두고 로직은 `internal/`에 둔다는 기존 규약에 맞춘다.
**이 추가는 `ARCHITECTURE.md` 디렉토리 절에 반영할 것.**

### 4.2 스키마

원본 5테이블. 제약까지 확정본이다.
`ARCHITECTURE.md`의 데이터 모델 개요와 다른 두 곳은 §4.3에 근거를 적었다.

```sql
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
```

### 4.3 ARCHITECTURE 개요와 다른 두 곳 — 근거

**1. `paragraphs.revision_id` 추가 (비정규화)**
개요에는 `chapter_id`만 있다. 그러면 `UNIQUE (revision_id, stable_id)`를 표현할 수 없다.
ADR-016이 "한 revision 안에서 `stable_id`는 유일"을 보장하는데,
불변식 2가 "애플리케이션 로직이 아니라 스키마가 지키게 한다"고 정한 이상 DB가 강제해야 한다.

**2. `book_revisions`에 `strategy` / `coverage` / `warnings` 추가**
관리자 확인 큐에서 "왜 이 책이 걸렸는지" 판단하려면 필요하다. 저장하지 않으면 다시 파싱해야 한다.

**두 변경 모두 `ARCHITECTURE.md` 데이터 모델 절에 반영할 것.** 문서와 스키마가 갈라지면 안 된다.

### 4.4 `openapi.yaml`

엔드포인트 하나. 응답을 **타입으로** 정의해야 양쪽 생성기가 타입을 만든다.

```
GET /healthz  →  200  { status: "ok", db: "ok" | "down", version: string }
```

`db`는 실제 `Ping` 결과를 반영한다. 상수를 돌려주면 DB 배선을 검증하지 못한다.

### 4.5 `cmd/api`

- 라우터는 chi. oapi-codegen 생성 대상은 `chi-server` (ADR-018)
- **`[::]`에 바인딩한다** (불변식 4). `0.0.0.0`은 Railway 내부 호출을 못 받는다
- 미들웨어 3종: 패닉 복구 / 요청 ID / `log/slog` 요청 로깅.
  **인증 미들웨어는 자리만 비워 둔다** — 이번 범위가 아니다
- `DATABASE_URL`이 없으면 **기동 시점에 실패**시킨다. 기본값을 조용히 채우지 않는다 (CONVENTIONS)
- `cmd/api/main.go`는 얇게. 라우터 조립과 핸들러는 `internal/api/`

### 4.6 `cmd/worker`

기동해서 시작 로그를 남기고 SIGTERM까지 대기한다. **잡 소비는 없다.**
River는 다음다음 슬라이스다. 배포 단위가 둘이라는 것(ADR-001)만 확인한다.

### 4.7 `web/`

- TanStack Start. `/healthz`를 **SSR에서** 호출해 화면에 값을 표시한다
- 패키지 매니저는 **pnpm**. 루트에 `package.json`을 만들지 않는다 (ADR-019)
- API 호출은 **orval 생성 클라이언트만** 쓴다. 직접 `fetch` 금지 (CONVENTIONS)
- 베이스 URL은 서버·클라이언트에서 분기한다. CONVENTIONS의 코드 조각을 그대로 쓴다
- **server function에 로직을 넣지 않는다.** 데이터 페칭과 쿠키 전달 전용 (ADR-006)

### 4.8 Makefile

`generate`(sqlc → oapi-codegen → orval 순), `migrate`, `dev`를 되살리고
`AGENTS.md` 명령어 표를 함께 갱신한다. **Makefile이 유일한 진입점**이다.
orval은 `web/` 안에서 실행되지만 진입점은 `make generate`다. `cd web && pnpm ...`을
직접 치게 하지 않는다.

**`make doctor`도 함께 고친다.** 지금은 go·docker·node·sqlc·golangci-lint만 본다.
`pnpm`과 `oapi-codegen`이 빠져 있고, `sqlc`는 "(아직 불필요)"로 표시돼 있는데 이제 필요하다.

---

## 5. 반드시 지킬 것

1. **생성 코드(sqlc / oapi-codegen / orval)는 손으로 고치지 않는다.**
   잘못됐으면 SQL이나 `openapi.yaml`을 고치고 다시 생성한다.
2. **API 변경은 `openapi.yaml`부터.** 핸들러나 TS 클라이언트를 먼저 고치면 다음 생성에서 덮어써진다.
3. **`[::]`에 바인딩한다.** `0.0.0.0` 금지.
4. **적용된 마이그레이션 파일은 수정하지 않는다.** 항상 새 파일을 추가한다.
5. **환경변수에 기본값을 조용히 채우지 않는다.** 없으면 기동 시점에 실패.
6. **`DATABASE_URL` 없이 `make test`가 통과해야 한다.** DB가 필요한 테스트는 `t.Skip`.
7. **범위 밖(적재·River·R2·번역·인증)을 건드리지 않는다.**
8. 작업 종료 전 `make lint && make test` 통과.

---

## 6. 완료 조건

- [ ] `make doctor`가 go·docker·node·**pnpm**·**sqlc**·**oapi-codegen**·golangci-lint를 확인한다
- [ ] `make dev`로 Postgres가 뜨고 `make migrate`가 5테이블을 만든다
- [ ] `make generate`가 sqlc · oapi-codegen · orval 산출물을 모두 만든다
- [ ] `make build`가 `cmd/api`, `cmd/worker`, `cmd/parsecheck` 세 바이너리를 만든다
- [ ] `cmd/api`가 `[::]`에 바인딩하고 `GET /healthz`가 실제 DB Ping 결과를 돌려준다
- [ ] `cmd/worker`가 기동하고 SIGTERM에 정상 종료한다
- [ ] `web/`이 SSR로 `/healthz`를 호출하고, **`curl`로 받은 HTML에 그 값이 들어 있다**
      (자바스크립트 실행 없이 보여야 SSR이다)
- [ ] **`openapi.yaml`의 응답 필드 하나를 지우면 Go와 TS 양쪽이 컴파일 에러가 난다** (ADR-009 검증)
- [ ] `DATABASE_URL` 없이 `make test` 통과
- [ ] `ARCHITECTURE.md`에 `internal/api/`와 §4.3의 스키마 두 변경이 반영됐다
- [ ] `AGENTS.md` 기술 스택 표에 chi가, 명령어 표에 새 명령이 반영됐다
- [ ] `make lint && make test` 통과

---

## 7. 다음

`docs/SLICE_PERSISTENCE.md` — 파서 결과 적재와 `stable_id` 승계 매칭률 측정.
이 골격의 빈 테이블을 채우는 작업이다.

# 슬라이스 명세 — 수집 자동화 (River + R2)

**작업 종류:** 구현 슬라이스 (수직)
**선행 슬라이스:** `docs/slices/completed/read-path.md` — 적재 로직(`internal/book.Ingester`)이 있어야 한다.
이 슬라이스는 그것을 CLI가 아니라 잡 핸들러가 부르게 만든다.
**선행 문서:** `AGENTS.md`, `docs/ARCHITECTURE.md` (특히 "수집 파이프라인"),
`docs/decisions/index.md` (특히 ADR-001 / 003 / 008 / 014)
**다음 슬라이스:** `docs/slices/completed/translation.md`

---

## 1. 이 작업이 답해야 할 질문

지금 수집은 손으로 한다. `make fetch-corpus`로 내려받고 `make ingest`로 적재한다.
관리자가 책을 고르면 나머지가 알아서 되는 경로가 없다.

> **1. River가 정말 Postgres 트랜잭션 안에서 enqueue 되는가?**
> **2. 잡을 잘게 나눈 파이프라인이 실패에서 회복되는가?**
> **3. R2에 원본이 올라가고, 재파싱 때 다시 읽히는가?**

1번이 ADR-003이 Redis 대신 River를 고른 **유일한 근거**다.
"책 레코드는 생성됐는데 잡 등록이 실패"하는 틈이 없다고 적었지만 확인된 적은 없다.
근거가 틀렸다면 여기서 드러나야 한다.

3번은 ADR-008의 전제다. R2는 S3 호환이라 `aws-sdk-go-v2`를 그대로 쓴다고 했다.
로컬에서는 MinIO로 같은 프로토콜을 확인한다 — 실제 R2 자격증명 없이 배선을 검증하기 위함이다.

---

## 2. 범위

### 하는 것

- River 마이그레이션과 워커 기동. `cmd/worker`가 실제로 잡을 소비한다
- `internal/storage/` — S3 호환 어댑터 (R2 / MinIO)
- `FetchSource` 잡 — Gutenberg 원문 요청 → R2 저장 → `book_sources` 기록
- `ParseBook` 잡 — R2에서 읽어 기존 `internal/book.Ingester` 호출
- 관리자 엔드포인트 **하나** — `POST /admin/books`.
  **한 트랜잭션 안에서** `books` 행 생성과 `FetchSource` enqueue를 함께 한다
- `docker-compose.yml`에 MinIO

### 하지 않는 것 — 건드리면 안 됨

- **번역 테이블 4개, 제안·검수.** 다음 슬라이스
- **사이트맵 생성.** 색인 기준이 번역 커버리지라 번역 슬라이스에 붙는다
- **Gutendex 카탈로그 조회·검색 화면.** 관리자는 도서 번호를 직접 넣는다
- **세션 인증.** 관리자 엔드포인트는 환경변수 토큰으로 임시로 막는다 (§4.6)
- **도서 목록 화면**
- 파서 로직 변경. `internal/book.Ingester`도 고치지 않는다 — 호출자만 바뀐다

### 의존성 — 새로 추가한다

| 패키지 | 근거 |
|---|---|
| `github.com/riverqueue/river` | ADR-003 |
| `github.com/riverqueue/river/riverdriver/riverpgxv5` | 같음. pgx/v5 드라이버 (ADR-017) |
| `github.com/aws/aws-sdk-go-v2/...` (config, credentials, service/s3) | ADR-008이 명시 |

셋 다 기존 ADR이 이미 고른 것이라 새 결정이 아니다.
**River 마이그레이션과 goose의 공존 순서는 새 결정이다** — ADR-022에 남긴다.

---

## 3. 산출물

| 경로 | 내용 |
|---|---|
| `internal/storage/s3.go` | S3 호환 어댑터. 인터페이스 뒤에 둔다 |
| `internal/jobs/fetch_source.go` | FetchSource 잡 |
| `internal/jobs/parse_book.go` | ParseBook 잡 |
| `internal/jobs/client.go` | River 클라이언트 조립 |
| `internal/book/enqueue.go` | 트랜잭션 안에서 books 행 + 잡 등록 |
| `openapi.yaml` | `POST /admin/books` |
| `cmd/worker/main.go` | River 워커 기동 |
| `docker-compose.yml` | MinIO 추가 |
| `.env.example` | MinIO 로컬 값, `ADMIN_TOKEN` (ADR-027에서 제거됨) |
| Makefile · `AGENTS.md` | 함께 갱신 |

---

## 4. 구현 명세

### 4.1 마이그레이션 순서 — goose 먼저, River 나중

ADR-017이 "River를 붙일 때 적용 순서를 그때 명시한다"고 남겨 둔 항목이다.
**`make migrate`는 goose를 돌린 뒤 River 마이그레이션을 돌린다.** 근거는 ADR-022.

### 4.2 잡 파이프라인

`ARCHITECTURE.md` "수집 파이프라인" 그대로다. 잡을 잘게 나눈다 — 재시도가 쉽다.

```
POST /admin/books
  └─ (트랜잭션) books 행 생성 + FetchSource enqueue
       └─ FetchSource : 원문 HTML 요청 → R2 저장 → book_sources 기록 → ParseBook enqueue
            └─ ParseBook : R2에서 읽어 Ingester 호출 → revision + chapters + paragraphs
```

- 각 잡은 **멱등**이어야 한다. River가 재시도하기 때문이다.
  `book_sources`의 `UNIQUE (book_id, content_hash)`와 `book_revisions`의
  `UNIQUE (book_id, source_id, parser_version)`이 그것을 받아낸다.
- 핸들러는 **패닉하지 않고 에러를 반환**한다. River가 재시도하게 둔다 (CONVENTIONS).
- **4xx는 재시도하지 않는다.** River의 `JobCancel`로 즉시 취소하고 `books.status='failed'`.

### 4.3 Gutenberg 수집 규칙 — 완화하지 말 것

`internal/gutenberg.Client`가 이미 직렬화와 최소 간격을 지킨다. 잡에서도 그것을 쓴다.

- **FetchSource 큐의 동시성은 1이다.** River 워커를 여러 개 띄워도 이 큐만은 1이어야 한다.
  큐를 나누는 이유가 그것이다 — 파싱은 병렬로 돌려도 되지만 수집은 안 된다.
- User-Agent 없으면 기동 거부.

### 4.4 R2 어댑터

```go
type ObjectStore interface {
    Put(ctx context.Context, key string, body []byte, contentType string) error
    Get(ctx context.Context, key string) ([]byte, error)
}
```

- 키는 `sources/{gutenberg_id}/{sha256}.html`. 내용 해시를 키에 넣으면
  같은 원문을 두 번 올려도 같은 자리에 덮어써진다.
- 외부 I/O는 인터페이스 뒤에 둔다 (CONVENTIONS). 잡 테스트에서 갈아끼운다.
- 로컬은 MinIO. `S3_ENDPOINT`가 있으면 path-style을 쓴다 (MinIO 요구사항).
  R2에서는 비워 둔다.

### 4.5 `POST /admin/books`

```
POST /admin/books  { gutenbergId, title, language? }
→ 202 { bookId, status: "pending" }
→ 409 이미 있는 책
```

- **한 트랜잭션 안에서** `books` upsert와 `FetchSource` enqueue를 한다.
  이것이 ADR-003의 근거이므로 트랜잭션 밖으로 빼지 않는다.
- 202다. 수집은 비동기다.

### 4.6 관리자 인증 — 임시

세션 인증은 열린 질문이다(`AGENTS.md`). 그렇다고 무인증으로 열어 둘 수는 없다.

> **이 절은 이미 걷어냈다 (ADR-027).** `ADMIN_TOKEN`과 `X-Admin-Token`은
> 코드·계약·환경변수에 없다. 관리자 엔드포인트는 세션 + `role='admin'`이다.
> 아래는 당시 기록이다.

**`ADMIN_TOKEN` 환경변수와 `X-Admin-Token` 헤더를 비교하는 미들웨어를 둔다.**
없으면 기동 실패. 틀리면 401.

이것은 세션 설계가 아니라 **임시 운영 가드**다. 인증 슬라이스에서 걷어낸다.
사용자 인증(제안자·검수자)은 이 방식으로 하지 않는다.

### 4.7 테스트

| 종류 | 대상 | 필요 |
|---|---|---|
| 단위 | S3 키 생성, 잡 인자 직렬화, 4xx 취소 판정 | 없음 |
| 통합 | 트랜잭션 안 enqueue, 잡 멱등성, R2 왕복 | Postgres + MinIO |

**`DATABASE_URL`이 없으면 `t.Skip`.** CI가 DB 없이 통과해야 한다.

---

## 5. 반드시 지킬 것

1. **Gutenberg 요청은 직렬, 최소 1초 간격, User-Agent 명시.** 큐 동시성 1.
2. **books 행 생성과 잡 등록은 한 트랜잭션.** 이걸 깨면 River를 쓸 이유가 없다.
3. **잡 핸들러는 에러를 반환한다.** 패닉을 제어 흐름으로 쓰지 않는다.
4. **적용된 마이그레이션 파일은 수정하지 않는다.**
5. **`internal/book.Ingester`를 고치지 않는다.** 호출자만 바뀐다.
6. **API 변경은 `openapi.yaml`부터.**
7. **`DATABASE_URL` 없이 `make test`가 통과해야 한다.**
8. 작업 종료 전 `make lint && make test` 통과.

---

## 6. 완료 조건

- [x] `make migrate`가 goose와 River 마이그레이션을 순서대로 적용한다
- [x] `make dev`로 Postgres와 MinIO가 뜬다
- [x] `POST /admin/books`가 202를 주고, **같은 트랜잭션에서 books 행과 잡이 함께 생긴다**
      (커밋 실패 시 둘 다 없다 — 테스트로 확인)
- [x] `X-Admin-Token` 없이 호출하면 401
- [x] `cmd/worker`가 FetchSource → ParseBook을 돌려 **책 한 권이 끝까지 적재된다**
- [x] R2(MinIO)에 원본이 올라가 있고, ParseBook이 그걸 읽어서 파싱한다
- [x] 같은 책을 두 번 요청해도 revision이 중복 생성되지 않는다 (멱등)
- [x] FetchSource 큐의 동시성이 1이다
- [x] `DATABASE_URL` 없이 `make test` 통과
- [x] `make lint && make test` 통과

---

## 7. 다음

`docs/slices/completed/translation.md` — 번역 테이블 4개, 제안·검수, 승계 실행, 사이트맵.
**여기서 SEO 값어치가 나온다.**

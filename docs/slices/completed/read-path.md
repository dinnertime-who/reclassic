# 슬라이스 명세 — 읽기 경로 (적재 → API → 화면)

**작업 종류:** 구현 슬라이스 (수직)
**선행 슬라이스:** `docs/slices/completed/skeleton.md` — 골격이 서 있어야 시작할 수 있다.
테이블·sqlc·`openapi.yaml`·`web/`는 거기서 만들어진다. 이 슬라이스는 그 배선에 실제 데이터를 흘린다.
**선행 문서:** `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/decisions/index.md`
(특히 ADR-004 / 005 / 006 / 007 / 009 / 013 / 014 / 016), `docs/references/parser-report.md`
**다음 슬라이스:** `docs/slices/completed/ingest-automation.md`
**작업 시작 전에 위 문서를 읽을 것.** 특히 ARCHITECTURE의 "핵심 불변식"과 "공개와 SEO" 절.

---

## 1. 이 작업이 답해야 할 질문

골격은 배선만 확인했다. 이번엔 **책 한 권이 파서에서 브라우저까지 실제로 흐르는지** 본다.

> **1. 파서 결과가 손실 없이 스키마에 들어가는가?**
> **2. 책 한 권이 실제로 브라우저에 뜨는가 — 자바스크립트 없이?**
> **3. 파서를 고쳤을 때 `stable_id`로 번역을 승계할 수 있는가 — 몇 퍼센트나?**

3번이 가장 위험하다. ADR-004는 "revision 전환 시 해시 일치 → 자동 승계"라고 정했지만,
**파서를 실제로 고쳤을 때 몇 퍼센트가 일치하는지는 아무도 측정한 적이 없다.**
번역이 쌓인 뒤에 알면 늦다. 이번에 측정 도구를 만든다.

### 이 슬라이스가 아직 주지 않는 것

**첫 읽기 화면은 원문 전용이라 `noindex`다.** ADR-007이 정한 그대로다 —
Gutenberg 원문은 이미 수백 개 사이트에 있어 중복 경쟁에서 이길 수 없다.
색인 대상이 되는 것은 번역 페이지이고, 그건 번역 슬라이스 뒤다.

**즉 이번에 검증되는 것은 SSR 배선과 데이터 흐름이지 SEO 성과가 아니다.**
기대를 그렇게 맞추고 시작할 것.

---

## 2. 범위

### 하는 것

- 캐시된 원문을 파싱해 DB에 적재하는 경로
- ADR-014 크기 게이트와 신뢰도 게이트 → 관리자 확인 큐
- **`stable_id` 승계 매칭률 측정 도구**
- 챕터 조회 API **엔드포인트 하나**
- SSR 읽기 화면 **하나** — 원문 문단이 보인다
- 22권 코퍼스 적재 검증 (golden 대조)

### 하지 않는 것 — 건드리면 안 됨

- **River 잡 큐.** 적재는 CLI로 한다. 잡 큐는 다음 슬라이스
- **R2 업로드, FetchSource.** 원문은 기존 `.cache/gutenberg/`에서 읽는다
- **번역 테이블 4개, 번역 표시, 편집·검수 화면**
- **승계 실행.** 이번엔 **측정만** 한다
- **인증·세션**
- **도서 목록·검색 화면.** URL을 직접 치면 된다
- **화면 디자인.** 문단이 읽히면 된다
- 파서 로직 변경. 남은 이슈 3건(76 목차 제외, 중간 삽입 경고, `alt` 2글자)은 이 슬라이스 밖이다

범위를 넓히지 말 것. 실패했을 때 원인이 하나여야 한다.

### 의존성

**새로 추가하지 않는다.** 골격의 chi·pgx·goose·pnpm과 기존 파서 의존성으로 충분하다.
필요하다고 판단되면 추가 전에 `Proposed` ADR로 남기고 확인할 것.

### 사전 준비

**`make fetch-corpus`를 먼저 돌린다.** `.cache/gutenberg/`는 gitignore 대상이라
새로 클론한 저장소에는 없다. 적재할 원문이 있어야 이 슬라이스를 시작할 수 있다.

**시간이 걸린다.** Gutenberg 수집 규칙상 요청은 직렬이고 요청 간 최소 1초다(22권).
**이 간격을 줄이지 말 것.** 공격적으로 긁으면 IP가 차단되고 복구가 어렵다.
`.env`의 `GUTENBERG_USER_AGENT`가 없으면 실행이 거부된다.

---

## 3. 산출물

| 경로 | 내용 | 비고 |
|---|---|---|
| `internal/book/ingest.go` | 적재 로직 + 게이트 | **유지됨.** 다음에 ParseBook 잡이 호출 |
| `internal/book/succession.go` | `stable_id` 매칭률 계산 | **유지됨** |
| `internal/book/read.go` | 챕터 조회 | **유지됨** |
| `internal/db/queries/*.sql` | 적재·조회 쿼리 | sqlc 재생성 |
| `internal/api/` | 챕터 엔드포인트 핸들러 | |
| `internal/parse/version.go` | `parse.Version` 상수 | |
| `openapi.yaml` | 챕터 엔드포인트 추가 | **여기부터 고친다** |
| `web/` | SSR 읽기 라우트 | |
| `cmd/ingest/main.go` | CLI | **임시.** River 도입 시 `cmd/worker`로 흡수 |
| Makefile · `AGENTS.md` 명령어 표 | `ingest` / `succession` 추가 | 함께 갱신 |

---

## 4. 구현 명세

### 4.1 `parse.Version`

```go
package parse
// Version은 추출 결과가 달라질 수 있는 변경마다 올린다.
// 전략·후처리·정규화·stable_id 규칙이 대상이다. 리포트 서식만 바뀌면 올리지 않는다.
const Version = "2026-08-19"
```

**올리는 것을 잊으면 같은 `parser_version`으로 다른 결과가 저장된다.**
`UNIQUE (book_id, source_id, parser_version)`이 그걸 막아 두 번째 적재가 무시되므로,
파서를 고쳤는데 결과가 안 바뀌면 이 상수부터 의심할 것.

### 4.2 적재

한 책의 적재는 **하나의 트랜잭션**이다 (CONVENTIONS "여러 행을 함께 바꾸는 작업은 트랜잭션").

```
1. books upsert (gutenberg_id 기준)
2. book_sources upsert (content_hash 기준)
3. 파싱 → parse.EvaluateHTML (후처리 포함)
4. 게이트 판정 (§4.3)
5. book_revisions insert
6. chapters / paragraphs 일괄 insert
7. 통과했으면: 기존 활성 revision을 is_active=false로, 새 revision을 true로,
   books.status='ready'
   걸렸으면: is_active=false 유지, books.status='needs_review'
COMMIT
```

- `stable_id`는 `internal/parse`가 만든 값을 **그대로 저장한다.** 적재 코드에서 다시 계산하지 않는다.
  두 곳에서 계산하면 언젠가 갈라진다.
- 이미 같은 `(book_id, source_id, parser_version)` revision이 있으면 **아무것도 하지 않고 성공 반환**한다.
  적재는 멱등이어야 한다.

### 4.3 게이트

| 조건 | 근거 | 처리 |
|---|---|---|
| 챕터 > 200 **또는** 문단 > 15,000 | ADR-014 | `needs_review` |
| `confidence` < 0.85 | ARCHITECTURE 파서 전략 체인 | `needs_review` |
| 파싱 자체 실패 | — | `failed`, revision 없음 |

`needs_review`도 **revision은 저장한다.** 관리자가 보고 판단해야 하므로 버리지 않는다.
다만 `is_active`는 false다. 하드 차단이 아니라 확인 큐라는 ADR-014 결정 그대로다.

게이트 판정 함수는 DB 없이 단위 테스트가 가능해야 한다.

### 4.4 승계 매칭률 측정

```
ingest succession -corpus=<path> -cache=<dir> [-only=ID]
```

저장된 **활성 revision**의 `stable_id` 집합과, **지금 파서로 다시 파싱한 결과**의 집합을 비교한다.
DB에 쓰지 않는다. 읽기 전용이다.

```
권    저장 문단  현재 문단  일치     승계율   신규   소실
1342      2139      2139   2139   100.0%      0      0
```

- `일치` = 양쪽에 다 있는 `stable_id` 수
- `신규` = 현재 파서에만 있는 것 (미번역으로 남을 문단)
- `소실` = 저장본에만 있는 것 (**번역이 붙어 있었다면 갈 곳을 잃는 문단**)

파서를 안 고쳤으면 100%가 나와야 한다. **파서를 고친 뒤 이 명령을 돌려
승계율이 얼마나 떨어지는지 보는 것이 사용법이다.** `make succession`으로 감싼다.

### 4.5 챕터 조회 API — **잠정 계약임을 명시할 것**

ARCHITECTURE의 최종 계약은 번역 프로젝트 기반이다.

```
GET /projects/{id}/chapters/{idx}      ← 최종. translation_projects가 있어야 성립
```

**이번 슬라이스에는 번역 테이블이 없다.** 그래서 도서 기반의 잠정 엔드포인트를 쓴다.

```
GET /books/{gutenbergId}/chapters/{idx}
→ { chapter: { idx, title }, paragraphs: [ { stableId, sourceText } ], totalChapters }
```

- **번역 관련 필드를 넣지 않는다.** `approvedTranslation`을 항상 `null`로 채우면
  계약이 거짓말을 하게 된다. 없는 것은 스키마에 없어야 한다.
- **활성 revision만 조회한다.** 활성 revision이 없으면(예: `needs_review`) **404**.
- 번역 슬라이스에서 프로젝트 기반 엔드포인트로 교체한다.
  그때 이 엔드포인트를 남길지 없앨지 함께 결정한다. **`openapi.yaml`에 잠정임을 주석으로 남길 것.**

**`openapi.yaml`부터 고친다.** 핸들러나 TS 클라이언트를 먼저 고치면 다음 생성에서 덮어써진다.

### 4.6 SSR 읽기 화면

- 라우트 하나: `/books/{gutenbergId}/chapters/{idx}`
- **SSR.** 자바스크립트 실행 없이 받은 HTML에 문단 본문이 들어 있어야 한다
- **`noindex, follow` 메타를 넣는다.** 원문 전용 페이지다 (ADR-007). 빠뜨리면 안 된다
- 이전·다음 챕터 링크 정도만. 디자인하지 않는다
- API 호출은 **orval 생성 클라이언트만** 쓴다. 직접 `fetch` 금지 (CONVENTIONS)
- **server function에 로직을 넣지 않는다.** 데이터 페칭 전용 (ADR-006)

### 4.7 테스트

| 종류 | 대상 | DB 필요 |
|---|---|---|
| 단위 | 게이트 판정, 승계율 계산 | 아니오 |
| 통합 | 적재 트랜잭션, 활성 revision 전환, 멱등성, 챕터 조회 | 예 |

**통합 테스트는 `DATABASE_URL`이 없으면 `t.Skip`한다.**
`parse_test.go`가 캐시 없을 때 건너뛰는 것과 같은 이유다 — **CI가 DB 없이 통과해야 한다.**

---

## 5. 반드시 지킬 것

1. **API 변경은 `openapi.yaml`부터.** 핸들러·TS 클라이언트를 먼저 고치지 않는다.
2. **생성 코드(sqlc / oapi-codegen / orval)는 손으로 고치지 않는다.**
3. **`stable_id`는 `internal/parse`에서만 만든다.** 적재·조회 코드에서 재계산 금지.
4. **읽기 화면에 `noindex`를 빠뜨리지 않는다.** 원문 페이지가 색인되면 ADR-007이 무너진다.
5. **적용된 마이그레이션 파일은 수정하지 않는다.** 필요하면 새 파일을 추가한다.
6. **환경변수에 기본값을 조용히 채우지 않는다.** 없으면 기동 시점에 실패 (CONVENTIONS).
7. **`DATABASE_URL` 없이 `make test`가 통과해야 한다.**
8. **범위 밖(River·R2·번역·인증·목록 화면)을 건드리지 않는다.**
9. **승계율이 낮게 나오는 것도 유효한 결과다.** 수치를 맞추려고 매칭 기준을 느슨하게 하지 말 것.
   낮으면 그게 다음 설계 결정의 근거다.
10. 작업 종료 전 `make lint && make test` 통과.

---

## 6. 완료 조건

- [x] `make ingest`가 22권을 적재하고, DB에서 읽은 챕터·문단 수가 `golden/*.json`과 일치한다
- [x] 같은 명령을 두 번 돌려도 revision이 중복 생성되지 않는다 (멱등)
- [x] 100번(셰익스피어)이 ADR-014 게이트에 걸려 `needs_review` + `is_active=false`이고,
      그 책의 챕터 조회가 **404**를 준다
- [x] 책당 활성 revision이 최대 하나임을 DB 제약이 강제한다 (중복 삽입 시도가 실패한다)
- [x] `make succession`이 22권 승계율을 출력하고, 파서 미변경 상태에서 **100%**가 나온다
- [x] **`curl`로 받은 읽기 화면 HTML에 문단 본문이 들어 있다** (자바스크립트 없이 = SSR)
- [x] **그 HTML에 `noindex` 메타가 있다** (ADR-007)
- [x] `DATABASE_URL` 없이 `make test`가 통과한다
- [x] `AGENTS.md` 명령어 표에 새 명령이 추가됐다
- [x] `make lint && make test` 통과

---

## 7. 다음 슬라이스로 열어 둔 것

| 항목 | 조건 |
|---|---|
| 수집 자동화 (River + R2 + FetchSource) → `docs/slices/completed/ingest-automation.md` | `internal/book/ingest.go`를 잡 핸들러가 호출 |
| 번역 (테이블 4개 + 제안·검수 화면 + 승계 실행) → `docs/slices/completed/translation.md` | §4.4 승계율 숫자를 보고 설계. 여기서 SEO 값어치가 나온다 |
| 인증 → `docs/slices/completed/auth.md` | |
| 도서 목록·검색 화면 | 아직 명세 없음 |
| 파서 잔여 3건 | 76 목차 제외, 중간 삽입 경고, `alt` 2글자 복원 |

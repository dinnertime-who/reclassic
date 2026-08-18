# reclassic 아키텍처

## 개요

Project Gutenberg의 퍼블릭 도메인 도서를 수집해 장·문단 단위로 분해하고,
사용자가 문단별 번역을 제안하면 검수를 거쳐 확정본을 공개한다.

- **관리자**: Gutenberg 카탈로그에서 책을 골라 다운로드를 지시한다.
- **워커**: 원문 HTML을 받아 R2에 원본을 보관하고, 장·문단으로 분해해 Postgres에 적재한다.
- **사용자**: 장·문단이 구분된 원문을 읽고, 문단 단위로 번역을 제안한다.
- **검수자**: 제안 중 하나를 승인한다. 승인된 것만 공개된다.

## 서비스 구성

배포 단위는 넷이며, Go 코드는 **하나의 모듈**을 공유한다.

```
api        Go   JSON API (사용자 / 관리자 / 검수)
worker     Go   Gutenberg 수집, 파싱, 사이트맵 생성
web        TS   TanStack Start — 읽기 SSR + 편집·검수 CSR
postgres   —    Railway 관리형
R2         —    Cloudflare. 원본 HTML 스냅샷 (외부)
```

MSA로 더 쪼개지 않는다. 워커 부하가 커지면 **같은 바이너리를 다른 큐만 소비하도록** 추가 기동한다.
코드 분리 없이 스케일 단위만 늘리는 것이 이 프로젝트의 확장 경로다.

### 디렉토리

```
cmd/api/            HTTP 서버 진입점
cmd/worker/         잡 컨슈머 진입점
cmd/parsecheck/     파서 검증 스파이크 실행기
internal/
  gutenberg/        카탈로그 조회, 원문 fetch (레이트리밋 포함)
  parse/            DOM → 장·문단 추출. 전략 체인
  book/             도서 도메인
  translate/        제안·검수 도메인
  storage/          R2 어댑터
  db/
    migrations/     순번 SQL. 기존 파일 수정 금지
    queries/        sqlc 입력
web/                TanStack Start 앱
openapi.yaml        API 계약 단일 원본
```

## 수집 파이프라인

잡을 잘게 나눈다. 하나의 거대한 잡보다 재시도가 쉽다.

```
관리자가 다운로드 지시
  └─ (트랜잭션) books 행 생성 + FetchSource 잡 enqueue
       └─ FetchSource : 원문 HTML 요청 → R2 저장 → book_sources 기록
            └─ ParseBook : R2에서 읽어 추출 → book_revisions + chapters + paragraphs
```

River를 쓰는 이유는 **Postgres 트랜잭션 안에서 enqueue가 가능**하기 때문이다.
"책은 생성됐는데 잡 등록이 실패"하는 틈이 생기지 않는다.

### Gutenberg 수집 규칙

- 카탈로그는 스크래핑하지 않는다. Gutendex API 또는 공식 카탈로그 덤프를 쓴다.
- 요청은 직렬. 동시성 1~2, 요청 간 최소 1초. 식별 가능한 User-Agent 필수.
- 본문 경계는 `*** START OF THE PROJECT GUTENBERG EBOOK ... ***` /
  `*** END OF ... ***` 텍스트 마커를 1순위로 쓴다. DOM 휴리스틱은 그 다음이다.

### 파서 전략 체인

전사 시기에 따라 HTML 구조 편차가 크다. 단일 로직이 아니라 전략 체인으로 간다.

```go
type Extractor interface {
    Name() string
    Extract(*goquery.Document) (*Result, Confidence)
}
```

신뢰도가 임계값 미만이면 자동 확정하지 않고 **관리자 수동 검토 큐**로 보낸다.
전부 자동으로 처리하려 들지 않는다.

## 데이터 모델

### 원본 — Gutenberg에서 온 불변 데이터

```
books            id, gutenberg_id UNIQUE, title, author, language, status
book_sources     id, book_id, s3_key, content_hash, fetched_at
book_revisions   id, book_id, source_id, parser_version, confidence, is_active
chapters         id, revision_id, idx, title
paragraphs       id, chapter_id, idx, stable_id, text, html
```

### 번역 — 사용자가 만드는 가변 데이터

```
translation_projects
  id, book_id, target_lang, status, published_at
  UNIQUE (book_id, target_lang)

translation_proposals                    -- 문단당 N개
  id, project_id, paragraph_stable_id, text, author_id,
  status,                                -- pending|approved|rejected|superseded|withdrawn
  reviewed_by, reviewed_at, review_note, created_at
  INDEX (project_id, paragraph_stable_id, status)

paragraph_translations                   -- 문단당 0 또는 1. 공개되는 것
  project_id, paragraph_stable_id,       -- PRIMARY KEY
  text, proposal_id, approved_by, approved_at

book_glossary                            -- 인명·지명·호칭 일관성
  book_id, source_term, target_term, note
```

## 핵심 불변식

**이 절의 규칙을 깨는 변경은 하지 말 것.**

### 1. 번역은 `paragraph_stable_id`에 붙는다. `paragraphs.id`가 아니다.

파서를 개선해 재파싱하면 문단 행 ID가 밀린다. 번역이 `paragraphs.id`를 참조하면
그 순간 모든 번역이 엉뚱한 문단에 붙는다. 이 프로젝트에서 유일하게 되돌리기 어려운 실수다.

- `stable_id = sha256(정규화된 본문)[:16]` — 공백·따옴표·대시를 정규화한 뒤 해시
- 파싱 결과는 불변 `book_revisions`로 쌓는다. 기존 revision을 수정하지 않는다.
- revision 전환 시: 해시 일치 → 번역 자동 승계 / 유사 → 관리자 확인 큐 / 신규 → 미번역

FK로 잡지 않는 것이 부자연스러워 보이지만, 이것이 재파싱을 안전하게 만든다.

### 2. 문단당 공개 번역은 정확히 하나다.

`paragraph_translations`의 복합 PK `(project_id, paragraph_stable_id)`가 이를 보장한다.
애플리케이션 로직이 아니라 스키마가 지키게 한다.

### 3. 승인은 한 트랜잭션 안에서 세 가지를 함께 한다.

```sql
BEGIN;
  -- 1. 기존 확정본의 제안을 superseded로
  UPDATE translation_proposals SET status = 'superseded'
   WHERE id = (SELECT proposal_id FROM paragraph_translations
                WHERE project_id = $1 AND paragraph_stable_id = $2);

  -- 2. 새 제안 승인. status 조건이 동시 승인을 막는다
  UPDATE translation_proposals
     SET status = 'approved', reviewed_by = $3, reviewed_at = now()
   WHERE id = $4 AND status = 'pending';    -- 0 rows면 롤백 후 409

  -- 3. 확정본 교체
  INSERT INTO paragraph_translations (...)
  VALUES (...)
  ON CONFLICT (project_id, paragraph_stable_id) DO UPDATE SET ...;
COMMIT;
```

2번의 `AND status = 'pending'`이 두 검수자가 서로 다른 제안을 동시에 승인하는 경우를 막는다.
영향 행이 0이면 롤백하고 409를 반환한다.

### 4. Railway 프라이빗 네트워크는 IPv6 전용이다.

Go 서버는 `[::]`에 바인딩한다. `0.0.0.0`은 서비스 간 내부 호출을 받지 못한다.

```go
srv := &http.Server{Addr: "[::]:" + os.Getenv("PORT")}
```

SSR 서버가 API를 호출할 때는 `서비스명.railway.internal`을 쓴다. 브라우저는 공개 도메인을 쓴다.
**API 베이스 URL은 서버·클라이언트에서 서로 달라야 한다.**

## 공개와 SEO

SSR을 쓰는 이유는 검색 노출이다. 그래서 색인 정책이 프레임워크 선택보다 중요하다.

- **원문 전용 페이지는 `noindex, follow`.** Gutenberg 원문은 이미 수백 개 사이트에 있어
  중복 경쟁에서 이길 수 없고, 크롤 예산만 소모한다.
- **번역 페이지가 색인 대상이자 canonical.** 이 사이트의 유일한 고유 콘텐츠다.
- **커버리지 임계값을 색인 조건으로 건다.** 승인률이 낮은 챕터는 대부분이 원문이라 thin content다.
  `approved / total`이 임계값 미만이면 `noindex`.
- 사이트맵은 같은 기준으로 워커가 생성해 R2에 올린다. 파일당 URL 50,000 / 50MB 제한을 지켜 분할한다.
- 읽기 페이지는 승인 시점에만 바뀐다. 승인 트랜잭션에서 해당 챕터 캐시를 무효화하고
  나머지는 CDN이 받아낸다. SSR 서버가 실제로 하는 일은 적다.

공개는 부분 공개를 허용한다. 승인률 100%를 기다리면 아무 책도 공개하지 못한다.
미확정 문단은 읽기 화면에서 원문으로 노출하고 진행률을 표시한다.

## API 계약

`openapi.yaml`이 단일 원본이다.

```
openapi.yaml ─┬─→ oapi-codegen → Go 서버 인터페이스 + 타입
              └─→ orval        → TS 클라이언트 + react-query 훅
```

읽기 화면은 챕터 단위로 자르고, 한 요청에 필요한 것을 다 조인해 내려준다.
책 하나에 문단이 5,000개를 넘길 수 있어 전체를 한 번에 보내면 안 되고,
문단마다 따로 요청하면 N+1이 그대로 네트워크로 나간다.

```
GET /projects/{id}/chapters/{idx}
→ { chapter, paragraphs: [{ stable_id, source_text, approved_translation,
                            proposal_count, my_proposal_status }] }
```

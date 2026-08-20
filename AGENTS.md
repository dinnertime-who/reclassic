# reclassic — 에이전트 작업 지침

> **모든 AI 에이전트는 이 파일 하나를 읽습니다.**
> 도구별 지침 파일(`CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md` 등)을
> 따로 만들지 마세요. 갈라지는 순간 이 하네스의 의미가 없어집니다.
> 지침 변경은 언제나 이 파일에서만 합니다.

## 이 프로젝트가 하는 일

Project Gutenberg의 퍼블릭 도메인 도서를 내려받아 장·문단 단위로 분해하고,
사용자가 문단별 번역을 제안하면 검수를 거쳐 **문단당 확정본 1개**를 공개하는 서비스.

| 문서 | 내용 |
|---|---|
| `docs/ARCHITECTURE.md` | 시스템 구조, 데이터 모델, 핵심 불변식 |
| `docs/DECISIONS.md` | 설계 결정 이력 (ADR) |
| `docs/CONVENTIONS.md` | 코딩 규약 |
| `docs/SPIKE_PARSER.md` | 파서 검증 스파이크 명세 (완료) |
| `docs/PARSER_REPORT.md` | 스파이크 결과와 권고 — 파서 판단의 근거 |
| `docs/SLICE_SKELETON.md` | 프로젝트 골격 명세 (완료) |
| `docs/SLICE_READ_PATH.md` | 읽기 경로 명세 (완료) |
| `docs/SLICE_INGEST_AUTOMATION.md` | 수집 자동화 명세 (완료) |
| `docs/SLICE_TRANSLATION.md` | 번역(제안·검수) 명세 (완료) |
| `docs/SLICE_AUTH.md` | 세션 인증 명세 (완료) |
| `docs/SLICE_DEPLOY.md` | **현재 작업 — 배포(Railway) 명세** |

**코드를 쓰기 전에 `docs/ARCHITECTURE.md`를 먼저 읽으세요.** 이 파일에는 요약만 있습니다.

**슬라이스 1(골격)은 끝났습니다.** 도구 여섯(goose·sqlc·oapi-codegen·orval·docker compose·
TanStack Start)이 한 저장소에서 맞물려 돌고, `GET /healthz` 하나가 Go API → SSR 화면까지
흐르는 것을 확인했습니다. ADR-009("계약 변경이 양쪽에서 컴파일 에러로 드러난다")와
ADR-006(SSR)의 전제도 실물로 검증했습니다.

**슬라이스 2(읽기 경로)도 끝났습니다.** 22권이 DB에 적재되고 golden과 수가 일치하며,
책 한 권이 브라우저에 자바스크립트 없이 뜹니다. 셰익스피어 전집은 ADR-014 게이트에 걸려
`needs_review`이고 읽기 조회는 404입니다.

**`stable_id` 승계율은 파서 미변경 상태에서 21권 37,125문단 100%입니다.**
ADR-004가 전제한 "해시 일치 → 자동 승계"가 처음으로 수치로 확인됐습니다.
파서를 고친 뒤 `make succession`을 돌려 이 수치가 얼마나 떨어지는지 보는 것이 사용법입니다.

**슬라이스 3(수집 자동화)도 끝났습니다.** `POST /admin/books` 한 번으로
FetchSource → R2 → ParseBook → 적재가 자동으로 돕니다.
**ADR-003의 근거("트랜잭션 안 enqueue")는 테스트로 못 박혀 있습니다** —
`internal/book/enqueue_integration_test.go`가 잡 등록이 실패하면 `books` 행도
남지 않는 것을 확인합니다. 이걸 깨면 River를 쓸 이유가 사라집니다.

**슬라이스 4(번역)도 끝났습니다.** 제안 → 검수 → 확정본이 흐르고, 커버리지 80%를 넘으면
읽기 화면이 `index`로 바뀝니다(ADR-023). 사이트맵은 색인 대상만 담습니다.

**여기서 ADR-005의 구멍이 하나 드러나 ADR-024로 막았습니다.** `WHERE status='pending'`은
같은 제안의 동시 승인만 막습니다. 같은 문단의 **서로 다른** 제안을 동시에 승인하면
`approved`가 둘 남습니다. 승인 트랜잭션 맨 앞의 자문 잠금을 빼지 마세요 —
단발 테스트로는 통과합니다.

**슬라이스 5(세션 인증)도 끝났습니다.** Google 로그인 + Postgres 세션이고,
임시 헤더(`X-User-Handle`·`X-Admin-Token`)는 코드·계약·환경변수에서 전부 사라졌습니다.
관리자는 `ADMIN_EMAIL`과 일치하는 Google 계정입니다 (ADR-027).

**현재 작업은 배포(Railway)입니다** — `docs/SLICE_DEPLOY.md`.
ADR-002가 정한 4서비스 구성이 실물로 검증된 적이 없습니다 —
`[::]` 바인딩, `railway.internal`, 서브도메인 간 쿠키 공유가 전부 미확인입니다.

빌드는 Nixpacks가 아니라 Dockerfile입니다 (ADR-029).
**착수 블로커 둘이 남아 있습니다: 도메인, 마이그레이션 실행 위치.**

계획된 순서:

| | 슬라이스 | 명세 |
|---|---|---|
| 1 | 골격 (walking skeleton) | `docs/SLICE_SKELETON.md` — **완료** |
| 2 | 읽기 경로 — 적재 → API → SSR 화면 | `docs/SLICE_READ_PATH.md` — **완료** |
| 3 | 수집 자동화 (River + R2) | `docs/SLICE_INGEST_AUTOMATION.md` — **완료** |
| 4 | 번역 (제안·검수) — **여기서 SEO 값어치가 나온다** | `docs/SLICE_TRANSLATION.md` — **완료** |
| 5 | 세션 인증 — 임시 헤더를 걷어낸다 | `docs/SLICE_AUTH.md` — **완료** |
| 6 | 배포 (Railway) | `docs/SLICE_DEPLOY.md` ← **지금** |
| 7 | 편집·검수 화면, 도서 목록, 관리자 확인 큐 | 미작성 |

슬라이스 2에서 책이 브라우저에 뜨지만 **원문 전용 페이지라 `noindex`입니다**(ADR-007).
색인 대상이 되는 것은 번역 페이지이고 그건 슬라이스 4입니다.

파서 쪽(ADR-013·016)은 구현·검증이 끝났습니다 (`internal/parse/postprocess.go`).
ADR-014(합본 크기 판정 → 관리자 큐)는 `internal/book/gate.go`에 구현돼 있습니다.
ADR-015에 따라 **희곡은 초기 지원 범위에서 제외**합니다. 코드 게이트가 아니라
도서 선정 단계의 운영 정책이므로, 파서나 적재 코드에 장르 판별을 넣지 마세요.

ADR-011이 걸어둔 게이트("스파이크가 끝나기 전에는 DB·API·웹 금지")는 **해제됐습니다.**
스파이크가 답을 냈고, 스키마 보정 레이어는 넣지 않기로 정해졌습니다(PARSER_REPORT 권고 2).
다만 **DB 스키마를 쓸 때 `stable_id` 생성 규칙은 ADR-016을 반드시 따르세요.**
같은 본문이 반복되면 2번째부터 등장 순서를 붙입니다. 나중에 바꾸면 쌓인 번역이 전부 어긋납니다.

## 기술 스택

| 영역 | 선택 | 비고 |
|---|---|---|
| 백엔드 | Go + chi | 배포 단위 2개: `cmd/api`, `cmd/worker`. 라우터는 chi (ADR-018) |
| DB | PostgreSQL | 드라이버 pgx/v5, 마이그레이션 goose (ADR-017). `internal/db/migrations/` |
| 잡 큐 | River | Postgres 기반. 트랜잭션 안에서 enqueue |
| 쿼리 | sqlc | SQL → Go 타입 생성 |
| HTML 파싱 | goquery | Gutenberg 원문 추출 |
| 오브젝트 스토리지 | Cloudflare R2 | S3 호환. 원본 HTML 스냅샷 보관 |
| 웹 | TanStack Start | 읽기 화면 SSR, 편집·검수 화면 CSR |
| 프론트 패키지 | pnpm | Node 패키지는 `web/` 하나. 루트 `package.json` 없음 (ADR-019) |
| API 계약 | OpenAPI → oapi-codegen(Go) + Orval(TS) | 스펙 우선 |
| 빌드 타임 도구 | sqlc·oapi-codegen은 `go.mod` tool 디렉티브 | 별도 설치 없음 (ADR-020) |
| 배포 | Railway | 서비스: api / worker / postgres / web. Cloudflare 이전은 검토 후 기각 (ADR-028) |

## 명령어 — 이것만 사용하세요

| 목적 | 명령 |
|---|---|
| 빌드 | `make build` |
| 테스트 | `make test` |
| 린트 | `make lint` |
| 포맷 | `make fmt` |
| 코드 생성 (sqlc / oapi-codegen / orval) | `make generate` |
| 마이그레이션 적용 | `make migrate` |
| 로컬 의존 서비스 기동 (Postgres + MinIO) | `make dev` |
| 로컬 의존 서비스 정지 | `make dev-down` |
| API 서버 실행 (:8080) | `make run-api` |
| 웹 개발 서버 실행 (:3100, SSR) | `make run-web` |
| 웹 개발 서버를 LAN에 노출 | `make run-web-lan` (주소 지정: `LAN_IP=192.168.x.x`) |
| `web/` 의존성 설치 | `make web-install` |
| 파서 결과 적재 (멱등) | `make ingest` (한 권만: `make ingest ONLY=1342`) |
| `stable_id` 승계율 측정 | `make succession` |
| 워커 실행 (잡 소비) | `make run-worker` |
| 검증용 도서 내려받기 | `make fetch-corpus` |
| 파서 검증 리포트 | `make parsecheck` |
| golden 스냅샷 비교 | `make golden` |
| golden 스냅샷 갱신 | `make golden GOLDEN_UPDATE=1` |

`go test ./...`나 `npm run ...`을 직접 추론해서 실행하지 마세요.
**Makefile이 유일한 진입점입니다.** 필요한 명령이 없으면 Makefile에 추가하고 이 표도 같이 갱신하세요.

## 작업 규칙

1. **작업 전에 `docs/ARCHITECTURE.md`를 읽습니다.** 특히 "핵심 불변식" 절.
2. **아키텍처에 영향을 주는 결정은 `docs/DECISIONS.md`에 ADR로 남깁니다.**
   기존 결정을 뒤집을 때는 해당 ADR을 `Superseded`로 표시하고 새 항목을 추가하세요. 지우지 마세요.
3. **DB 스키마는 마이그레이션 파일로만 바꿉니다.** 기존 마이그레이션 파일은 절대 수정하지 않고, 항상 새 파일을 추가합니다.
4. **생성 코드는 손으로 고치지 않습니다.** `sqlc`, `oapi-codegen`, `orval` 산출물이 잘못됐으면 원본(SQL / `openapi.yaml`)을 고치고 `make generate`를 다시 돌립니다.
5. **API 변경은 `openapi.yaml`부터 고칩니다.** Go 핸들러나 TS 클라이언트를 먼저 고치면 다음 `make generate`에서 덮어써집니다.
6. **작업을 끝내기 전에 `make lint && make test`가 통과해야 합니다.**
7. 커밋 메시지는 Conventional Commits: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, `test:`

## 하지 말 것

- **Gutenberg 원본 HTML을 커밋하지 마세요.** `/.cache/`는 gitignore 대상입니다.
  검증용 도서는 `internal/parse/testdata/corpus.json`에 ID만 기록하고 `make fetch-corpus`로 받습니다.
- **Gutenberg에 병렬 요청을 보내지 마세요.** 워커 동시성은 1~2, 요청 간 최소 1초 간격, User-Agent 명시.
  공격적으로 긁으면 IP가 차단되고 복구가 어렵습니다.
- **수집 큐(`fetch`)의 동시성을 1보다 올리지 마세요.** Gutenberg에 병렬 요청이 나갑니다.
  파싱 큐(`parse`)는 올려도 됩니다. 큐를 나눈 이유가 그것입니다.
- **`PARSE_CONCURRENCY`를 메모리 확인 없이 올리지 마세요.** 셰익스피어 전집 파싱이 힙 310MB를 씁니다 (ADR-029).
- **`0.0.0.0`에 바인딩하지 마세요.** Railway 프라이빗 네트워크는 IPv6 전용이라 `[::]`를 써야 서비스 간 호출이 됩니다.
- **비즈니스 로직을 TanStack Start의 server function에 넣지 마세요.**
  권한 검사·DB 접근·도메인 로직은 전부 Go. server function은 SSR 데이터 페칭과 쿠키 전달 전용입니다.
- **`paragraph_translations`에 문단당 2행 이상 넣지 마세요.** 복합 PK가 막고 있고, 이건 의도된 제약입니다.
- **승인 트랜잭션의 자문 잠금(ADR-024)과 `AND status='pending'`을 빼지 마세요.**
  둘 다 있어야 동시 승인이 막힙니다. 하나만으로는 부족합니다.
- **세션 토큰을 평문으로 저장하지 마세요.** DB에는 sha256만 들어갑니다 (ADR-027).
- **OAuth `state` 검증을 빼지 마세요.** 로그인 CSRF가 열립니다.
- **프로덕션에서 `COOKIE_SECURE=true`를 잊지 마세요.** 로컬에서만 false입니다.
- **CORS 허용 출처에 `*`를 넣지 마세요.** credentials와 함께 쓸 수 없고, 써서도 안 됩니다 (ADR-026).
  LAN·Tailscale로 접속하려면 그 출처를 `CORS_ALLOWED_ORIGINS`에 추가하세요.
- 시크릿을 커밋하지 마세요. 새 환경변수는 `.env.example`에 이름과 설명만 추가합니다.

## 열려 있는 질문

작업 중 아래에 해당하는 판단이 필요하면, 임의로 정하지 말고 `docs/DECISIONS.md`에
`Proposed` 상태의 ADR로 남기고 사람에게 확인을 요청하세요.

- 번역 프로젝트 전체의 공개 기준 (챕터 색인 기준과 별개)

인증 방식은 더 이상 열린 질문이 아닙니다 — **Google 위임 + Postgres 세션 + 지정 계정 관리자**(ADR-027).

챕터 색인 임계값은 더 이상 열린 질문이 아닙니다 — **승인 커버리지 80%**(ADR-023).
프로젝트 전체의 공개 기준은 이와 별개이며 아직 열려 있습니다.

관리자 보정 UI의 범위는 더 이상 열린 질문이 아닙니다 — **가벼운 챕터 보정**(장 병합, 제목 수정)이면
충분하고, 스키마 보정 레이어는 넣지 않습니다. 근거는 `docs/PARSER_REPORT.md` 권고 1·2와 ADR-013.

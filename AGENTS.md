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

**코드를 쓰기 전에 `docs/ARCHITECTURE.md`를 먼저 읽으세요.** 이 파일에는 요약만 있습니다.

## 기술 스택

| 영역 | 선택 | 비고 |
|---|---|---|
| 백엔드 | Go | 배포 단위 2개: `cmd/api`, `cmd/worker` |
| DB | PostgreSQL | 마이그레이션은 `internal/db/migrations/` |
| 잡 큐 | River | Postgres 기반. 트랜잭션 안에서 enqueue |
| 쿼리 | sqlc | SQL → Go 타입 생성 |
| HTML 파싱 | goquery | Gutenberg 원문 추출 |
| 오브젝트 스토리지 | Cloudflare R2 | S3 호환. 원본 HTML 스냅샷 보관 |
| 웹 | TanStack Start | 읽기 화면 SSR, 편집·검수 화면 CSR |
| API 계약 | OpenAPI → oapi-codegen(Go) + Orval(TS) | 스펙 우선 |
| 배포 | Railway | 서비스: api / worker / postgres / web |

## 명령어 — 이것만 사용하세요

| 목적 | 명령 |
|---|---|
| 빌드 | `make build` |
| 테스트 | `make test` |
| 린트 | `make lint` |
| 포맷 | `make fmt` |
| 코드 생성 (sqlc / oapi-codegen / orval) | `make generate` |
| 마이그레이션 적용 | `make migrate` |
| 로컬 실행 | `make dev` |
| 검증용 도서 내려받기 | `make fetch-corpus` |
| 파서 검증 리포트 | `make parsecheck` |

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
- **`0.0.0.0`에 바인딩하지 마세요.** Railway 프라이빗 네트워크는 IPv6 전용이라 `[::]`를 써야 서비스 간 호출이 됩니다.
- **비즈니스 로직을 TanStack Start의 server function에 넣지 마세요.**
  권한 검사·DB 접근·도메인 로직은 전부 Go. server function은 SSR 데이터 페칭과 쿠키 전달 전용입니다.
- **`paragraph_translations`에 문단당 2행 이상 넣지 마세요.** 복합 PK가 막고 있고, 이건 의도된 제약입니다.
- 시크릿을 커밋하지 마세요. 새 환경변수는 `.env.example`에 이름과 설명만 추가합니다.

## 열려 있는 질문

작업 중 아래에 해당하는 판단이 필요하면, 임의로 정하지 말고 `docs/DECISIONS.md`에
`Proposed` 상태의 ADR로 남기고 사람에게 확인을 요청하세요.

- 파서 자동 분리 실패 시 관리자 보정 UI의 범위
- 번역 프로젝트의 공개 기준 (승인 커버리지 임계값)
- 인증 방식 세부 (세션 저장소, 만료 정책)

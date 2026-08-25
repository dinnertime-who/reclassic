# reclassic — 에이전트 작업 지침

> **모든 AI 에이전트는 이 파일을 먼저 읽습니다.**
> 도구별 지침 파일(`CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md` 등)을
> 따로 만들지 마세요. 갈라지는 순간 이 하네스의 의미가 없어집니다.
>
> **이 파일은 지도이지 백과사전이 아닙니다.** 세부는 `docs/`로 내려갑니다.
> 여기에 내용을 쌓지 마세요 — 길어지면 아무도(사람도 에이전트도) 끝까지 읽지 않습니다.

## 이 프로젝트

Project Gutenberg의 퍼블릭 도메인 도서를 내려받아 장·문단 단위로 분해하고,
사용자가 문단별 번역을 제안하면 검수를 거쳐 **문단당 확정본 1개**를 공개하는 서비스.

프로덕션이 떠 있습니다 — 웹 `https://reclassic.dinnertimes.app` ·
API `https://api-reclassic.dinnertimes.app` (둘 다 Cloudflare 프록시 뒤, ADR-034).

## 어디를 읽는가

**전체 지도는 [`docs/index.md`](docs/index.md)입니다.** 자주 쓰는 것만 여기 둡니다.

| 하려는 일 | 읽을 것 |
|---|---|
| **코드를 쓰기 전** | [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — 특히 "핵심 불변식" |
| 스타일·브랜치·PR·보안 규약 | [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) |
| "왜 이렇게 돼 있지?" | [`docs/decisions/index.md`](docs/decisions/index.md) — 해당 ADR만 엽니다 |
| 지금 무슨 작업 중인지 | [`docs/slices/index.md`](docs/slices/index.md) |
| 알려진 구멍·미검증·부채 | [`docs/tech-debt.md`](docs/tech-debt.md) |
| 배포·Railway 조작 | [`docs/slices/completed/deploy.md`](docs/slices/completed/deploy.md) — §5 CLI, §6 사람이 직접 |
| 파서를 건드릴 때 | [`docs/references/parser-report.md`](docs/references/parser-report.md) |

## 지금 하는 것

**슬라이스 8 — 도서 목록 · 관리자 확인 큐 · 역할 부여.**
[`docs/slices/index.md`](docs/slices/index.md)를 **먼저 읽으세요.** 명세는 아직 없습니다.
셋 다 **화면보다 없는 쿼리가 먼저입니다** ([`docs/tech-debt.md`](docs/tech-debt.md) D1·D2·D3·D5).

슬라이스 7에서 **web 스택이 실물로 섰습니다** — react-query·Tailwind v4·shadcn·Vitest·ESLint (ADR-035).
**경로 별칭은 `#/*`입니다.** shadcn 기본값 `@/*`가 아닙니다 — `web/components.json`에서 맞춰져
있으니 그대로 두세요 (ADR-019).

## 기술 스택

| 영역 | 선택 | 비고 |
|---|---|---|
| 백엔드 | Go + chi | 배포 단위 2개: `cmd/api`, `cmd/worker` (ADR-018) |
| DB | PostgreSQL | pgx/v5 + goose (ADR-017). `internal/db/migrations/` |
| 잡 큐 | River | Postgres 기반. 트랜잭션 안에서 enqueue (ADR-003) |
| 쿼리 | sqlc | SQL → Go 타입 생성 |
| HTML 파싱 | goquery | Gutenberg 원문 추출 |
| 오브젝트 스토리지 | Cloudflare R2 | S3 호환. 원본 HTML 스냅샷 (ADR-008) |
| 웹 | TanStack Start | 읽기 SSR, 편집·검수 CSR (ADR-006) |
| 웹 서버 상태 | TanStack Query | orval이 훅 생성. **`QueryClient`는 요청마다 새로** (ADR-035) |
| 웹 스타일 | Tailwind CSS v4 | CSS-first. `tailwind.config.js` 없음 (ADR-035) |
| 웹 컴포넌트 | shadcn/ui | **먼저 찾고, 없을 때만 직접 구현** (ADR-035) |
| 웹 폼 | 없음 | 제어 컴포넌트로 직접 (ADR-035) |
| 웹 테스트·린트 | Vitest + ESLint | `DATABASE_URL` 없이 돕니다 (ADR-035) |
| 프론트 패키지 | pnpm | Node 패키지는 `web/` 하나 (ADR-019) |
| API 계약 | OpenAPI → oapi-codegen + Orval | 스펙 우선 (ADR-009) |
| 배포 | Railway | api / worker / postgres / web (ADR-002·031) |

## 명령어 — 이것만 사용하세요

| 목적 | 명령 |
|---|---|
| 빌드 · 테스트 · 린트 · 포맷 (뒤 셋은 Go + web 한 번에) | `make build` · `make test` · `make lint` · `make fmt` |
| 문서 구조 검사 | `make docs-check` |
| 코드 생성 (sqlc / oapi-codegen / orval) | `make generate` |
| 마이그레이션 적용 | `make migrate` |
| 로컬 의존 서비스 (Postgres + MinIO) | `make dev` · `make dev-down` |
| API · 워커 · 웹 실행 | `make run-api` · `make run-worker` · `make run-web` (:3100) |
| 웹 개발 서버를 LAN에 노출 | `make run-web-lan` (`LAN_IP=192.168.x.x`) |
| shadcn 컴포넌트 추가 | `make ui-add C=button` |
| `web/` 의존성 설치 | `make web-install` |
| 파서 결과 적재 (멱등) | `make ingest` (한 권만: `ONLY=1342`) |
| `stable_id` 승계율 측정 | `make succession` |
| 검증용 도서 내려받기 · 파서 리포트 | `make fetch-corpus` · `make parsecheck` |
| golden 스냅샷 비교 · 갱신 | `make golden` · `make golden GOLDEN_UPDATE=1` |
| 이미지 빌드 | `make docker-build` · `make docker-build-web` |
| PR 생성 | `make pr` |

`go test ./...`나 `pnpm ...`을 직접 추론해 실행하지 마세요.
**Makefile이 유일한 진입점입니다.** 명령을 추가하면 이 표도 같이 갱신하세요.

## 작업 규칙

1. 작업 전에 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) "핵심 불변식"을 읽습니다.
2. **아키텍처에 영향을 주는 결정은 ADR로 남깁니다** — `docs/decisions/ADR-NNN.md` 파일을 만들고
   [색인](docs/decisions/index.md)에 줄을 추가합니다. 뒤집을 때는 기존 항목을 `Superseded`로 표시하고 새로 만듭니다. **지우지 마세요.**
3. **DB 스키마는 마이그레이션 파일로만 바꿉니다.** 기존 파일은 수정하지 않고 새 파일을 추가합니다.
4. **생성 코드는 손으로 고치지 않습니다.** 원본(SQL / `openapi.yaml`)을 고치고 `make generate`를 다시 돌립니다.
   `shadcn`이 복사한 컴포넌트는 예외입니다 — 그건 우리 소스입니다 (ADR-035).
5. **API 변경은 `openapi.yaml`부터** 고칩니다. 핸들러를 먼저 고치면 다음 `make generate`에서 덮어써집니다.
6. **끝내기 전에 `make lint && make test && make docs-check`가 통과해야 합니다.**
7. **`main`은 보호 브랜치입니다.** PR로 넣습니다. 머지는 squash라 **PR 제목이 곧 커밋 제목**입니다.
   브랜치·커밋·PR 규칙은 [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) "기여 흐름"에 있습니다.

## 절대 깨지 말 것

**되돌릴 수 없거나, 조용히 깨져서 테스트·배포가 성공으로 보이는 것들만 여기 둡니다.**
나머지 규칙은 [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md)와
[`deploy.md` §7](docs/slices/completed/deploy.md)에 있습니다.

- **`stable_id` 생성 규칙(ADR-016)을 바꾸지 마세요.** 같은 본문이 반복되면 2번째부터 등장 순서를 붙입니다.
  **위치를 해시에 넣지 마세요.** 나중에 바꾸면 쌓인 번역이 전부 어긋납니다 — 이 프로젝트에서 가장 되돌리기 어렵습니다.
- **승인 트랜잭션의 자문 잠금(ADR-024)과 `AND status='pending'`을 빼지 마세요.**
  둘 다 있어야 동시 승인이 막힙니다. **단발 테스트로는 통과합니다.**
- **`paragraph_translations`에 문단당 2행 이상 넣지 마세요.** 복합 PK가 막고 있는 의도된 제약입니다.
- **`0.0.0.0`에 바인딩하지 마세요.** Railway 프라이빗 네트워크는 IPv6 전용이라 `[::]`를 써야 서비스 간 호출이 됩니다.
- **`QueryClient`를 모듈 스코프에 두지 마세요** (ADR-035). 공유하면 **한 사람의 응답이 다른 사람에게 나갑니다.**
- **읽기 화면에 react-query 훅이나 shadcn 컴포넌트를 넣지 마세요** (ADR-035).
  자바스크립트 없이 뜨는 성질이 ADR-007·023의 SEO 전제입니다. **깨져도 SSR은 멀쩡해서 배포는 성공으로 보입니다.**
- **응답 본문을 고치는 Cloudflare 기능을 켜지 마세요** — Rocket Loader, Mirage, Polish 등 (ADR-034).
  같은 이유입니다: 하이드레이션이 죽어도 배포는 성공으로 보이고 브라우저에서만 죽습니다.
- **`gh`를 직접 부르지 말고 `./scripts/gh`를 쓰세요.** 활성 계정이 머신 전역이라 다른 저장소를 오가면 바뀝니다.

## 열려 있는 질문

**지금은 없습니다.** 마지막 항목(번역 프로젝트의 공개 기준)은 ADR-036으로 닫혔습니다 —
**관리자가 손으로 공개합니다.**

새로 판단이 필요한 것이 생기면 임의로 정하지 말고 `Proposed` ADR로 남기고 확인을 요청하세요.
`docs/decisions/ADR-NNN.md` 파일을 만들고 [색인](docs/decisions/index.md)에 줄을 추가합니다.

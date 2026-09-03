# 설계 결정 이력 (ADR)

**이 표가 목차다. 필요한 항목만 열어 읽는다.**
전부 읽지 않는다 — 35개를 다 열면 컨텍스트가 결정으로만 찬다.

## 규칙

- 새 결정은 `ADR-NNN.md` 파일을 만들고 **이 표에 줄을 추가한다.**
- **기존 파일을 지우지 않는다.** 뒤집을 때는 상태를 `Superseded by ADR-NNN`으로 바꾸고 새 파일을 만든다.
- 상태는 `Accepted` / `Proposed` / `Superseded` / `Rejected` 넷이다.
- 번호는 비우지 않는다. `make docs-check`가 연속성과 표·파일 일치를 검사한다.

## 목차

| | 결정 | 상태 |
|---|---|---|
| [001](ADR-001.md) | 백엔드는 Go, MSA로 쪼개지 않는다 | Accepted |
| [002](ADR-002.md) | 배포는 Railway | Accepted |
| [003](ADR-003.md) | 잡 큐는 River (Redis 아님) | Accepted |
| [004](ADR-004.md) | 번역은 `paragraph_stable_id`에 결합한다 | Accepted |
| [005](ADR-005.md) | 문단당 확정 번역 1개를 스키마로 강제한다 | Accepted |
| [006](ADR-006.md) | 웹은 TanStack Start (SSR) | Accepted |
| [007](ADR-007.md) | 원문 페이지는 색인하지 않는다 | Accepted |
| [008](ADR-008.md) | 스토리지는 Cloudflare R2 | Accepted |
| [009](ADR-009.md) | API는 스펙 우선 (OpenAPI) | Accepted |
| [010](ADR-010.md) | LLM 번역 제안은 초기 범위에서 제외한다 | Accepted |
| [011](ADR-011.md) | 파서 검증을 스켈레톤보다 먼저 한다 | Accepted |
| [012](ADR-012.md) | 정규화에 `golang.org/x/text`를 쓴다 | Accepted |
| [013](ADR-013.md) | 파서 후처리: 앞부분 문단 보존, 빈 챕터, 제목 정리, 이미지 이니셜 | Accepted (2026-08-19) |
| [014](ADR-014.md) | 합본은 단권으로 수집하지 않는다 | Accepted (2026-08-19) |
| [015](ADR-015.md) | 희곡은 초기 지원 범위에서 제외한다 | Accepted (2026-08-19) |
| [016](ADR-016.md) | 반복 본문의 `stable_id` 충돌은 등장 순서로 분리한다 | Accepted (2026-08-19) |
| [017](ADR-017.md) | 마이그레이션은 goose, DB 드라이버는 pgx/v5 | Accepted (2026-08-19) |
| [018](ADR-018.md) | HTTP 라우터는 chi | Accepted (2026-08-19) |
| [019](ADR-019.md) | 패키지 매니저는 pnpm, Node 패키지는 `web/` 하나 | Accepted (2026-08-19) |
| [020](ADR-020.md) | 빌드 타임 코드 생성기는 `go.mod`의 tool 디렉티브로 고정한다 | Accepted (2026-08-19) |
| [021](ADR-021.md) | `oapi-codegen/runtime`은 생성 코드가 강제하는 런타임 의존성이다 | Accepted (2026-08-19) |
| [022](ADR-022.md) | 마이그레이션은 goose 먼저, River 나중 | Accepted (2026-08-19) |
| [023](ADR-023.md) | 챕터 색인 임계값은 승인 커버리지 80% | Accepted (2026-08-19) |
| [024](ADR-024.md) | 같은 문단의 승인은 자문 잠금으로 직렬화한다 | Accepted (2026-08-19) |
| [025](ADR-025.md) | 인라인 쪽번호를 본문에서 제거하고, 이미지 제목을 alt에서 복원한다 | Accepted (2026-08-19) |
| [026](ADR-026.md) | 브라우저의 교차 출처 호출은 출처 허용목록으로 받는다 | Accepted (2026-08-19) |
| [027](ADR-027.md) | 인증은 Google 위임, 세션은 Postgres 테이블, 관리자는 지정 계정 | Accepted (2026-08-20) |
| [028](ADR-028.md) | Cloudflare(D1 + Workers) 이전을 검토하고 현행 스택을 유지한다 | Accepted (2026-08-20) |
| [029](ADR-029.md) | 빌드는 Nixpacks가 아니라 서비스별 Dockerfile로 한다 | Accepted (2026-08-20) |
| [030](ADR-030.md) | 마이그레이션은 Railway pre-deploy 명령으로 실행한다 | Accepted (2026-08-20) |
| [031](ADR-031.md) | Railway 설정은 서비스별 `railway.json`에 둔다. 이미지는 하나로 합친다 | Accepted (2026-08-20) |
| [032](ADR-032.md) | 클라이언트 번들에 박히는 값은 빌드 인자로 받는다 | Accepted (2026-08-20) |
| [033](ADR-033.md) | 도메인은 `reclassic.dinnertimes.app`. API는 하이픈으로 옆에 둔다 | Accepted (2026-08-20) |
| [034](ADR-034.md) | 웹·API는 Cloudflare 프록시 뒤에 둔다 | Accepted (2026-08-25) |
| [035](ADR-035.md) | 편집·검수 화면의 프론트 스택을 고정한다 | Accepted (2026-08-25) |
| [036](ADR-036.md) | 번역 프로젝트의 공개는 관리자가 손으로 한다 | Accepted (2026-08-25) |
| [037](ADR-037.md) | 목록 API 응답은 배열이 아니라 `{ items }` 객체다 | Accepted (2026-08-25) |
| [038](ADR-038.md) | 화면 디자인은 모바일 우선이고, 토큰은 `styles.css` 한 곳에 둔다 | Accepted (2026-09-03) |

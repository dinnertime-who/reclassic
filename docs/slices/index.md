# 슬라이스 — 진행 상황

**한 슬라이스는 "질문 하나를 실물로 답한다"가 단위다.** 기능 목록이 아니다.
명세는 착수 전에 쓰고, 끝나면 실제로 확인된 수치를 명세에 되먹인 뒤 `completed/`로 옮긴다.

| | 슬라이스 | 명세 | 상태 |
|---|---|---|---|
| 1 | 골격 (walking skeleton) | [skeleton.md](completed/skeleton.md) | 완료 |
| 2 | 읽기 경로 — 적재 → API → SSR 화면 | [read-path.md](completed/read-path.md) | 완료 |
| 3 | 수집 자동화 (River + R2) | [ingest-automation.md](completed/ingest-automation.md) | 완료 |
| 4 | 번역 (제안·검수) | [translation.md](completed/translation.md) | 완료 |
| 5 | 세션 인증 | [auth.md](completed/auth.md) | 완료 |
| 6 | 배포 (Railway) | [deploy.md](completed/deploy.md) | 완료 (2026-08-25) |
| 7 | 편집·검수 화면, 도서 목록, 관리자 확인 큐 | **미작성** | **진행 중** |

## 슬라이스 7 — 지금 하는 것

**명세가 아직 없다.** `active/`에 쓰고 시작한다.

프론트 스택은 [ADR-035](../decisions/ADR-035.md)로 고정됐다 —
TanStack Query · Tailwind v4 · shadcn/ui · 폼 라이브러리 없음 · Vitest + ESLint.

**화면 작업으로 보이지만 아니다.** 아래 셋은 API도 쿼리도 없다.
`openapi.yaml`과 `internal/db/queries/`부터 손대야 한다. 자세한 것은
[tech-debt.md](../tech-debt.md)의 "결정은 있는데 코드가 없는 것".

| 조작 | API | 화면 |
|---|---|---|
| 도서 수집 지시 | `POST /admin/books` 있음 | 없음 |
| 번역 프로젝트 생성 | `POST /admin/projects` 있음 | 없음 |
| 제안 검수 | `POST /proposals/{id}/review` 있음 | 없음 |
| `needs_review` 큐 확인 | **없음** | 없음 |
| 승계 고아 번역 확인 | **없음** | 없음 |
| 역할 부여 (reviewer) | **없음** | 없음 |

**착수 전에 닫아야 할 결정이 하나 있다** — 번역 프로젝트 전체의 공개 기준.
`translation_projects.status`에 `published`가 있는데 그리로 가는 경로가 없다.
임의로 정하지 말고 `Proposed` ADR로 남기고 확인받을 것.

---

## 실물로 확인된 것 — 슬라이스별

각 슬라이스가 **무엇을 수치로 증명했는지**만 적는다. 설계 근거는 ADR에 있다.

### 1. 골격

도구 여섯(goose·sqlc·oapi-codegen·orval·docker compose·TanStack Start)이 한 저장소에서
맞물려 돌고, `GET /healthz` 하나가 Go API → SSR 화면까지 흐른다.
[ADR-009](../decisions/ADR-009.md)("계약 변경이 양쪽에서 컴파일 에러로 드러난다")와
[ADR-006](../decisions/ADR-006.md)(SSR)의 전제를 실물로 확인했다.

### 2. 읽기 경로

22권이 DB에 적재되고 golden과 수가 일치한다. 책 한 권이 **자바스크립트 없이** 브라우저에 뜬다.
셰익스피어 전집은 [ADR-014](../decisions/ADR-014.md) 게이트에 걸려 `needs_review`이고 읽기 조회는 404다.

**`stable_id` 승계율은 파서 미변경 상태에서 21권 37,125문단 100%다.**
[ADR-004](../decisions/ADR-004.md)가 전제한 "해시 일치 → 자동 승계"가 처음으로 수치로 확인됐다.
파서를 고친 뒤 `make succession`을 돌려 이 수치가 얼마나 떨어지는지 보는 것이 사용법이다.

### 3. 수집 자동화

`POST /admin/books` 한 번으로 FetchSource → R2 → ParseBook → 적재가 자동으로 돈다.
**[ADR-003](../decisions/ADR-003.md)의 근거("트랜잭션 안 enqueue")는 테스트로 못 박혀 있다** —
`internal/book/enqueue_integration_test.go`가 잡 등록이 실패하면 `books` 행도 남지 않는 것을 확인한다.
이걸 깨면 River를 쓸 이유가 사라진다.

### 4. 번역

제안 → 검수 → 확정본이 흐르고, 커버리지 80%를 넘으면 읽기 화면이 `index`로 바뀐다
([ADR-023](../decisions/ADR-023.md)). 사이트맵은 색인 대상만 담는다.

**여기서 [ADR-005](../decisions/ADR-005.md)의 구멍이 드러나 [ADR-024](../decisions/ADR-024.md)로 막았다.**
`WHERE status='pending'`은 같은 제안의 동시 승인만 막는다. 같은 문단의 **서로 다른** 제안을
동시에 승인하면 `approved`가 둘 남는다.

### 5. 세션 인증

실물 로그인까지 확인했다. Google 로그인 + Postgres 세션이고,
임시 헤더(`X-User-Handle`·`X-Admin-Token`)는 코드·계약·환경변수에서 전부 사라졌다.
관리자는 `ADMIN_EMAIL`과 일치하는 Google 계정이다 ([ADR-027](../decisions/ADR-027.md)).

### 6. 배포

**프로덕션이 실제로 떠 있다** — 웹 `https://reclassic.dinnertimes.app` ·
API `https://api-reclassic.dinnertimes.app`.

[ADR-002](../decisions/ADR-002.md)의 4서비스 구성이 처음으로 실물 검증됐다.
**`[::]` 바인딩과 `api.railway.internal`이 맞물리고**(불변식 4),
**웹과 API가 다른 서브도메인인데 세션 쿠키가 공유된다**([ADR-033](../decisions/ADR-033.md)).

관리자 로그인 후 도서 한 권(1342)을 지시해 **수집→R2→파싱→적재가 2.5초에 완주했고,
챕터 63 / 문단 2131이 골든과 정확히 일치**했다. 원본 sha256까지 같다.

[ADR-031](../decisions/ADR-031.md)의 미확인 항목은 없다 — 설정 파일 경로와 메모리 상한이
둘 다 실물에서 먹는 것을 확인했다. [ADR-028](../decisions/ADR-028.md)의 숙제도 닫혔다:
운영이 아프지 않았으므로 Cloudflare 이전을 열지 않는다.

**이 슬라이스가 남긴 부채는 [tech-debt.md](../tech-debt.md)에 있다.**

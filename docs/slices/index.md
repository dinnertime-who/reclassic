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
| 7 | 편집·검수 화면 (CSR) | [editor.md](completed/editor.md) | 완료 (2026-08-25) |
| 8 | 도서 목록 · 관리자 확인 큐 · 역할 부여 | 미작성 | **다음** |

## 슬라이스 8 — 지금 하는 것

**도서 목록 · 관리자 확인 큐 · 역할 부여.** 셋 다 **화면보다 없는 쿼리가 먼저다.**

| 조작 | API | 화면 |
|---|---|---|
| 도서 수집 지시 | `POST /admin/books` 있음 | 없음 |
| 번역 프로젝트 생성 | `POST /admin/projects` 있음 | 없음 |
| `needs_review` 큐 확인 | **없음** | 없음 |
| 승계 고아 번역 확인 | **없음** | 없음 |
| 역할 부여 (reviewer) | **없음** | 없음 |
| 프로젝트 공개 (`open → published`) | **없음** | 없음 |

목록은 [tech-debt.md](../tech-debt.md) D1·D2·D3·D5에 있다.
프로젝트 공개 방식은 [ADR-036](../decisions/ADR-036.md)으로 정해졌다 — **관리자가 손으로 한다.**

**슬라이스 7이 넘긴 숙제 하나:** 프로덕션에서 **제안 → 승인이 실제로 도는 것**을 아직 못 봤다.
프로덕션에 번역 프로젝트가 없어서였다(id 1~12 전부 404). **프로젝트 생성 화면이 생기는 순간
그 경로가 프로덕션에서 처음 돈다 — 그때 한 번 확인한다.**

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


### 7. 편집·검수 화면 (로컬까지)

**ADR-035가 고른 스택이 SSR 라우터 안에서 CSR 화면을 지탱한다.** 요청마다 만든 `QueryClient`가
SSR을 통과하고, 뮤테이션 뒤 부분 갱신이 돌고, 읽기 화면의 무자바스크립트 성질이 옆에서 유지된다.

**읽기 화면이 자바스크립트 실행 0으로 문단 38개를 SSR HTML에 담아 온다** — `curl`로 확인했다.
편집 라우트를 옆에 두고도 ADR-007·023의 SEO 전제가 살아 있다.

**계약은 한 줄도 늘지 않았다** — `openapi.yaml` 0줄, 읽기 라우트 셋 0줄, `api/http.ts` 0줄.
승인 한 번에 커버리지가 27/33 → 28/33으로 움직이고(챕터 뷰 무효화), 문단 33개를 그려도
`/proposals` 요청은 0건이다. `reviewProposal` 409는 실패가 아니라 재조회로 회복한다 (ADR-024).

**프로덕션에서도 확인했다** — Cloudflare를 통과한 뒤에도 읽기 화면이 자바스크립트 실행 0으로
본문을 낸다. 응답 본문에 Rocket Loader·Mirage 주입 흔적은 없다([ADR-034](../decisions/ADR-034.md)).
`VITE_` 둘이 프로덕션 값으로 박혔고 `localhost` 잔재는 0건이다([ADR-032](../decisions/ADR-032.md)).
**읽기 라우트 청크는 669·186바이트이고 API URL도 react-query도 0건이다** — react-query가 딸려
들어갔다면 수십 KB가 됐을 것이다. 다만 **제안 → 승인은 프로덕션에서 아직 못 돌렸다**(위 슬라이스 8 항목).

**여기서 조용한 고장이 둘 나왔다 — 둘 다 SSR·빌드·테스트가 전부 통과한 채로 숨어 있었다.**
Tailwind preflight가 읽기 화면 타이포를 납작하게 만든 것, 그리고 읽기 화면 순수성을 지키려고
넣은 ESLint 가드가 **정작 아무것도 잡지 못하고 있던 것**(group 패턴이 gitignore 문법이라
`#/`의 `#`이 주석으로 먹혔다). ADR-035가 경고한 실패 모양 그대로다.

**이 슬라이스가 남긴 부채는 [tech-debt.md](../tech-debt.md)에 있다.**

# 슬라이스 명세 — 편집·검수 화면 (CSR)

**선행 슬라이스:** [auth.md](../completed/auth.md) — 세션과 역할이 있어야 한다.
[deploy.md](../completed/deploy.md) — 프로덕션이 떠 있다.

**선행 문서:** `AGENTS.md`, [ARCHITECTURE.md](../../ARCHITECTURE.md),
[ADR-035](../../decisions/ADR-035.md)(프론트 스택), [ADR-024](../../decisions/ADR-024.md)(동시 승인),
[ADR-006](../../decisions/ADR-006.md)(SSR/CSR 분리)

**다음 슬라이스:** 도서 목록 · 관리자 확인 큐 · 역할 부여 — 아직 명세 없음

---

## 1. 이 작업이 답해야 할 질문

**ADR-035가 고른 스택이 SSR 라우터 안에서 CSR 화면을 실제로 지탱하는가?**

지금까지 화면은 전부 로더로만 돌았다. 뮤테이션도, 부분 갱신도, 낙관적 처리도 한 번도 안 해봤다.
ADR-035는 문서로만 존재하고 `web/`에는 아직 아무것도 설치돼 있지 않다.

부수적으로 답하는 것:

- **제안 → 검수 → 확정본이 화면으로 도는가.** 슬라이스 4에서 API로만 확인했다.
- **`QueryClient`를 요청마다 만드는 구성이 SSR에서 실제로 도는가** (ADR-035).
- **읽기 화면의 "자바스크립트 없이 뜬다"가 CSR 라우트를 옆에 두고도 유지되는가** (ADR-007).

### 이 슬라이스가 답하지 않는 것

- 검수자가 여럿일 때의 workflow. **지금 검수는 관리자 한 명만 할 수 있다** (§2 제약).
- 어떤 책을 번역할지 고르는 경로. 도서 목록이 없어 URL을 직접 친다.

## 2. 범위

### 하는 것

- **ADR-035 스택 설치** — react-query · Tailwind v4 · shadcn/ui · Vitest · ESLint
- **편집·검수 화면 하나** — `/projects/$projectId/chapters/$idx/edit` (CSR)
  - 문단 목록: 원문 · 확정 번역 · 대기 제안 수
  - 문단을 펼치면 그 문단의 제안 목록
  - 제안 작성
  - 검수 — 승인 / 반려 (권한이 있을 때만)
- **`make test` · `make lint` · `make fmt`가 web을 포함하도록 고친다**

### 하지 않는 것 — 건드리면 안 됨

- **새 API 엔드포인트.** 기존 계약 9개로 이 화면이 성립하는 것을 확인했다 (§4.1).
  **부족하다고 느끼면 먼저 이 명세를 의심할 것** — 계약을 늘리는 순간 슬라이스가 두 배가 된다.
- **도서 목록 · 관리자 확인 큐 · 역할 부여 UI** — 다음 슬라이스. 셋 다 없는 쿼리부터 만들어야 한다
  (`docs/tech-debt.md` D1·D3·D5).
- **프로젝트 공개 전이** (`open → published`). [ADR-036](../../decisions/ADR-036.md)으로 정해졌지만
  **쓰는 화면이 도서 목록이라 거기서 만든다.**
- **읽기 화면 수정.** `/projects/$projectId/chapters/$idx`는 그대로 둔다.
  편집 화면은 **옆에 새로 만드는 것**이지 기존 화면을 CSR로 바꾸는 것이 아니다.
- **용어집 UI** (`book_glossary`) — 테이블만 있는 의도된 미구현 (ADR-010).
- **낙관적 갱신.** 승인은 서버에서 트랜잭션 세 개가 도는 무거운 조작이고
  409로 실패할 수 있다 (ADR-024). 먼저 정직하게 무효화로 만든다.

### 알고 시작해야 하는 제약

- **검수는 지금 관리자만 할 수 있다.** `users.role`에 `reviewer`가 CHECK로 있고
  `auth.CanReview`도 있지만 **역할을 바꾸는 쿼리가 없다** (`docs/tech-debt.md` D3).
  관리자는 `ADMIN_EMAIL` 일치로만 정해진다 (ADR-027).
  화면은 `role`을 보고 분기하되, **`reviewer`가 실제로 존재할 수 없다는 것을 알고 만든다.**
- **문단마다 "내 제안 상태"를 주는 필드가 계약에 없다.** `TranslatedParagraph`는
  `stableId` · `sourceText` · `approvedTranslation` · `proposalCount`뿐이다.
  `ARCHITECTURE.md`의 스케치에 있는 `my_proposal_status`는 구현되지 않았다.
  **그래서 제안 목록은 펼친 문단만 부른다** (§4.5).

### 의존성

새로 추가하는 것은 ADR-035가 이미 정한 것뿐이다. **그 밖의 의존성을 늘리지 않는다.**
필요하면 `Proposed` ADR로 남기고 확인할 것.

## 3. 산출물

| 파일 | 내용 |
|---|---|
| `web/package.json` | ADR-035 의존성 |
| `web/vite.config.ts` | `@tailwindcss/vite` 플러그인 |
| `web/src/styles.css` | `@import "tailwindcss"` + `@theme` 토큰 |
| `web/components.json` | shadcn 설정 — **별칭을 `#/*`로 맞춘다** |
| `web/src/router.tsx` | 요청마다 `QueryClient` + SSR 통합 |
| `web/orval.config.ts` | `client: 'react-query'` |
| `web/src/api/gen/**` | 재생성 (손대지 않음) |
| `web/src/routes/projects.$projectId.chapters.$idx.edit.tsx` | 편집·검수 화면 |
| `web/src/components/ui/**` | shadcn 산출물 — **우리 소스다** (ADR-035) |
| `web/vitest.config.ts` · `web/eslint.config.js` | 테스트·린트 |
| `Makefile` | `test` · `lint` · `fmt`에 web 추가 |

## 4. 구현 명세

### 4.1 기존 계약으로 충분한지 — 확인 결과

| 화면이 해야 하는 것 | 쓰는 계약 |
|---|---|
| 문단 목록 + 확정 번역 + 제안 수 | `getProjectChapter` |
| 문단의 제안 목록 | `listProposals` |
| 제안 작성 | `createProposal` (401 / 409) |
| 승인 · 반려 | `reviewProposal` (401 / 403 / 404 / 409) |
| 내 신원과 권한 | `getCurrentUser` |

**새 엔드포인트가 필요 없다.** `openapi.yaml`을 고치지 않는다.

### 4.2 스택 설치

Tailwind v4는 **CSS-first**다. `tailwind.config.js`를 만들지 않는다.

```css
/* web/src/styles.css */
@import "tailwindcss";

@theme {
  /* 토큰은 여기 둔다. 지금 있는 읽기 화면 스타일은 남겨 둔다 */
}
```

`vite.config.ts`의 플러그인 순서는 기존 배열에 `tailwindcss()`를 더하는 것으로 족하다.

**`components.json`의 별칭을 `#/*`로 맞춘다.** shadcn 기본값은 `@/*`인데
이 저장소는 `web/package.json`의 `imports`로 `#/*`를 쓴다 (ADR-019).
**여기서 어긋나면 shadcn이 넣는 파일마다 import를 손으로 고치게 된다.**

컴포넌트는 `make ui-add C=button`으로 넣는다. **먼저 shadcn에 있는지 찾고, 없을 때만 직접 만든다** (ADR-035).

### 4.3 `QueryClient`는 요청마다 — SSR 규칙

```tsx
// web/src/router.tsx
export function getRouter() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: 60_000 } },  // 0이면 하이드레이션 직후 전부 재요청
  })
  const router = createTanStackRouter({
    routeTree, context: { queryClient },
    scrollRestoration: true, defaultPreload: 'intent', defaultPreloadStaleTime: 0,
  })
  setupRouterSsrQueryIntegration({ router, queryClient })
  return router
}
```

- **모듈 스코프에 두지 않는다.** 서버는 프로세스 하나가 모든 요청을 처리하므로,
  공유하면 **한 사람의 응답이 다른 사람에게 나간다.**
- **`QueryClientProvider`를 직접 감싸지 않는다.** `wrapQueryClient`가 기본 true다.
- `staleTime`(60초)과 `defaultPreloadStaleTime`(0)은 **다른 값이다.** 헷갈리지 말 것.

### 4.4 orval 전환

`client: 'fetch'` → `'react-query'`. **mutator(`src/api/http.ts`)는 그대로 둔다** —
베이스 URL 분기와 SSR 쿠키 전달은 여전히 한 곳이다 (CONVENTIONS).

`make generate`를 돌리고 **산출물까지 커밋한다.**
읽기 라우트 둘은 여전히 로더에서 생성 함수를 직접 부른다 — 훅으로 바꾸지 않는다 (§5).

### 4.5 편집·검수 화면

라우트 `/projects/$projectId/chapters/$idx/edit`.

- **문단 목록은 `getProjectChapter`로 한 번에 받는다.** 읽기 화면과 같은 계약이다.
- **제안 목록은 펼친 문단만 부른다.** 계약에 `myProposalStatus`가 없어서
  전부 부르면 문단 수만큼 요청이 나간다 — 챕터 하나에 수백 개다.
  `proposalCount`가 0이면 아예 부르지 않는다.
- 제안 작성 후 · 검수 후에는 **해당 문단의 제안 목록과 챕터 뷰를 무효화**한다.
  챕터 뷰를 빼먹으면 `approvedTranslation`과 `proposalCount`가 낡은 채 남는다.

**에러는 삼키지 않는다. 화면에 뜻을 보여준다.**

| 상태 | 언제 | 화면이 하는 것 |
|---|---|---|
| 401 | 비로그인 제안·검수 | 로그인을 안내한다 |
| 403 | 검수 권한 없음 | 검수 조작을 애초에 렌더하지 않는다 |
| 409 (`createProposal`) | 이미 대기 중인 제안이 있다 | 그 사실을 알린다 |
| 409 (`reviewProposal`) | **다른 검수자가 먼저 처리했다** (ADR-024) | **다시 조회한다.** 실패로 끝내지 않는다 |

### 4.6 권한

`getCurrentUser`의 `role`로 분기한다. **화면이 권한을 판정하지 않는다** — 서버가 판정하고
화면은 그 결과를 반영할 뿐이다. 403을 받을 수 있는 버튼을 처음부터 그리지 않는 것이 목적이지,
그것이 방어선인 것은 아니다.

### 4.7 테스트

**web 테스트는 `DATABASE_URL` 없이, API 없이 돈다.** orval 클라이언트를 모의한다.

- 문단 목록이 확정 번역과 원문을 옳게 가른다
- 제안 작성이 성공하면 목록이 갱신된다
- **`reviewProposal` 409를 받으면 재조회한다** — ADR-024가 만드는 상황이다
- 권한이 없으면 검수 조작이 렌더되지 않는다

## 5. 반드시 지킬 것

1. **`QueryClient`를 모듈 스코프에 두지 않는다** (ADR-035). 한 사람의 응답이 다른 사람에게 나간다.
2. **읽기 라우트에 react-query 훅이나 shadcn 컴포넌트를 넣지 않는다** (ADR-035).
   자바스크립트 없이 뜨는 성질이 ADR-007·023의 SEO 전제다.
   **깨져도 SSR은 멀쩡해 보여 배포가 성공으로 찍힌다.**
3. **`openapi.yaml`을 고치지 않는다.** 고쳐야 할 것 같으면 §2를 다시 읽는다.
4. **생성 코드를 손으로 고치지 않는다.** shadcn 산출물은 예외다 — 그건 우리 소스다 (ADR-035).
5. **`reviewProposal`의 409를 실패로 처리하지 않는다.** 재조회한다.
6. 작업 종료 전 `make lint && make test && make docs-check` 통과.

## 6. 완료 조건

- [ ] `make run-web`에서 편집 화면이 뜨고, **제안 → 승인 → 확정본 반영이 화면 안에서 돈다**
- [ ] 승인 직후 그 문단의 확정 번역과 커버리지가 갱신된다 (챕터 뷰 무효화 확인)
- [ ] **읽기 화면이 여전히 자바스크립트 없이 뜬다** — JS를 끄고 확인한다. 눈으로 볼 것
- [ ] 비로그인·비권한 상태에서 검수 조작이 렌더되지 않는다
- [ ] `reviewProposal` 409를 인위로 만들어 재조회로 회복하는 것을 확인한다
- [ ] `make test`가 Go와 web을 함께 돌린다. **`DATABASE_URL` 없이 통과한다**
- [ ] `make lint`가 ESLint를 돌린다
- [ ] 실물(`https://reclassic.dinnertimes.app`)에서 한 번 확인한다

## 7. 다음

**도서 목록 · 관리자 확인 큐 · 역할 부여.** 셋 다 **없는 쿼리부터 만들어야 한다** —
`docs/tech-debt.md`의 D1·D2·D3·D5다. [ADR-036](../../decisions/ADR-036.md)이 정한
프로젝트 공개 전이(`open → published`)도 거기서 구현한다.

이 슬라이스가 남기는 것은 `docs/tech-debt.md`에 적는다.

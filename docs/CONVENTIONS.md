# 코딩 규약

## 공통

- 들여쓰기·개행은 `.editorconfig`를 따른다.
- 주석과 문서는 한국어. 식별자·커밋 메시지 타입은 영어.
- 커밋·브랜치·PR 규칙은 아래 "기여 흐름" 절에 있다.

## Go

- 패키지는 `internal/` 아래에 도메인별로 둔다. `pkg/`는 만들지 않는다.
- 에러는 감싸서 올린다: `fmt.Errorf("fetch source %d: %w", id, err)`.
  같은 문장을 두 계층에서 반복하지 않는다.
- `context.Context`는 항상 첫 인자. 구조체 필드로 보관하지 않는다.
- 외부 I/O(HTTP, S3, DB)는 인터페이스 뒤에 두고 호출부는 인터페이스에 의존한다.
  테스트에서 갈아끼울 수 있어야 한다.
- 로그는 `log/slog`. 구조화 필드를 쓰고 문자열 포매팅으로 값을 섞지 않는다.
- 패닉을 제어 흐름으로 쓰지 않는다. 워커 잡 핸들러는 에러를 반환하고 River가 재시도하게 한다.

### 서버 바인딩

`[::]`에 바인딩한다. Railway 프라이빗 네트워크가 IPv6 전용이다.

```go
srv := &http.Server{Addr: "[::]:" + os.Getenv("PORT")}
```

### 클라이언트 IP

**`r.RemoteAddr`을 방문자 IP로 쓰지 않는다.** 프로덕션은 Cloudflare 프록시 뒤에 있어서
(ADR-034) 그 값은 Cloudflare의 주소다. 레이트리밋·남용 로깅·지역 판별을 넣을 때
전부 같은 소수의 IP로 뭉쳐 보인다 — **막으려던 것을 못 막고 애먼 것을 막는다.**

방문자 IP는 **`CF-Connecting-IP`** 헤더에 있다.

```go
ip := r.Header.Get("CF-Connecting-IP")
if ip == "" {
    ip = r.RemoteAddr // 로컬 개발. 프로덕션에서 여기로 오면 프록시 설정을 의심한다
}
```

**이 헤더를 신뢰할 수 있는 것은 오리진이 Cloudflare로만 노출돼 있을 때뿐이다.**
Railway 공개 도메인이 따로 열리면 아무나 이 헤더를 넣어 보낼 수 있다.

## SQL / 마이그레이션

- 마이그레이션은 `internal/db/migrations/`에 `NNN_설명.sql` 순번으로 추가한다.
- **적용된 마이그레이션 파일은 절대 수정하지 않는다.** 항상 새 파일을 추가한다.
- 쿼리는 `internal/db/queries/`에 작성하고 `make generate`로 sqlc 타입을 만든다.
- 생성된 Go 코드는 손으로 고치지 않는다.
- 여러 행을 함께 바꾸는 작업은 트랜잭션으로 감싼다. 특히 번역 승인(ARCHITECTURE.md 참조).

## TypeScript / 웹

- API 호출은 `orval` 생성 클라이언트만 사용한다. 직접 `fetch`를 쓰지 않는다.
- API 베이스 URL은 서버·클라이언트에서 분기한다.

```ts
const baseURL = typeof window === 'undefined'
  ? `http://${process.env.API_INTERNAL_HOST}:${process.env.API_PORT}`  // *.railway.internal
  : import.meta.env.VITE_API_URL
```

- SSR 중 Go API를 호출할 때 요청 쿠키를 전달한다. 빠뜨리면 SSR 결과가 항상 로그아웃 상태가 되고
  하이드레이션 후 화면이 깜빡인다.
- server function에는 비즈니스 로직을 넣지 않는다. SSR 데이터 페칭과 쿠키 전달 전용이다.

### 편집·검수 화면 (CSR) — ADR-035

- **`QueryClient`는 `getRouter()` 안에서 요청마다 새로 만든다. 모듈 스코프에 두지 않는다.**
  서버는 프로세스 하나가 모든 요청을 처리하므로, 공유하면 한 사람의 응답이 다른 사람에게 나간다.
- `QueryClientProvider`를 직접 감싸지 않는다. `setupRouterSsrQueryIntegration`이 기본으로 감싼다.
- 로더에서 SSR을 막고 기다리려면 `ensureQueryData()`. 막지 않고 흘리려면 `fetchQuery()`를 await 없이.
- **`useQuery`는 서버에서 돌지 않는다.** 하이드레이션 후 클라이언트에서만 실행된다.
  서버에서 돌려야 하면 `useSuspenseQuery`다.
- `staleTime`을 0보다 크게 둔다(60초). 0이면 하이드레이션 직후 전부 다시 요청한다.
- **읽기 화면(`/books/…`·`/projects/…`)에 react-query 훅을 넣지 않는다.** 데이터는 로더가 가져온다.
  Tailwind 클래스는 써도 되고, **접근성 동작이 붙는 위젯(다이얼로그·시트·팝오버 등)은 shadcn을
  쓴다** — 손으로 만들지 않는다 ([ADR-043](decisions/ADR-043.md)). 본문 리듬은 여전히
  `styles.css`의 시맨틱 클래스가 갖는다 (ADR-038). 지켜야 할 것은 **본문이 SSR HTML에 담겨 자바스크립트 없이
  렌더되는 것**이고(ADR-007·023), 그 위에 얹는 작은 스크립트는 허용된다 — 읽기 설정이 그것이다
  ([ADR-040](decisions/ADR-040.md)). 스크립트가 없으면 본문이 안 보이는 구조는 금지다.
- **읽기 설정 쿠키(`reclassic_reader`) 값을 CSS에 그대로 넣지 않는다.** 단계 번호를 허용 목록으로
  좁혀 미리 정해 둔 값에만 매핑한다. **문자열을 `style`에 이어 붙이면 CSS 주입이다** (ADR-040).
- 컴포넌트는 **shadcn/ui에 있는지 먼저 찾고, 없을 때만 직접 만든다.**
  shadcn이 복사해 넣은 파일은 우리 소스이고 고쳐도 된다 — 다만 고친 것을 다시 `add`하지 않는다.
- **shadcn에 있는 컴포넌트를 손으로 다시 쓰지 않는다. 반드시 `make ui-add C=<이름>` 으로 넣는다.**
  시트·다이얼로그·팝오버처럼 겉보기가 단순한 것일수록 손으로 만들면 포커스 트랩·Esc·스크롤 잠금·
  포커스 복귀·`aria` 배선이 조용히 빠진다. **그 결함은 눈에도 테스트에도 안 보인다.**
  CLI 산출물을 복사해 붙이는 것도 안 된다 — 버전과 의존성이 갈린다.
- **새 shadcn 컴포넌트를 넣으면 그것이 쓰는 토큰이 `styles.css`에 있는지 먼저 확인한다.**
  `styles.css`는 **지금 쓰는 토큰만** 옮겨 둔 상태다(ADR-035). 없는 토큰(`--popover` 등)을 쓰는
  컴포넌트를 넣으면 **배경이 해석되지 않은 채로 렌더되고 빌드는 성공한다.**
  없으면 shadcn 원본에서 그 줄만 더 가져와 `:root`·`.dark`·`@theme inline` 세 곳에 함께 넣는다.

### 화면 스타일 — ADR-038

- **모바일이 기본이다.** 넓은 폭은 `min-width` 미디어 쿼리로 더한다. `max-width`로 좁은 쪽을
  깎지 않는다 — 그렇게 쓰면 새 화면이 기본으로 넓은 폭을 물려받는다.
- **색값과 px를 화면 파일에 쓰지 않는다.** 전부 `styles.css`의 CSS 변수에서 나간다.
  거기 없는 값이 필요하면 변수를 먼저 만든다.
- **터치 표적은 44px(`--tap`) 이상.** shadcn Button 기본은 36px이라 모바일에서 부족하다 —
  `styles.css`가 좁은 폭에서 키운다. 헤더 안은 예외다.
- 본문 크기는 `clamp()`로 흐르게 둔다. **중단점마다 `font-size`를 다시 적지 않는다** —
  그 사이에서 크기가 튄다.
- 읽기 화면의 리듬(행간·문단 간격·본문 폭)은 **`styles.css`의 시맨틱 클래스에 모은다.**
  유틸리티로 흩으면 라우트 네 곳에 복사되고, 한 곳을 고칠 때 나머지가 조용히 어긋난다.
- **시맨틱 클래스 이름을 Tailwind 유틸리티와 겹치지 않게 짓는다.** 겹치면 유틸리티가
  같이 나가서 우리 규칙을 덮는다 — `main.contents`가 `display: contents`에 걸려 상자를
  통째로 잃었던 것이 그 예다(그래서 `contents-page`다). **SSR도 빌드도 테스트도 통과하고
  브라우저에서만 드러난다.** 흔한 지뢰: `contents` · `grid` · `flex` · `table` · `visible` · `hidden`.

### 본문 서체 — ADR-039

- **세리프는 본문과 제목에만 쓴다.** 헤더·메타·버튼 같은 UI 크롬은 시스템 산세리프(`--font-sans`)로
  남긴다. 폰트 비용을 본문에만 쓰는 것이 그 결정이다.
- **`@fontsource/noto-serif-kr/korean.css`(와 `korean-<weight>.css`)를 import하지 않는다.**
  그것은 **한 덩어리 949KB**다. 쓰는 것은 `400.css` — `unicode-range`가 붙은 서브셋 124벌이라
  브라우저가 **화면에 실제로 쓰인 글자가 든 조각만** 받는다.
  **잘못 import해도 화면은 똑같이 뜬다.** 드러나는 곳은 청구서와 느린 회선뿐이다.
- **`font-display`는 `optional`이다.** Fontsource CSS는 `swap`으로 오므로 `vite.config.ts`의
  플러그인이 바꾼다. `@font-face` 디스크립터는 캐스케이드로 덮이지 않아 거기서 고칠 수밖에 없다.
- web 의존성은 **`make web-add PKG=…`** 로 더한다. `package.json`과 `pnpm-lock.yaml`을 함께 커밋한다.

### 모션 — ADR-041

- **모션은 `@media (prefers-reduced-motion: no-preference)` 안에서만 켠다.** `reduce`에서
  끄는 것이 아니라 `no-preference`에서만 켜는 것이다 — 그래야 규칙을 빠뜨렸을 때
  **모션이 걸리지 않는 쪽으로** 틀린다. **빼먹어도 아무 테스트도 실패하지 않는다.**
- **방향이 뜻을 가진다.** 앞으로 가는 이동은 오른쪽에서 들어오고, 뒤로 가는 이동은
  왼쪽에서 들어온다. 방향 없는 페이드는 "바뀌었다"만 말하고 "어디로 갔는지"를 말하지 않는다.
- 시간·이징·거리는 `styles.css`의 **`--motion-*` 토큰**에서 나간다. 화면 파일에 ms나 px를
  쓰지 않는다 (ADR-038).
- **클라이언트 이동은 CSS가 시작하지 않는다** — 라우터가 `document.startViewTransition`을
  부른다. 그래서 `reduce`에서도 전환 자체는 돈다. `::view-transition-old/new(root)`의
  `animation: none`만 미디어 쿼리 밖에 두는 이유가 이것이다. 없으면 `reduce`인데도
  **화면 전체가 흐려졌다 켜진다.**

## 보안

- **세션 토큰을 평문으로 저장하지 않는다.** DB에는 sha256만 넣는다 (ADR-027).
- **OAuth `state` 검증을 빼지 않는다.** 로그인 CSRF가 열린다.
- **프로덕션에서 `COOKIE_SECURE=true`.** 로컬에서만 false다.
- **CORS 허용 출처에 `*`를 넣지 않는다.** credentials와 함께 쓸 수 없고, 써서도 안 된다 (ADR-026).
  LAN·Tailscale로 접속하려면 그 출처를 `CORS_ALLOWED_ORIGINS`에 추가한다.
- 시크릿을 커밋하지 않는다. 새 환경변수는 `.env.example`에 이름과 한 줄 설명만 추가한다.
  `railway.json`도 커밋되는 파일이다 — 값은 Railway 변수로 간다 (ADR-031).

## 수집 (Gutenberg)

- **병렬 요청을 보내지 않는다.** 워커 동시성 1~2, 요청 간 최소 1초, User-Agent 명시.
  공격적으로 긁으면 IP가 차단되고 복구가 어렵다.
- **수집 큐(`fetch`)의 동시성을 1보다 올리지 않는다.** 파싱 큐(`parse`)는 올려도 된다.
  **큐를 나눈 이유가 그것이다.**
- **`PARSE_CONCURRENCY`를 메모리 확인 없이 올리지 않는다.** 셰익스피어 전집 파싱이 힙 310MB를 쓴다 (ADR-029).
- **원본 HTML을 커밋하지 않는다.** `.cache/`는 gitignore 대상이다.
  검증용 도서는 `internal/parse/testdata/corpus.json`에 ID만 기록하고 `make fetch-corpus`로 받는다.

## 테스트

- 파서는 `internal/parse/testdata/golden/`의 스냅샷으로 회귀를 검증한다.
- 골든 파일에는 원문 텍스트를 넣지 않는다. 챕터 수, 챕터별 문단 수, 챕터 제목,
  사용된 전략, 신뢰도, 본문 길이와 해시만 기록한다.
- 파서를 고친 뒤에는 골든 diff를 반드시 눈으로 확인한다. 무비판적으로 갱신하지 않는다.

## 환경변수

- 새 변수는 `.env.example`에 이름과 한 줄 설명을 추가한다. 실제 값은 커밋하지 않는다.
- 코드에서 읽을 때 기본값을 조용히 채우지 않는다. 없으면 기동 시점에 실패시킨다.

## 기여 흐름

`main`은 보호 브랜치다. 직접 push하지 않고 PR로 들어간다.

### 브랜치 이름

`<타입>/<한두-단어-요약>`. 타입은 커밋 타입과 같은 집합을 쓴다.

```
feat/session-auth
fix/inline-page-markers
docs/deploy-slice
chore/bump-pnpm
```

### 커밋 메시지

Conventional Commits. 타입은 여섯이다:
`feat` · `fix` · `refactor` · `docs` · `chore` · `test`

```
<타입>: <제목 — 한국어, 마침표 없음, 명령형>

<본문 — "왜"를 쓴다. "무엇"은 diff가 이미 말해준다.>
```

- **제목에 "무엇을 했다"를 쓰지 않는다.** 그건 diff에 있다.
  왜 그렇게 했는지, 무엇을 고르지 않았는지가 남아야 할 것이다.
- 본문에는 **검증한 수치**를 적는다. "테스트 통과"가 아니라 "22권 golden 일치",
  "승계율 100%", "40라운드 중 0건 실패"처럼.
- 설계 판단이 섞였으면 **ADR 번호를 적는다** (`(ADR-024)`).
- 되돌리기 어려운 결정이나 남는 위험은 본문에 남긴다. 나중에 그것만 읽게 된다.

### PR

**PR은 `make pr`로 만든다.** `gh`를 직접 부르면 머신 전역 활성 계정이 쓰이는데,
그 계정이 이 저장소 협업자가 아니면 `must be a collaborator`로 막힌다.
`scripts/gh`가 keyring에서 토큰을 꺼내 계정을 고정한다. `gh auth switch`는 쓰지 않는다 —
전역 상태를 바꾸므로 되돌리는 것을 잊는다.

**머지는 squash다.** PR 하나가 `main`에 커밋 하나로 들어간다.

**머지하면 그 브랜치를 지운다.** `./scripts/gh pr merge <번호> --squash --delete-branch`.
남겨 두면 두 가지가 따라온다 — 브랜치 목록이 쌓여 지금 사는 것이 무엇인지 흐려지고,
**그 브랜치를 base로 삼은 PR이 자동으로 `main`으로 재조준되지 않는다.**
GitHub은 base 브랜치가 **삭제될 때만** 열린 PR을 옮긴다. 실제로 그것 때문에 쌓인 PR의
base를 손으로 고친 적이 있다.

- **PR 제목이 곧 커밋 제목이다.** 따라서 PR 제목도 Conventional Commits를 따른다.
- PR 본문이 곧 커밋 본문이 된다. 위 커밋 규칙을 그대로 적용한다.
- 작업 중 커밋은 지저분해도 된다. squash되므로 정리에 시간 쓰지 않는다.
- 슬라이스 하나 = PR 하나를 기본으로 한다. 슬라이스가 크면 나누되,
  **각 PR이 그 자체로 동작하는 상태여야 한다.**

### PR 전 체크리스트

`.github/pull_request_template.md`가 같은 내용을 담고 있다.

1. `make lint && make test && make docs-check` 통과
2. **`DATABASE_URL` 없이도** `make test` 통과 (CI가 DB 없이 돈다)
3. 아키텍처에 영향을 주는 판단이 있었으면 **`docs/decisions/ADR-NNN.md` 파일 추가 +
   `docs/decisions/index.md` 표에 줄 추가**. 둘 중 하나만 하면 `make docs-check`가 잡는다
4. `openapi.yaml`을 고쳤으면 `make generate`를 돌리고 산출물까지 커밋
5. 마이그레이션을 추가했으면 **기존 파일은 건드리지 않았는지** 확인
6. 파서를 고쳤으면 `make parsecheck`로 눈 검증 후 golden 갱신,
   `make succession`으로 승계 영향 측정
7. 환경변수를 추가했으면 `.env.example`에 이름과 한 줄 설명 추가
8. 명령을 추가했으면 Makefile과 `AGENTS.md` 명령어 표를 함께 갱신
9. **알면서 남긴 것이 있으면 `docs/tech-debt.md`에 기록** — "언제 아플 것인가"까지 적는다

### 배포 확인

**`/healthz`의 `version`이 머지 커밋과 같다는 것으로 끝내지 않는다.** 그것은 "새 이미지가
떴다"만 말한다. 실제로 렌더된 화면을 본다 — 최소한 **SSR HTML이 참조하는 자산이 실제로
200으로 서빙되는지**까지 (ADR-042).

**자산 경로·발행 여부를 건드리는 변경은 Docker 이미지로 검증한다.**
`make docker-build-web` 으로 이미지를 만들고 컨테이너를 띄워 확인한다 —
`make lint`·`make test`·로컬 `pnpm build` 어느 것도 이 유형을 잡지 못한다. 실제로 놓쳤다.

### `main` 브랜치 보호 — 권장 설정

| 설정 | 값 | 이유 |
|---|---|---|
| Require a pull request before merging | 켬 | 직접 push를 막는다 |
| Require approvals | **끔** | 1인 개발이라 자기 PR을 승인할 수 없어 막힌다 |
| Require status checks | CI를 붙인 뒤 켬 | 지금은 CI가 없다 |
| Require linear history | 켬 | squash 머지와 맞는다 |
| Do not allow bypassing | 취향 | 켜면 관리자도 우회 못 한다. 혼자면 과할 수 있다 |
| Allow force pushes / deletions | **끔** | 사고는 대개 자기 손에서 난다 |

저장소 Settings → General → Pull Requests에서 **Allow squash merging만 켜고
나머지 둘은 끄는 것**을 권한다. 실수로 다른 방식으로 머지되는 것을 막는다.

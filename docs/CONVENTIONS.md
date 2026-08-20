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

- **PR 제목이 곧 커밋 제목이다.** 따라서 PR 제목도 Conventional Commits를 따른다.
- PR 본문이 곧 커밋 본문이 된다. 위 커밋 규칙을 그대로 적용한다.
- 작업 중 커밋은 지저분해도 된다. squash되므로 정리에 시간 쓰지 않는다.
- 슬라이스 하나 = PR 하나를 기본으로 한다. 슬라이스가 크면 나누되,
  **각 PR이 그 자체로 동작하는 상태여야 한다.**

### PR 전 체크리스트

`.github/pull_request_template.md`가 같은 내용을 담고 있다.

1. `make lint && make test` 통과
2. **`DATABASE_URL` 없이도** `make test` 통과 (CI가 DB 없이 돈다)
3. 아키텍처에 영향을 주는 판단이 있었으면 `docs/DECISIONS.md`에 ADR 추가
4. `openapi.yaml`을 고쳤으면 `make generate`를 돌리고 산출물까지 커밋
5. 마이그레이션을 추가했으면 **기존 파일은 건드리지 않았는지** 확인
6. 파서를 고쳤으면 `make parsecheck`로 눈 검증 후 golden 갱신,
   `make succession`으로 승계 영향 측정
7. 환경변수를 추가했으면 `.env.example`에 이름과 한 줄 설명 추가
8. 명령을 추가했으면 Makefile과 `AGENTS.md` 명령어 표를 함께 갱신

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

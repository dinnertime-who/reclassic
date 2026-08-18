# 코딩 규약

## 공통

- 들여쓰기·개행은 `.editorconfig`를 따른다.
- 주석과 문서는 한국어. 식별자·커밋 메시지 타입은 영어.
- 커밋 메시지는 Conventional Commits: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, `test:`
  본문은 "왜"를 쓴다. "무엇"은 diff가 이미 말해준다.

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

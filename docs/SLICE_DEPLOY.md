# 슬라이스 명세 — 배포 (Railway)

**작업 종류:** 구현 슬라이스 (인프라)
**선행 슬라이스:** `docs/SLICE_AUTH.md` — 인증이 없으면 배포하면 안 된다.
임시 헤더를 걷어낸 뒤에야 시작할 수 있다.
**선행 문서:** `AGENTS.md`, `docs/ARCHITECTURE.md` (특히 "핵심 불변식 4"),
`docs/DECISIONS.md` (특히 ADR-001 / 002 / 008 / 019 / 022 / 026 / 027 / 028 / 029)

---

## 1. 이 작업이 답해야 할 질문

ADR-002가 Railway 4서비스를 정한 지 오래됐지만 **실물로 돌아본 적이 없다.**

> **1. `[::]` 바인딩과 `*.railway.internal`이 실제로 맞물리는가?**
> **2. 웹과 API가 다른 서브도메인일 때 세션 쿠키가 공유되는가?**
> **3. 운영이 실제로 아픈가?**

1번은 불변식 4다. Railway 프라이빗 네트워크가 IPv6 전용이라는 전제 위에
`[::]` 바인딩 규칙이 서 있는데, 검증된 적이 없다.

3번은 ADR-028이 남긴 숙제다. Cloudflare 이전을 검토하고 "운영이 실제로 아프면 다시 연다"로
닫았는데, 그 판단의 근거가 이 슬라이스에서 나온다. **아프면 안 C를 꺼낸다.**

---

## 2. 범위

### 하는 것

- 서비스별 Dockerfile (ADR-029)
- Railway 4서비스 구성과 환경변수
- 마이그레이션 실행 자리 결정 (§4.4 — **미결**)
- 도메인 연결과 프로덕션 Google OAuth 클라이언트
- 배포 후 확인: 로그인, 수집 한 권, 읽기 화면

### 하지 않는 것

- **CI 파이프라인.** 배포가 손으로 되는 것을 먼저 확인한다
- **관측성(OTel), 알림, 백업 정책**
- **CDN 캐시 무효화.** ARCHITECTURE가 예고했지만 트래픽이 없다
- 화면 추가, 기능 추가

### 사전에 정해야 하는 것 — 착수 블로커

| | 상태 |
|---|---|
| **도메인** | **미정.** 나머지 환경변수 값이 전부 여기 의존한다 |
| **마이그레이션 실행 위치** | **미정.** §4.4 |

---

## 3. 산출물

| 경로 | 내용 |
|---|---|
| `Dockerfile` | Go 서비스 공용. `--build-arg CMD=api\|worker` |
| `web/Dockerfile` | TanStack Start (Node) |
| `.dockerignore` | `.cache/`·`node_modules`·`bin/` 제외 |
| `docs/DECISIONS.md` | 마이그레이션 실행 위치 ADR |
| `AGENTS.md` | 배포 절차 |

---

## 4. 구현 명세

### 4.1 빌드는 Dockerfile로 한다 (ADR-029)

Nixpacks를 쓰지 않는다. **메모리 때문이 아니다** — 근거는 §4.2에 있다.

- **결정성.** 이 저장소는 생성기 버전을 `go.mod` tool로 박고(ADR-020),
  pnpm을 `packageManager`로 고정한다(ADR-019). Nixpacks 자동 감지는 배포마다
  달라질 수 있어 이 문화와 맞지 않는다.
- **어느 바이너리인지 명시.** `cmd/`에 넷(`api`·`worker`·`ingest`·`parsecheck`)이 있다.
- **이미지 크기.** Go는 multi-stage + distroless로 20~30MB. 배포가 빠르고 공격 표면이 준다.

**Go Dockerfile — 요점**

```dockerfile
# 빌드 단계는 소스 전체가 필요하다.
# internal/db/migrations/*.sql이 바이너리에 embed된다 (internal/db/migrate.go).
FROM golang:1.26 AS build
ARG CMD=api
...
RUN CGO_ENABLED=0 go build -o /out/app ./cmd/${CMD}

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
```

- `CGO_ENABLED=0`. distroless static에 libc가 없다.
- 런타임 이미지에 SQL 파일을 싣지 않는다. 이미 바이너리 안에 있다.
- `.env`를 이미지에 넣지 않는다. Railway 환경변수로 주입한다.

**web Dockerfile — 요점**

```dockerfile
FROM node:24 AS build
RUN corepack enable          # packageManager 필드의 pnpm 10.28.1 강제 (ADR-019)
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
...
RUN pnpm build

FROM node:24-slim
COPY --from=build /app/.output ./.output   # devDependencies를 싣지 않는다
CMD ["node", ".output/server/index.mjs"]
```

- **`corepack enable`을 빼지 말 것.** Nixpacks에 맡기면 pnpm 버전 고정이 보장되지 않는다.
- `orval.config.ts`가 `../openapi.yaml`을 참조하므로 **빌드 컨텍스트는 저장소 루트**다.
  `web/`만 컨텍스트로 잡으면 생성이 깨진다.

### 4.2 메모리 산정 — 실측

**이미지 크기와 런타임 RSS는 다른 축이다.** Dockerfile로 줄어드는 것은 앞쪽이고,
이 서비스의 메모리를 정하는 것은 파서다.

`parse.EvaluateHTML` 실측 (2026-08-20):

| 도서 | 입력 | 최대 힙 | 누적 할당 |
|---|---|---|---|
| 100 셰익스피어 전집 | 7.3MB | **310MB** | 1,813MB |
| 4300 율리시스 | 1.7MB | 58MB | 331MB |
| 1342 오만과 편견 | 0.9MB | 29MB | 108MB |

전략 넷을 모두 돌리고 각각 DOM을 순회하므로 입력의 40배가 넘는 힙을 쓴다.

| 서비스 | 메모리 | 근거 |
|---|---|---|
| `worker` | **1GB** | 최악 310MB + GC 여유. `PARSE_CONCURRENCY`를 올리면 배수로 는다 |
| `api` | 512MB | 요청당 챕터 하나만 읽는다 |
| `web` | 512MB | SSR |

**`PARSE_CONCURRENCY`를 메모리 확인 없이 올리지 말 것.** 2면 최악이 620MB다.

### 4.3 서비스 구성

넷 다 같은 저장소를 본다.

| 서비스 | Dockerfile | 빌드 인자 |
|---|---|---|
| `postgres` | — | Railway 관리형 |
| `api` | `Dockerfile` | `CMD=api` |
| `worker` | `Dockerfile` | `CMD=worker` |
| `web` | `web/Dockerfile` | — |

**`api`는 `[::]`에 바인딩한다** (불변식 4). 이미 그렇게 돼 있고, 이 슬라이스가 그것을 처음 검증한다.

### 4.4 마이그레이션 실행 자리 — **미결**

`make migrate`는 `tools/migrate`를 `go run`으로 돌린다. Railway에는 그 자리가 없다.

| 안 | 장점 | 단점 |
|---|---|---|
| A. pre-deploy 명령 | 배포마다 자동, 배포당 한 번만 실행 | 실패 시 배포가 막힌다 |
| B. 일회성 서비스 수동 실행 | 통제 가능 | 배포할 때마다 잊기 쉽다 |
| C. api 기동 시 자동 | 설정 없음 | **인스턴스가 여럿이면 경합.** ADR-022가 기각한 방식 |

Dockerfile을 쓰면 A가 자연스럽다 — `tools/migrate`를 같은 이미지에 넣고
pre-deploy로 부르면 된다. 배포당 한 번만 도니 ADR-022의 경합 우려가 해소된다.
**정하고 ADR로 남길 것.**

### 4.5 환경변수

**`api`**

| 키 | 값 |
|---|---|
| `PORT` | Railway 자동 주입 |
| `DATABASE_URL` | Postgres 서비스 참조 |
| `CORS_ALLOWED_ORIGINS` | `https://<웹도메인>` (ADR-026) |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | 프로덕션 클라이언트 |
| `GOOGLE_REDIRECT_URL` | `https://<api도메인>/auth/google/callback` |
| `LOGIN_SUCCESS_REDIRECT` | `https://<웹도메인>/` |
| `ADMIN_EMAIL` | 마스터 계정 (ADR-027) |
| `COOKIE_SECURE` | **`true`** |
| `COOKIE_DOMAIN` | `.<도메인>` — 앞의 점이 서브도메인 공유를 만든다 |

**`worker`**

`DATABASE_URL` / `R2_ACCOUNT_ID` / `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_BUCKET` /
`GUTENBERG_USER_AGENT` / `GUTENBERG_MIN_INTERVAL_MS=1000` / `PUBLIC_BASE_URL`

**`S3_ENDPOINT`를 비운다.** 값이 있으면 MinIO 모드(path-style + 버킷 자동 생성)로 간다.

**`web`**

| 키 | 값 |
|---|---|
| `VITE_API_URL` | `https://<api도메인>` — 브라우저용 |
| `VITE_LOGIN_URL` | `https://<api도메인>/auth/google/start` |
| `API_INTERNAL_HOST` | `api.railway.internal` — SSR용 |
| `API_PORT` | `8080` |

마지막 둘이 불변식 4의 "API 베이스 URL은 서버·클라이언트에서 서로 달라야 한다"이다.

### 4.6 프로덕션 Google OAuth

로컬 클라이언트와 **별도로** 만들거나 리디렉션 URI를 추가한다.

- 승인된 리디렉션 URI: `https://<api도메인>/auth/google/callback`
- 동의 화면을 테스트에서 **프로덕션으로 게시**해야 테스트 사용자 밖에서도 로그인된다

---

## 5. 반드시 지킬 것

1. **`[::]`에 바인딩한다.** `0.0.0.0`은 서비스 간 내부 호출을 받지 못한다.
2. **`COOKIE_SECURE=true`.** 로컬에서만 false다.
3. **`S3_ENDPOINT`를 비운다.** 프로덕션에서 MinIO 모드로 가면 안 된다.
4. **`GUTENBERG_MIN_INTERVAL_MS`를 1000 미만으로 낮추지 않는다.**
5. **`PARSE_CONCURRENCY`를 메모리 확인 없이 올리지 않는다.** §4.2.
6. **시크릿을 이미지에 굽지 않는다.** `.env`는 `.dockerignore` 대상이다.
7. 작업 종료 전 `make lint && make test` 통과.

---

## 6. 완료 조건

- [ ] `docker build --build-arg CMD=api .`와 `CMD=worker`가 각각 바이너리 하나만 담은 이미지를 만든다
- [ ] `web/Dockerfile`이 pnpm 10.28.1로 빌드하고 런타임 이미지에 devDependencies가 없다
- [ ] 4서비스가 Railway에 뜨고 `/healthz`가 `db: ok`를 준다
- [ ] **SSR이 `api.railway.internal`로 API를 부른다** — 공개 도메인을 경유하지 않는다 (불변식 4)
- [ ] 프로덕션 도메인에서 Google 로그인이 되고, `ADMIN_EMAIL` 계정이 `admin`이 된다
- [ ] **웹과 API가 다른 서브도메인인데 세션 쿠키가 공유된다** (`COOKIE_DOMAIN`)
- [ ] 관리자가 도서 한 권을 지시하면 수집·파싱·적재가 끝까지 돈다
- [ ] 읽기 화면이 프로덕션에서 뜨고 `noindex`가 붙어 있다
- [ ] 마이그레이션이 배포 흐름 안에서 한 번만 실행된다 (§4.4)
- [ ] `make lint && make test` 통과

---

## 7. 다음

이 슬라이스가 끝나면 **ADR-028의 숙제에 답할 수 있다** — 운영이 실제로 아픈가.
아프면 안 C(Workers + Hyperdrive + Postgres)를 다시 연다. 아니면 화면 슬라이스로 간다.

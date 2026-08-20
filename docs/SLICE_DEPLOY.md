# 슬라이스 명세 — 배포 (Railway)

**작업 종류:** 구현 슬라이스 (인프라)
**선행 슬라이스:** `docs/SLICE_AUTH.md` — 인증이 없으면 배포하면 안 된다.
임시 헤더를 걷어낸 뒤에야 시작할 수 있다.
**다음 슬라이스:** 편집·검수 화면 — 아직 명세 없음
**선행 문서:** `AGENTS.md`, `docs/ARCHITECTURE.md` (특히 "핵심 불변식 4"),
`docs/DECISIONS.md` (특히 ADR-001 / 002 / 008 / 019 / 022 / 026 / 027 / 028 / 029 / 030 / 031 / 032 / 033)

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
- **서비스별 `railway.json`으로 빌드·배포 설정을 저장소에 둔다** (ADR-031)
- 마이그레이션을 pre-deploy 명령으로 실행 (§4.4, ADR-030)
- 도메인 연결과 프로덕션 Google OAuth 클라이언트
- 배포 후 확인: 로그인, 수집 한 권, 읽기 화면

### 하지 않는 것

- **CI 파이프라인.** 배포가 손으로 되는 것을 먼저 확인한다
- **관측성(OTel), 알림, 백업 정책**
- **CDN 캐시 무효화.** ARCHITECTURE가 예고했지만 트래픽이 없다
- **`.railway/railway.ts`(TypeScript IaC)로의 이전.** 왜 지금이 아닌지는 ADR-031
- 화면 추가, 기능 추가

### 사전에 정해야 하는 것 — 착수 블로커

| | 상태 |
|---|---|
| **마이그레이션 실행 위치** | **정해졌다 — pre-deploy 명령 (ADR-030).** §4.4 |
| **설정을 어디에 두는가** | **정해졌다 — 서비스별 `railway.json` (ADR-031).** §4.3 |
| **도메인** | **정해졌다 (2026-08-20).** 웹 `reclassic.dinnertimes.app` · API `api-reclassic.dinnertimes.app` (ADR-033) |

도메인은 파일을 쓰는 데는 필요 없다 — Dockerfile도 `railway.json`도 도메인을 모르고,
환경변수는 저장소가 아니라 Railway에 들어간다.
**Railway를 실제로 설정하는 시점에 필요하다.** §6.1에 절차를 모아 뒀다.

---

## 3. 산출물

| 경로 | 내용 |
|---|---|
| `Dockerfile` | Go 서비스 공용. `api`·`worker`·`migrate` 셋을 한 이미지에 |
| `web/Dockerfile` | TanStack Start (Node) |
| `.dockerignore` | `.cache/`·`node_modules`·`bin/` 제외 |
| `railway.api.json` | `dockerfilePath`·`startCommand=/api`·`preDeployCommand=/migrate`·메모리 |
| `railway.worker.json` | `dockerfilePath`·`startCommand=/worker`·메모리 |
| `railway.web.json` | `dockerfilePath=web/Dockerfile`·메모리 |
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
...
RUN CGO_ENABLED=0 go build -o /out/api     ./cmd/api      \
 && CGO_ENABLED=0 go build -o /out/worker  ./cmd/worker   \
 && CGO_ENABLED=0 go build -o /out/migrate ./tools/migrate

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/ /
```

- **빌드 인자로 가르지 않는다.** Railway 설정 표면에 Docker 빌드 인자가 없다 (ADR-031).
  **셋을 다 넣고 서비스별 `startCommand`로 고른다.** `ENTRYPOINT`를 박지 않는 이유가 이것이다.

- `CGO_ENABLED=0`. distroless static에 libc가 없다.
- 런타임 이미지에 SQL 파일을 싣지 않는다. 이미 바이너리 안에 있다.
- `.env`를 이미지에 넣지 않는다. Railway 환경변수로 주입한다.

**실측 (2026-08-20, 로컬 `make docker-build`)**

| | |
|---|---|
| Go 이미지 | **57.3MB** — `/api` 12.5MB · `/worker` 15.0MB · `/migrate` 10.4MB |
| 웹 이미지 | **349MB** — 대부분 `node:24-slim` 베이스. `.output` 말고는 아무것도 없다 |
| 빌드 컨텍스트 | 276MB → **1MB 미만** (`.dockerignore` 적용 후) |

ADR-029가 말한 "20~30MB"는 바이너리 하나 기준이다. 셋을 넣어 57MB가 됐다 — 여전히 작다.
셋 다 distroless 안에서 실행되는 것을 확인했다 (`DATABASE_URL` 없이 돌려 설정 오류로 즉시 종료).

**web Dockerfile — 요점**

```dockerfile
FROM node:24 AS build
RUN corepack enable          # packageManager 필드의 pnpm 10.28.1 강제 (ADR-019)
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

ARG VITE_API_URL             # 클라이언트 번들에 박히는 값 (ADR-032)
ARG VITE_LOGIN_URL           # 없으면 여기서 빌드를 세운다
...
RUN pnpm build

FROM node:24-slim
COPY --from=build /app/web/.output ./.output   # devDependencies를 싣지 않는다
ENV HOST=::                                    # 불변식 4
CMD ["node", ".output/server/index.mjs"]
```

- **`corepack enable`을 빼지 말 것.** Nixpacks에 맡기면 pnpm 버전 고정이 보장되지 않는다.
- **`pnpm-workspace.yaml`도 락파일과 함께 복사한다.** `allowBuilds`(esbuild·lightningcss)가
  거기 있다. 빠지면 pnpm 10이 빌드 스크립트를 막아 설치가 반쪽이 된다.
- **`ARG`를 빼지 말 것.** Railway는 이름이 같은 `ARG`를 선언한 스테이지에만 변수를 넘긴다 (§4.5).
- `orval.config.ts`가 `../openapi.yaml`을 참조하므로 **빌드 컨텍스트는 저장소 루트**다.
  `web/`만 컨텍스트로 잡으면 생성이 깨진다. Railway에서는 `source.rootDirectory`를
  **건드리지 않는 것**이 곧 이 조건이다.

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

| 서비스 | 메모리 | `memoryBytes` | 근거 |
|---|---|---|---|
| `worker` | **1GB** | `1073741824` | 최악 310MB + GC 여유. `PARSE_CONCURRENCY`를 올리면 배수로 는다 |
| `api` | 512MB | `536870912` | 요청당 챕터 하나만 읽는다 |
| `web` | 512MB | `536870912` | SSR |

**`PARSE_CONCURRENCY`를 메모리 확인 없이 올리지 말 것.** 2면 최악이 620MB다.

### 4.3 서비스 구성과 설정 파일 (ADR-031)

넷 다 같은 저장소를 본다. **빌드·배포 설정은 서비스마다 `railway.json`에 둔다.**

| 서비스 | 설정 파일 | `dockerfilePath` | `startCommand` | pre-deploy | 메모리 |
|---|---|---|---|---|---|
| `postgres` | — | — | — | — | Railway 관리형 |
| `api` | `railway.api.json` | `Dockerfile` | `/api` | `/migrate` | 512MB |
| `worker` | `railway.worker.json` | `Dockerfile` | `/worker` | — | 1GB |
| `web` | `railway.web.json` | `web/Dockerfile` | (이미지 `CMD`) | — | 512MB |

`railway.api.json`:

```json
{
  "$schema": "https://railway.com/railway.schema.json",
  "build": { "builder": "DOCKERFILE", "dockerfilePath": "Dockerfile" },
  "deploy": {
    "startCommand": "/api",
    "preDeployCommand": ["/migrate"],
    "healthcheckPath": "/healthz",
    "restartPolicyType": "ON_FAILURE",
    "limitOverride": { "containers": { "memoryBytes": 536870912 } }
  }
}
```

**파일이 셋인 이유.** 셋 다 저장소 루트에서 빌드하므로 기본 이름(`railway.json`) 하나를
두면 세 서비스가 같은 값을 읽는다. `startCommand`가 서로 달라야 하니 갈라야 한다.
**서비스마다 설정 파일 경로를 지정해야 한다** — §6.4의 미확인 항목이다.

**시크릿을 이 파일에 넣지 않는다.** 커밋되는 파일이다. 값은 Railway 변수로 간다 (§4.5).

**`api`는 `[::]`에 바인딩한다** (불변식 4). 이미 그렇게 돼 있고, 이 슬라이스가 그것을 처음 검증한다.

### 4.4 마이그레이션은 pre-deploy 명령으로 실행한다 (ADR-030)

`make migrate`는 `tools/migrate`를 `go run`으로 돌린다. Railway에는 그 자리가 없다 —
서비스를 띄우는 것만 하지 "이 명령을 한 번 실행" 하는 자리가 기본적으로 없다.

순서가 틀리면 바로 깨진다. `00003_auth.sql`이 `sessions`를 만드는데,
그 SQL 없이 새 API 코드가 뜨면 로그인마다 `relation "sessions" does not exist`다.
**스키마가 코드보다 먼저 가 있어야 한다.**

**결정: Railway pre-deploy 명령.** 새 버전을 띄우기 전에 한 번 실행된다.
`railway.api.json`의 `deploy.preDeployCommand`가 그 자리다.

**Dockerfile에 미치는 영향:** `tools/migrate`도 바이너리로 빌드해 같은 이미지에 넣는다.
`api`·`worker`·`migrate` 셋이 한 이미지에 들어가는 이유가 이것이다 (§4.1).

**`preDeployCommand`는 명령 하나만 받는다** — 스키마상 문자열이거나 원소 하나짜리 배열이다.
`tools/migrate`가 goose와 River를 이어 도는 구조(ADR-022)가 여기서 그대로 산다.
명령을 둘로 나눴다면 이 자리에 못 넣었다.

pre-deploy는 `api` 서비스 하나에만 건다. `worker`에도 걸면 배포마다 두 번 돈다 —
goose가 멱등이라 사고는 안 나지만 의미 없는 실행이다.

### 4.5 환경변수

**`api`**

| 키 | 값 |
|---|---|
| `PORT` | Railway 자동 주입 |
| `DATABASE_URL` | Postgres 서비스 참조 |
| `CORS_ALLOWED_ORIGINS` | `https://reclassic.dinnertimes.app` (ADR-026) |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | 프로덕션 클라이언트 |
| `GOOGLE_REDIRECT_URL` | `https://api-reclassic.dinnertimes.app/auth/google/callback` |
| `LOGIN_SUCCESS_REDIRECT` | `https://reclassic.dinnertimes.app/` |
| `ADMIN_EMAIL` | 마스터 계정 (ADR-027) |
| `COOKIE_SECURE` | **`true`** |
| `COOKIE_DOMAIN` | `.dinnertimes.app` — 앞의 점이 서브도메인 공유를 만든다. 범위가 존 전체인 이유는 ADR-033 |

**`worker`**

`DATABASE_URL` / `R2_ACCOUNT_ID` / `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_BUCKET` /
`GUTENBERG_USER_AGENT` / `GUTENBERG_MIN_INTERVAL_MS=1000` / `PUBLIC_BASE_URL`

**`S3_ENDPOINT`를 비운다.** 값이 있으면 MinIO 모드(path-style + 버킷 자동 생성)로 간다.

**`web`**

| 키 | 값 | 언제 쓰이는가 |
|---|---|---|
| `VITE_API_URL` | `https://api-reclassic.dinnertimes.app` — 브라우저용 | **빌드 시점** |
| `VITE_LOGIN_URL` | `https://api-reclassic.dinnertimes.app/auth/google/start` | **빌드 시점** |
| `API_INTERNAL_HOST` | `api.railway.internal` — SSR용 | 런타임 |
| `API_PORT` | `8080` | 런타임 |

마지막 둘이 불변식 4의 "API 베이스 URL은 서버·클라이언트에서 서로 달라야 한다"이다.

**`VITE_*` 둘은 런타임 환경변수가 아니다** (ADR-032). Vite가 빌드 시점에 클라이언트 번들에
상수로 박는다. Railway는 **이름이 같은 `ARG`를 선언한 스테이지에만** 서비스 변수를 넘기므로
`web/Dockerfile`이 `ARG VITE_API_URL`·`ARG VITE_LOGIN_URL`을 선언한다.

- **웹 서비스를 처음 배포하기 전에 값이 들어가 있어야 한다.** 순서가 §6.1(도메인)에 걸린다.
- **값이 바뀌면 재시작이 아니라 재빌드다.** 도메인을 갈아끼우면 웹은 다시 빌드해야 한다.
- 비어 있으면 `pnpm build` 앞에서 빌드를 세운다. 그냥 두면 번들에 `undefined`가 박히고
  **SSR은 뜨는데 브라우저에서만 죽는다** — 배포는 성공한 것처럼 보인다.

### 4.6 프로덕션 Google OAuth

로컬 클라이언트와 **별도로** 만들거나 리디렉션 URI를 추가한다.

- 승인된 리디렉션 URI: `https://api-reclassic.dinnertimes.app/auth/google/callback`
- 동의 화면을 테스트에서 **프로덕션으로 게시**해야 테스트 사용자 밖에서도 로그인된다

---

## 5. Railway 설정 — CLI로 하는 것

`railway` CLI 5.41.3 확인 (2026-08-20). 아래는 **명령이 존재하는 것을 확인한 경로**다.
실행해서 결과를 본 것은 `whoami`·`list`·`status`뿐이다.

```bash
railway whoami          # 로그인 확인
railway status --json   # 링크된 프로젝트 확인
```

### 5.1 프로젝트와 서비스

```bash
railway init --name reclassic
railway add --database postgres --json
railway add --service api    --repo dinnertime-who/reclassic --branch main --json
railway add --service worker --repo dinnertime-who/reclassic --branch main --json
railway add --service web    --repo dinnertime-who/reclassic --branch main --json
railway service list --json     # 다시 시도하기 전에 반드시 확인. add는 멱등이 아니다
```

### 5.2 환경변수

```bash
railway variable set COOKIE_SECURE=true --service api --skip-deploys
printf '%s' "$SECRET" | railway variable set GOOGLE_CLIENT_SECRET --stdin --service api --skip-deploys
railway variable list --service api --json
```

- **시크릿은 `--stdin`으로 넣는다.** 인자로 주면 셸 히스토리에 남는다.
- **`--skip-deploys`를 붙인다.** 변수를 하나 넣을 때마다 배포가 돌면 낭비다.
  다 넣고 나서 한 번 배포한다.
- `variable list --json`과 `--kv`는 **값을 그대로 출력한다.** 출력을 그대로 붙여넣지 말 것.

### 5.3 도메인

```bash
railway domain api-reclassic.dinnertimes.app --service api --port 8080
railway domain status --service api --json
railway domain list --service api --json
```

**`railway domain`은 등록하면서 넣어야 할 DNS 레코드를 함께 출력한다.**
그 값을 §6.1의 3단계에 쓴다. 대시보드를 열 필요가 없다.

### 5.4 설정 파일이 못 담는 것

`railway.json`에 없는 서비스 설정은 점 경로로 넣는다.

```bash
railway environment edit --service-config api build.dockerfilePath "Dockerfile"
railway environment edit --service-config web build.dockerfilePath "web/Dockerfile"
```

**`source.rootDirectory`는 건드리지 않는다.** 셋 다 저장소 루트에서 빌드해야 한다 (§4.1).

### 5.5 배포와 확인

```bash
railway logs --service api --lines 200
railway deployment list --json
railway metrics --service worker --since 1h --json
```

**`deployment list`가 `SUCCESS`를 줄 때까지 배포는 성공이 아니다.**
빌드가 큐에 들어간 것과 뜬 것은 다르다. `FAILED`·`CRASHED`면 `railway logs`로 간다.

---

## 6. 사람이 직접 해야 하는 것 — 체크리스트

CLI로 안 되는 것만 남겼다. **순서대로 하면 뒤 항목의 값이 앞에서 정해진다.**

### 6.1 도메인 (가장 먼저)

**발급만으로는 안 된다. DNS 레코드를 직접 넣어야 한다.**

**같은 상위 도메인을 써야 세션 쿠키가 공유된다** (ADR-027의 `COOKIE_DOMAIN`).
웹을 다른 호스팅(예: Vercel)에 두면 쿠키가 갈려 로그인이 되지 않는다.

**정해진 이름 (2026-08-20, ADR-033)**

```
reclassic.dinnertimes.app        웹
api-reclassic.dinnertimes.app    API      ← 점이 아니라 하이픈. 이유는 ADR-033
COOKIE_DOMAIN=.dinnertimes.app            ← 둘의 공통 상위가 여기까지다
```

DNS는 Cloudflare다 (`beth`·`rick.ns.cloudflare.com`). apex를 쓰지 않으므로
**apex CNAME 제약에는 걸리지 않는다.**

**절차**

1. 도메인 발급 ← **사람** — **끝났다**
2. `railway domain <이름> --service <서비스>` → **넣어야 할 DNS 레코드를 받는다.**
   서비스마다 값이 다르다 ← **CLI (§5.3)**
3. DNS에 CNAME 추가 ← **사람**

   ```
   api-reclassic   CNAME → <api 서비스가 준 값>   Proxy: DNS only(회색)
   reclassic       CNAME → <web 서비스가 준 값>   Proxy: DNS only(회색)
   ```

4. TLS는 Railway가 자동 발급한다. `railway domain status`로 확인한다

**걸리는 곳 둘**

- **이 존에는 이미 프록시가 켜진 와일드카드가 있다** (2026-08-20 확인).
  없는 이름(`definitely-not-real-xyz.dinnertimes.app`)까지 Cloudflare 프록시 IP로 응답하고
  `https://reclassic.dinnertimes.app/`이 `server: cloudflare`로 404를 준다.
  명시적 레코드가 와일드카드를 이기므로 동작에는 문제가 없지만,
  **Cloudflare는 새 CNAME을 기본으로 프록시 켜서 만든다.** 3단계에서 반드시 회색으로 둘 것.
- **Cloudflare 프록시(주황 구름)를 처음부터 켜지 말 것.**
  DNS only(회색)로 두고 Railway 인증서가 발급된 것을 확인한 뒤,
  켤 거면 SSL/TLS 모드를 **Full (strict)**로 맞추고 켠다.
  순서가 반대면 이중 프록시로 발급이 막힌다.
  `.app`은 HSTS 프리로드 TLD라 http 폴백이 없다 — 인증서가 안 서면 화면이 아예 안 뜬다.

이게 정해지면 아래 값이 전부 확정된다:
`CORS_ALLOWED_ORIGINS` · `COOKIE_DOMAIN` · `GOOGLE_REDIRECT_URL` ·
`LOGIN_SUCCESS_REDIRECT` · `VITE_API_URL` · `VITE_LOGIN_URL` · `PUBLIC_BASE_URL`

### 6.2 Cloudflare R2

- 버킷 생성 (예: `reclassic-sources`)
- R2 API 토큰 발급 → `R2_ACCOUNT_ID` · `R2_ACCESS_KEY_ID` · `R2_SECRET_ACCESS_KEY`

현재 `.env` 값은 **로컬 MinIO 자격증명**이다. 그대로 쓰면 안 된다.

### 6.3 Google OAuth — 프로덕션

로컬 클라이언트에 URI를 추가하거나 별도 클라이언트를 만든다.

- 승인된 리디렉션 URI에 `https://api-reclassic.dinnertimes.app/auth/google/callback` 추가
- **OAuth 동의 화면을 테스트 → 프로덕션으로 게시.** 안 하면 테스트 사용자만 로그인된다

### 6.4 Railway 대시보드 — 미확인 둘 (ADR-031)

CLI로 되는지 확인하지 못했다. **안 되면 대시보드에서 서비스당 한 번씩** 하면 된다.

| 항목 | 상태 |
|---|---|
| **서비스별 설정 파일 경로** (`railway.api.json` 등) | **문서 확인됨.** 서비스 Settings의 config file path 필드에 **저장소 절대 경로**(`/railway.api.json`)를 넣는다. CLI로 되는지는 미확인 |
| **메모리 상한** (§4.2) | **여전히 미확인.** 스키마에 `deploy.limitOverride.containers.memoryBytes`가 실재하는 것은 재확인했지만, `config-as-code/reference`의 설정 목록에는 없다. 파일에 적어 두고 **먹는지는 배포 후 확인** |

**확인되면 ADR-031의 "미확인 둘"에 결과를 적는다.** 지우지 말고 채운다.
문서로 확인된 부분은 이미 적어 뒀다 — **실물 확인 결과를 그 아래 이어서 적는다.**

### 6.5 배포 후 확인

§8 완료 조건을 위에서부터 훑는다. 특히 **SSR이 `api.railway.internal`을 쓰는지**와
**서브도메인 간 쿠키 공유**가 이 슬라이스의 핵심 검증이다.

---

## 7. 반드시 지킬 것

1. **`[::]`에 바인딩한다.** `0.0.0.0`은 서비스 간 내부 호출을 받지 못한다.
2. **`COOKIE_SECURE=true`.** 로컬에서만 false다.
3. **`S3_ENDPOINT`를 비운다.** 프로덕션에서 MinIO 모드로 가면 안 된다.
4. **`GUTENBERG_MIN_INTERVAL_MS`를 1000 미만으로 낮추지 않는다.**
5. **`PARSE_CONCURRENCY`를 메모리 확인 없이 올리지 않는다.** §4.2.
6. **시크릿을 이미지에도 `railway.json`에도 굽지 않는다.**
   `.env`는 `.dockerignore` 대상이고, `railway.json`은 커밋된다.
7. **`source.rootDirectory`를 설정하지 않는다.** `web` 빌드가 저장소 루트를 봐야 한다 (§4.1).
8. **`.railway/railway.ts`를 같이 두지 않는다.** 두 모델은 같은 서비스를 함께 관리할 수 없다 (ADR-031).
9. 작업 종료 전 `make lint && make test` 통과.

---

## 8. 완료 조건

- [x] `docker build .` 한 번으로 `api`·`worker`·`migrate` 셋이 든 이미지가 나온다 — 57.3MB
- [x] `web/Dockerfile`이 pnpm 10.28.1로 빌드하고 런타임 이미지에 devDependencies가 없다 —
      corepack이 `pnpm-10.28.1.tgz`를 받는 것과 런타임에 `node_modules`가 없는 것을 확인
- [x] **`VITE_*` 없이 웹을 빌드하면 실패한다** — 깨진 번들이 배포되지 않는다 (ADR-032).
      인자 없이 빌드하면 `pnpm build` 전에 멈춘다

**로컬 선행 검증 (2026-08-20).** Railway가 아니라 컨테이너 둘을 같은 도커 네트워크에 띄워 확인했다.
불변식 4의 구조를 로컬에서 미리 밟아 본 것이고, **Railway 실물 확인을 대신하지 않는다.**

- `/api`가 distroless에서 뜨고 `[::]:8080`에 바인딩한다 → `/healthz`가 `db: ok`
- **SSR이 컨테이너 이름(`API_INTERNAL_HOST`)으로 API를 부른다** — api 컨테이너 로그에
  `/healthz`·`/auth/me` 요청이 찍혔다. 공개 URL을 경유하지 않는다
- 같은 HTML의 로그인 링크는 **빌드 시점에 박힌 `VITE_LOGIN_URL`**이다 (ADR-032).
  서버는 내부 주소, 브라우저는 공개 주소 — 불변식 4가 실제로 갈린다
- [ ] 4서비스가 Railway에 뜨고 `/healthz`가 `db: ok`를 준다
- [ ] **서비스별 `railway.json`이 실제로 읽힌다** — 배포 상세에서 설정 출처가 파일로 표시된다 (ADR-031)
- [ ] **SSR이 `api.railway.internal`로 API를 부른다** — 공개 도메인을 경유하지 않는다 (불변식 4)
- [ ] 프로덕션 도메인에서 Google 로그인이 되고, `ADMIN_EMAIL` 계정이 `admin`이 된다
- [ ] **웹과 API가 다른 서브도메인인데 세션 쿠키가 공유된다** (`COOKIE_DOMAIN`)
- [ ] 관리자가 도서 한 권을 지시하면 수집·파싱·적재가 끝까지 돈다
- [ ] 읽기 화면이 프로덕션에서 뜨고 `noindex`가 붙어 있다
- [ ] **pre-deploy 명령으로 마이그레이션이 배포당 한 번만 실행된다** (ADR-030)
- [ ] 마이그레이션이 실패하면 새 버전이 뜨지 않는다
- [ ] **§6.4 미확인 둘의 결과가 ADR-031에 적혔다**
- [ ] `make lint && make test` 통과

---

## 9. 다음

이 슬라이스가 끝나면 **ADR-028의 숙제에 답할 수 있다** — 운영이 실제로 아픈가.
아프면 안 C(Workers + Hyperdrive + Postgres)를 다시 연다. 아니면 화면 슬라이스로 간다.

`.railway/railway.ts`(TypeScript IaC)로 옮길지도 여기서 판단한다 —
`dockerfilePath`·`preDeployCommand`·메모리를 DSL이 담게 되면 옮길 이유가 생긴다 (ADR-031).

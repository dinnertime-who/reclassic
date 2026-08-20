# Go 서비스 이미지. api·worker·migrate 셋이 한 이미지에 들어간다 (ADR-031).
#
# 왜 셋을 합치는가: Railway의 railway.json 스키마에 Docker 빌드 인자(build.args)가 없다.
# 빌드 시점에 어느 바이너리를 낼지 가를 방법이 없으니, 다 넣고 서비스별
# deploy.startCommand로 고른다. 그래서 ENTRYPOINT를 박지 않는다.
#
#   api      → /api
#   worker   → /worker
#   migrate  → /migrate   (api 서비스의 preDeployCommand, ADR-030)

FROM golang:1.26 AS build
WORKDIR /src

# 의존성을 먼저 굳힌다. 소스만 바뀐 배포에서는 이 레이어가 캐시된다.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Railway는 서비스 변수를 빌드에 넘길 때 이름이 같은 ARG를 요구한다.
# 선언하지 않으면 조용히 비어 있다. /healthz의 version에 배포된 커밋이 뜬다.
ARG RAILWAY_GIT_COMMIT_SHA

# CGO_ENABLED=0 — distroless static에는 libc가 없다.
# 마이그레이션 SQL은 internal/db/migrate.go의 go:embed로 이미 바이너리 안에 있다.
# 런타임 이미지에 .sql을 따로 싣지 않는 이유가 이것이다.
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w -X main.version=${RAILWAY_GIT_COMMIT_SHA:-docker}" \
      -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w" \
      -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w" \
      -o /out/migrate ./tools/migrate

# static-debian12는 ca-certificates를 포함한다.
# Gutenberg·R2·Google 모두 HTTPS라 이게 없으면 x509 오류로 죽는다.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /

# ENTRYPOINT·CMD를 박지 않는다. 서비스별 startCommand가 고른다 (위 주석).

# 웹(TanStack Start) 이미지.
#
# 빌드 컨텍스트는 저장소 루트다 (ADR-029 · SLICE_DEPLOY §7.7).
# 빌드가 orval을 돌리지는 않지만(생성 코드는 커밋된다) orval.config.ts가 ../openapi.yaml을
# 보므로, web/만 컨텍스트로 잡으면 그 경로가 컨텍스트 밖이 된다.
# Railway에서는 source.rootDirectory를 건드리지 않는 것이 곧 이 조건이다.

FROM node:24 AS build
WORKDIR /app/web

# pnpm 버전은 web/package.json의 packageManager가 고정한다 (ADR-019).
# corepack을 빼면 이미지에 딸려온 아무 pnpm이나 쓰게 된다.
RUN corepack enable

# 락파일 레이어를 먼저 굳힌다. 소스만 바뀐 배포에서는 install이 캐시된다.
# pnpm-workspace.yaml에 allowBuilds(esbuild·lightningcss)가 있어 함께 와야 한다.
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

# VITE_ 접두 값은 런타임 환경변수가 아니라 클라이언트 번들에 박히는 상수다.
# Railway는 이름이 같은 ARG를 선언한 스테이지에만 서비스 변수를 넘긴다 (ADR-032).
ARG VITE_API_URL
ARG VITE_LOGIN_URL

# 없으면 여기서 멈춘다. 그냥 두면 브라우저에서 undefined가 되고,
# 배포는 성공한 것처럼 보이는데 화면만 죽는다 (CONVENTIONS "기본값을 조용히 채우지 않는다").
RUN test -n "$VITE_API_URL"   || { echo "✗ VITE_API_URL 이 비어 있다 — Railway web 서비스 변수를 확인할 것"; exit 1; } \
 && test -n "$VITE_LOGIN_URL" || { echo "✗ VITE_LOGIN_URL 이 비어 있다 — Railway web 서비스 변수를 확인할 것"; exit 1; }

COPY openapi.yaml /app/openapi.yaml
COPY web/ ./

RUN pnpm build

FROM node:24-slim
WORKDIR /app

# .output만 옮긴다. nitro가 의존성을 번들에 넣으므로 devDependencies가 따라오지 않는다.
COPY --from=build /app/web/.output ./.output

ENV NODE_ENV=production
# Railway 프라이빗 네트워크는 IPv6 전용이다 (불변식 4).
# node는 ::에 바인딩하면 IPv4도 함께 받는다.
ENV HOST=::

CMD ["node", ".output/server/index.mjs"]

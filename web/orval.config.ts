// openapi.yaml → TS 클라이언트 (ADR-009).
// 진입점은 `make generate`다. `pnpm generate-api`를 직접 치지 않는다 (AGENTS.md).
// 산출물(src/api/gen)은 손으로 고치지 않는다.
import { defineConfig } from 'orval'

export default defineConfig({
  reclassic: {
    input: {
      target: '../openapi.yaml',
    },
    output: {
      target: './src/api/gen/reclassic.ts',
      schemas: './src/api/gen/model',
      client: 'fetch',
      httpClient: 'fetch',
      clean: true,
      override: {
        // 베이스 URL 분기와 쿠키 전달은 mutator 한 곳에서만 한다.
        mutator: {
          path: './src/api/http.ts',
          name: 'apiFetch',
        },
      },
    },
  },
})

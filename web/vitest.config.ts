import { defineConfig } from 'vitest/config'
import viteReact from '@vitejs/plugin-react'

// vite.config.ts를 공유하지 않는다 — 거기에는 nitro와 tanstackStart가 들어 있어
// 테스트가 SSR 서버 빌드를 함께 끌고 온다. 테스트는 API도 DB도 없이 돌아야 한다 (ADR-035).
//
// 별칭 `#/*`는 vite.config.ts와 같은 방식(tsconfig paths)으로 푼다.
// 여기서 어긋나면 테스트만 import를 못 찾는다.
export default defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [viteReact()],
  test: {
    environment: 'jsdom',
    globals: false,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
  },
})

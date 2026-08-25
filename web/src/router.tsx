import { QueryClient } from '@tanstack/react-query'
import { createRouter as createTanStackRouter } from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'

import { routeTree } from './routeTree.gen'

export function getRouter() {
  // 요청마다 새로 만든다 (ADR-035). 모듈 스코프에 두면 서버 프로세스 하나가
  // 모든 요청을 처리하므로 한 사람의 응답이 다른 사람에게 나간다.
  const queryClient = new QueryClient({
    defaultOptions: {
      // 0이면 하이드레이션 직후 서버가 이미 받아온 것을 전부 다시 요청한다.
      // 아래 defaultPreloadStaleTime(0)과는 다른 값이다 — 혼동하지 말 것.
      queries: { staleTime: 60_000 },
    },
  })

  const router = createTanStackRouter({
    routeTree,
    context: { queryClient },
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
  })

  // dehydrate/hydrate와 QueryClientProvider를 이 통합이 붙인다.
  // wrapQueryClient가 기본 true라 QueryClientProvider로 직접 감싸지 않는다 (ADR-035).
  setupRouterSsrQueryIntegration({ router, queryClient })

  return router
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}

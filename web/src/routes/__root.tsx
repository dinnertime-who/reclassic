import type { QueryClient } from '@tanstack/react-query'
import {
  HeadContent,
  Link,
  Scripts,
  createRootRouteWithContext,
  useRouter,
} from '@tanstack/react-router'

import { getCurrentUser, logout } from '#/api/gen/reclassic'
// 부수효과로 import한다. `?url`로 받아 links에 직접 넣으면 **SSR 빌드의 해시**가
// 박히는데, 그 파일은 .output/public 에 발행되지 않아 404가 된다 (ADR-042).
// 클라이언트·SSR 두 빌드가 만드는 CSS가 바이트 단위로 같을 때만 우연히 맞는다.
import '../styles.css'

// 라우터 컨텍스트의 타입을 여기서 연다. getRouter()가 요청마다 만든 QueryClient를
// 넣어 주고, 편집·검수 라우트가 로더에서 ensureQueryData로 받아 쓴다 (ADR-035).
// 이 파일은 읽기 화면이므로 훅은 쓰지 않는다 — 타입만 연다.
export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  // 비로그인도 읽기 화면은 볼 수 있어야 한다. 실패를 에러로 올리지 않는다.
  loader: async () => {
    try {
      const res = await getCurrentUser()
      return res.status === 200 ? res.data : null
    } catch {
      return null
    }
  },
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      { title: 'reclassic' },
    ],
  }),
  shellComponent: RootDocument,
})

function RootDocument({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ko">
      <head>
        <HeadContent />
      </head>
      <body>
        <SiteHeader />
        {children}
        <Scripts />
      </body>
    </html>
  )
}

function SiteHeader() {
  const user = Route.useLoaderData()
  const router = useRouter()

  // 브라우저가 직접 이동해야 하는 주소다. orval 클라이언트로 부를 수 없다 —
  // 302로 Google 동의 화면에 가야 하기 때문이다.
  const loginUrl = import.meta.env.VITE_LOGIN_URL

  async function onLogout() {
    await logout()
    await router.invalidate()
  }

  // 모바일에서 한 줄에 들어가야 한다. 이름은 좁은 폭에서 CSS가 숨기고
  // 로그아웃은 남는다 (ADR-038) — 지금 누구인지보다 나가는 길이 급하다.
  return (
    <header className="site-header">
      <Link to="/" className="site-brand">
        reclassic
      </Link>
      <nav className="site-nav">
        <Link to="/books">도서 목록</Link>
        {user?.role === 'admin' && <Link to="/admin">관리</Link>}
      </nav>
      <div className="site-session">
        {user ? (
          <>
            <span className="site-user">
              {user.displayName} · {user.role}
            </span>
            <button type="button" className="btn" onClick={onLogout}>
              로그아웃
            </button>
          </>
        ) : (
          <a className="btn" href={loginUrl}>
            Google로 로그인
          </a>
        )}
      </div>
    </header>
  )
}

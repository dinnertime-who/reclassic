import {
  HeadContent,
  Scripts,
  createRootRoute,
  useRouter,
} from '@tanstack/react-router'

import { getCurrentUser, logout } from '#/api/gen/reclassic'
import appCss from '../styles.css?url'

export const Route = createRootRoute({
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
    links: [{ rel: 'stylesheet', href: appCss }],
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
        <SessionBar />
        {children}
        <Scripts />
      </body>
    </html>
  )
}

function SessionBar() {
  const user = Route.useLoaderData()
  const router = useRouter()

  // 브라우저가 직접 이동해야 하는 주소다. orval 클라이언트로 부를 수 없다 —
  // 302로 Google 동의 화면에 가야 하기 때문이다.
  const loginUrl = import.meta.env.VITE_LOGIN_URL

  async function onLogout() {
    await logout()
    await router.invalidate()
  }

  return (
    <nav className="session">
      {user ? (
        <>
          <span>
            {user.displayName} · {user.role}
          </span>{' '}
          <button type="button" onClick={onLogout}>
            로그아웃
          </button>
        </>
      ) : (
        <a href={loginUrl}>Google로 로그인</a>
      )}
    </nav>
  )
}

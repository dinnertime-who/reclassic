import { Link, createFileRoute } from '@tanstack/react-router'

import { getHealthz } from '#/api/gen/reclassic'

// 로더는 최초 진입 시 서버에서 돈다. 그래서 자바스크립트 없이도 값이 HTML에 들어간다.
// 화면 디자인은 이번 슬라이스 범위가 아니다 — 값이 보이면 된다.
export const Route = createFileRoute('/')({
  loader: async () => (await getHealthz()).data,
  component: Skeleton,
})

function Skeleton() {
  const health = Route.useLoaderData()

  return (
    <main>
      <h1>reclassic</h1>
      <p>
        <Link to="/books">도서 목록</Link>
      </p>
      <p>골격 슬라이스. Go API의 /healthz를 SSR로 불러 표시한다.</p>
      <dl>
        <dt>status</dt>
        <dd data-testid="status">{health.status}</dd>
        <dt>db</dt>
        <dd data-testid="db">{health.db}</dd>
        <dt>version</dt>
        <dd data-testid="version">{health.version}</dd>
      </dl>
    </main>
  )
}

import { Link, createFileRoute, notFound } from '@tanstack/react-router'

import { getBookChapter } from '#/api/gen/reclassic'
import { ApiError } from '#/api/http'

export const Route = createFileRoute('/books/$gutenbergId/chapters/$idx')({
  loader: async ({ params }) => {
    const gutenbergId = Number(params.gutenbergId)
    const idx = Number(params.idx)
    if (!Number.isInteger(gutenbergId) || !Number.isInteger(idx) || idx < 0) {
      throw notFound()
    }
    try {
      const res = await getBookChapter(gutenbergId, idx)
      // 계약이 200|404 유니온이라 좁혀야 한다. mutator가 404에서 이미 던지므로
      // 여기까지 오지 않지만, 계약이 바뀌면 타입이 먼저 알려준다.
      if (res.status !== 200) throw notFound()
      return res.data
    } catch (err) {
      // 404만 notFound로 바꾼다. 500을 404로 삼키면 장애가 빈 페이지로 보인다.
      if (err instanceof ApiError && err.status === 404) throw notFound()
      throw err
    }
  },
  head: ({ loaderData }) => ({
    meta: [
      // 원문 전용 페이지다. Gutenberg 원문은 이미 수백 개 사이트에 있어
      // 중복 경쟁에서 이길 수 없고 크롤 예산만 소모한다 (ADR-007).
      // 색인 대상이 되는 것은 번역 페이지이고 그건 번역 슬라이스다.
      { name: 'robots', content: 'noindex, follow' },
      { title: loaderData?.chapter.title || 'reclassic' },
    ],
  }),
  notFoundComponent: () => (
    <main>
      <p>읽을 수 있는 챕터가 없습니다.</p>
    </main>
  ),
  component: ChapterPage,
})

function ChapterPage() {
  const { gutenbergId, idx } = Route.useParams()
  const { chapter, paragraphs, totalChapters } = Route.useLoaderData()

  const current = Number(idx)
  const to = '/books/$gutenbergId/chapters/$idx'

  return (
    <main>
      <p>
        {gutenbergId} — {current + 1} / {totalChapters}
      </p>
      <h1>{chapter.title || `(제목 없음) ${chapter.idx}`}</h1>

      {paragraphs.map((p) => (
        // key는 stable_id다. 번역이 붙는 키와 같다 (ADR-004/016).
        <p key={p.stableId}>{p.sourceText}</p>
      ))}

      <nav>
        {current > 0 && (
          <Link to={to} params={{ gutenbergId, idx: String(current - 1) }}>
            ← 이전
          </Link>
        )}{' '}
        {current + 1 < totalChapters && (
          <Link to={to} params={{ gutenbergId, idx: String(current + 1) }}>
            다음 →
          </Link>
        )}
      </nav>
    </main>
  )
}

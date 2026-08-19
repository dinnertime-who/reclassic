import { Link, createFileRoute, notFound } from '@tanstack/react-router'

import { getProjectChapter } from '#/api/gen/reclassic'
import { ApiError } from '#/api/http'

export const Route = createFileRoute('/projects/$projectId/chapters/$idx')({
  loader: async ({ params }) => {
    const projectId = Number(params.projectId)
    const idx = Number(params.idx)
    if (!Number.isInteger(projectId) || !Number.isInteger(idx) || idx < 0) {
      throw notFound()
    }
    try {
      const res = await getProjectChapter(projectId, idx)
      if (res.status !== 200) throw notFound()
      return res.data
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) throw notFound()
      throw err
    }
  },
  head: ({ loaderData }) => ({
    meta: [
      {
        name: 'robots',
        // 색인 여부는 서버가 커버리지로 판정한다 (ADR-007 + ADR-023).
        // 화면이 임계값을 다시 계산하지 않는다 — 두 곳에서 계산하면 갈라진다.
        content: loaderData?.indexable ? 'index, follow' : 'noindex, follow',
      },
      { title: loaderData?.chapter.title || 'reclassic' },
    ],
  }),
  notFoundComponent: () => (
    <main>
      <p>읽을 수 있는 챕터가 없습니다.</p>
    </main>
  ),
  component: TranslatedChapter,
})

function TranslatedChapter() {
  const { projectId, idx } = Route.useParams()
  const { chapter, paragraphs, totalChapters, coverage, indexable } =
    Route.useLoaderData()

  const current = Number(idx)
  const to = '/projects/$projectId/chapters/$idx'
  const percent = Math.round(coverage.ratio * 100)

  return (
    <main>
      <p>
        {current + 1} / {totalChapters} · 번역 {coverage.approved}/{coverage.total} (
        {percent}%) · {indexable ? '색인 대상' : '색인 제외'}
      </p>
      <h1>{chapter.title || `(제목 없음) ${chapter.idx}`}</h1>

      {paragraphs.map((p) => (
        // 확정 번역이 없으면 원문을 보여준다. 부분 공개를 허용한다 —
        // 100%를 기다리면 아무 책도 공개하지 못한다.
        <p key={p.stableId} lang={p.approvedTranslation ? 'ko' : 'en'}>
          {p.approvedTranslation ?? p.sourceText}
          {!p.approvedTranslation && p.proposalCount > 0 && (
            <small> (제안 {p.proposalCount}건)</small>
          )}
        </p>
      ))}

      <nav>
        {current > 0 && (
          <Link to={to} params={{ projectId, idx: String(current - 1) }}>
            ← 이전
          </Link>
        )}{' '}
        {current + 1 < totalChapters && (
          <Link to={to} params={{ projectId, idx: String(current + 1) }}>
            다음 →
          </Link>
        )}
      </nav>
    </main>
  )
}

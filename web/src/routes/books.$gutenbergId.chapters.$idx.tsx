import { Link, createFileRoute, notFound } from '@tanstack/react-router'
import type { CSSProperties } from 'react'

import { getBookChapter } from '#/api/gen/reclassic'
import { ApiError } from '#/api/http'
import { ReaderSettings } from '#/components/reader-settings'
import { chapterProgress, readingMinutes } from '#/lib/reading'

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
    <main className="reader">
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
  const progress = chapterProgress(current, totalChapters)
  const minutes = readingMinutes(paragraphs.map((p) => p.sourceText))

  // 상단은 정보만, 하단은 조작만이다 (ADR-038). 리듬과 색은 전부 styles.css에 있다.
  return (
    <>
      <div className="reader-top">
        <span>원문</span>
        <span>
          <span className="visually-hidden">읽는 위치 </span>
          {progress}%
        </span>
      </div>

      <main className="reader">
        <p className="reader-eyebrow">제 {chapter.idx + 1} 장</p>
        <h1 className="reader-title">
          {chapter.title || `(제목 없음) ${chapter.idx}`}
        </h1>
        <div className="reader-rule" />

        <div className="reader-body">
          {paragraphs.map((p) => (
            // key는 stable_id다. 번역이 붙는 키와 같다 (ADR-004/016).
            // lang은 원문 언어다 — 문서는 ko인데 이 화면의 본문은 전부 영문이다.
            <p key={p.stableId} lang="en">
              {p.sourceText}
            </p>
          ))}
        </div>
      </main>

      <div className="reader-foot">
        <div
          className="reader-progress"
          role="progressbar"
          aria-label="읽은 분량"
          aria-valuenow={progress}
          aria-valuemin={0}
          aria-valuemax={100}
        >
          {/* 퍼센트는 데이터라 여기서 넘기고, 단위는 CSS가 붙인다 (ADR-038). */}
          <span style={{ '--reader-progress': progress } as CSSProperties} />
        </div>

        {/* 위치 표시 옆에 설정 하나. **장 이동 격자(`.reader-nav`)는 그대로 둔다** —
            거기는 장 전환 작업이 다시 짜는 자리다. */}
        <div className="reader-tools">
          <p className="reader-foot-meta">
            {current + 1} / {totalChapters}장 · 이 장 {minutes}분
          </p>
          <ReaderSettings />
        </div>

        <nav className="reader-nav">
          {current > 0 && (
            <Link
              to={to}
              params={{ gutenbergId, idx: String(current - 1) }}
              className="btn btn-prev"
            >
              <span className="visually-hidden">이전 장</span>
              <span aria-hidden="true">‹</span>
            </Link>
          )}
          {current + 1 < totalChapters && (
            <Link
              to={to}
              params={{ gutenbergId, idx: String(current + 1) }}
              className="btn btn-primary"
            >
              다음 장 ›
            </Link>
          )}
        </nav>
      </div>
    </>
  )
}

import { Link, createFileRoute, notFound } from '@tanstack/react-router'
import type { CSSProperties } from 'react'

import { getProjectChapter } from '#/api/gen/reclassic'
import { ApiError } from '#/api/http'
import { ReaderSettings } from '#/components/reader-settings'
import { chapterProgress, readingMinutes } from '#/lib/reading'

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
    <main className="reader">
      <p>읽을 수 있는 챕터가 없습니다.</p>
    </main>
  ),
  component: TranslatedChapter,
})

function TranslatedChapter() {
  const { projectId, idx } = Route.useParams()
  // indexable은 head()의 robots 메타가 쓴다. 화면은 읽지 않는다 (ADR-038).
  const { chapter, paragraphs, totalChapters, coverage } = Route.useLoaderData()

  const current = Number(idx)
  const to = '/projects/$projectId/chapters/$idx'
  const progress = chapterProgress(current, totalChapters)
  // 읽는 사람이 실제로 보는 글로 잰다 — 번역이 있으면 번역, 없으면 원문이다.
  const minutes = readingMinutes(
    paragraphs.map((p) => p.approvedTranslation ?? p.sourceText),
  )

  // 상단은 정보만, 하단은 조작만이다 (ADR-038).
  // 색인 대상 여부는 서버의 판단이고 읽는 사람이 쓸 정보가 아니라 화면에서 내린다.
  return (
    <>
      <div className="reader-top">
        <span>
          번역 {coverage.approved}/{coverage.total}
        </span>
        <span>
          <span className="visually-hidden">읽는 위치 </span>
          {progress}%
        </span>
      </div>

      {/* data-vt는 장 전환이 읽는 홀짝이다 (ADR-041). 이웃한 장이 서로 다른
          `view-transition-name`을 갖게 해서 옛 본문과 새 본문이 한 그룹으로 묶이지
          않게 한다 — 묶이면 스크롤한 채 장을 넘겼을 때 본문이 화면 밖에서 날아온다.
          값을 읽어 이름으로 바꾸는 것은 styles.css다 (ADR-038). */}
      <main className="reader" data-vt={current % 2}>
        <p className="reader-eyebrow">제 {chapter.idx + 1} 장</p>
        <h1 className="reader-title">
          {chapter.title || `(제목 없음) ${chapter.idx}`}
        </h1>
        <div className="reader-rule" />

        <div className="reader-body">
          {paragraphs.map((p) =>
            // 확정 번역이 없으면 원문을 보여준다. 부분 공개를 허용한다 —
            // 100%를 기다리면 아무 책도 공개하지 못한다.
            p.approvedTranslation ? (
              <p key={p.stableId} lang="ko">
                {p.approvedTranslation}
              </p>
            ) : (
              // 색만 흐리게 두면 "흐린 번역"으로 읽힌다. 괘선과 꼬리표로 다른 글이라고 말한다.
              // lang="en"은 그대로 둔다 — 스크린 리더가 영어로 읽어야 한다.
              <div key={p.stableId} className="reader-source">
                <p className="reader-source-label">
                  원문 · 번역 없음
                  {p.proposalCount > 0 && ` · 제안 ${p.proposalCount}건`}
                </p>
                <p className="reader-source-text" lang="en">
                  {p.sourceText}
                </p>
              </div>
            ),
          )}
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

        {/* 목차 · 이전 · 다음. 목차로 가는 입구가 여기 하나 생기면서 63장을
            이전·다음으로만 넘던 문제가 닫힌다 (tech-debt S4).
            **원문 읽기 화면에는 이 링크가 없다** — 원문 쪽 목차 엔드포인트가 아직
            없어서 갈 곳이 없다 (tech-debt D8).

            viewTransition의 types가 장 이동의 방향이다 (ADR-041). 목차로 나가는 것은
            장 이동이 아니라 화면을 뜨는 것이라 붙이지 않는다. */}
        <nav className="reader-nav">
          {/* activeOptions.exact이 없으면 목차 경로가 이 화면 경로의 앞부분이라
              TanStack이 "지금 이 페이지"로 보고 `aria-current="page"`를 붙인다 —
              스크린 리더에게 거짓말이 된다. */}
          <Link
            to="/projects/$projectId/chapters"
            params={{ projectId }}
            className="btn"
            activeOptions={{ exact: true }}
          >
            목차
          </Link>
          {current > 0 && (
            <Link
              to={to}
              params={{ projectId, idx: String(current - 1) }}
              className="btn btn-prev"
              viewTransition={{ types: ['chapter-prev'] }}
            >
              <span className="visually-hidden">이전 장</span>
              <span aria-hidden="true">‹</span>
            </Link>
          )}
          {current + 1 < totalChapters && (
            <Link
              to={to}
              params={{ projectId, idx: String(current + 1) }}
              className="btn btn-primary btn-next"
              viewTransition={{ types: ['chapter-next'] }}
            >
              다음 장 ›
            </Link>
          )}
        </nav>
      </div>
    </>
  )
}

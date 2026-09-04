import type { CSSProperties } from 'react'
import { Link, createFileRoute, notFound } from '@tanstack/react-router'

import { listProjectChapters } from '#/api/gen/reclassic'
import { ApiError } from '#/api/http'

// `/projects/$projectId/chapters` 목차다. 파일 이름이
// `projects.$projectId.chapters.tsx`가 **아닌** 이유 — TanStack Router의 flat
// 라우팅은 그 이름을 `/projects/$projectId/chapters/$idx`의 부모 레이아웃으로
// 승격시킨다. 이 화면은 `<Outlet/>`을 렌더하지 않으므로 그대로 두면 읽기 화면이
// 아예 안 뜬다. 인덱스 파일이면 읽기 라우트와 형제가 되고 URL은 그대로다 —
// `books.index.tsx`와 같은 처방이고, 편집 라우트의 `$idx_`도 같은 함정이다.
//
// 읽기 화면이다. 로더만 쓰고 react-query 훅도 shadcn도 넣지 않는다 (ADR-035·038).
export const Route = createFileRoute('/projects/$projectId/chapters/')({
  loader: async ({ params }) => {
    const projectId = Number(params.projectId)
    if (!Number.isInteger(projectId)) throw notFound()
    try {
      const res = await listProjectChapters(projectId)
      if (res.status !== 200) throw notFound()
      return res.data
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) throw notFound()
      throw err
    }
  },
  head: ({ loaderData }) => ({
    meta: [
      // 목차에는 고유 본문이 없다. 장 제목은 Gutenberg 원문에서 온 것이라
      // 크롤 예산을 쓸 값이 아니다 — 색인은 번역 본문에 몰아준다 (ADR-007).
      // follow는 남긴다. 63장으로 가는 길이 이 페이지에만 있다.
      { name: 'robots', content: 'noindex, follow' },
      { title: loaderData ? `${loaderData.book.title} — 목차` : 'reclassic' },
    ],
  }),
  notFoundComponent: () => (
    <main className="contents-page">
      <p>목차를 찾을 수 없습니다.</p>
    </main>
  ),
  component: Contents,
})

const LANG_LABEL: Record<string, string> = { ko: '한국어' }

// 진행도를 0~100 정수로 가둔다. 이 값이 그대로 막대 폭이 되므로
// 범위를 벗어나면 막대가 칸을 넘어간다.
function percent(ratio: number) {
  return Math.max(0, Math.min(100, Math.round(ratio * 100)))
}

function Contents() {
  const { projectId } = Route.useParams()
  const { book, progress, items } = Route.useLoaderData()
  const done = percent(progress.ratio)
  const lang = LANG_LABEL[book.targetLang] ?? book.targetLang

  return (
    <main className="contents-page">
      <h1>{book.title}</h1>
      <p className="contents-meta">
        {book.author ? `${book.author} · ` : null}
        {items.length}장 · {lang} 번역 {done}%
      </p>
      {/* 막대는 바로 위 문장을 그림으로 되풀이할 뿐이라 보조기기에서는 숨긴다.
          숫자만 넘기고 단위(%)는 styles.css가 붙인다 — 화면 파일에 치수를 쓰지
          않는다는 규칙을 지키면서 데이터로 폭을 정하는 방법이다 (ADR-038). */}
      <div className="contents-bar" aria-hidden="true">
        <span style={{ '--pct': done } as CSSProperties} />
      </div>

      {items.length === 0 ? (
        <p>아직 읽을 수 있는 장이 없습니다.</p>
      ) : (
        <ol className="contents-list">
          {items.map((chapter) => {
            const ratio = percent(chapter.coverage.ratio)
            return (
              <li key={chapter.idx}>
                {/* 진행도 0%인 장도 링크다. 원문은 있으므로 읽을 수 있고,
                    막아 두면 번역이 시작되지 않은 장으로 갈 길이 사라진다.

                    목차에서 장으로 들어가는 것도 앞으로 가는 이동이다 — 문서 이동이
                    CSS만으로 그러듯 본문이 오른쪽에서 들어온다 (ADR-041). 붙이지 않으면
                    하이드레이션 전후로 같은 클릭이 다르게 보인다. */}
                <Link
                  to="/projects/$projectId/chapters/$idx"
                  params={{ projectId, idx: String(chapter.idx) }}
                  data-untranslated={ratio === 0 ? '' : undefined}
                  viewTransition={{ types: ['chapter-next'] }}
                >
                  <span className="contents-idx">
                    {String(chapter.idx + 1).padStart(2, '0')}
                  </span>
                  <span className="contents-title">
                    {chapter.title || `${chapter.idx + 1}장`}
                    {ratio === 0 && <small>원문만 있음</small>}
                  </span>
                  <span className="contents-pct">{ratio}%</span>
                </Link>
              </li>
            )
          })}
        </ol>
      )}
    </main>
  )
}

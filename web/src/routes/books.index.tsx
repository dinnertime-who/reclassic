import { Link, createFileRoute } from '@tanstack/react-router'

import { listBooks } from '#/api/gen/reclassic'

// `/books` 목록이다. 파일 이름이 `books.tsx`가 아닌 이유 — TanStack Router의
// flat 라우팅은 `books.tsx`를 `/books/$gutenbergId/...` 의 부모 레이아웃으로
// 승격시킨다. 이 화면은 `<Outlet/>`을 렌더하지 않으므로 그대로 두면 원문
// 읽기 화면이 아예 안 뜬다. 편집 라우트의 `$idx_` 와 같은 함정이다.
// 인덱스 파일이면 챕터 라우트와 형제가 되고 URL은 `/books` 그대로다.
export const Route = createFileRoute('/books/')({
  loader: async () => (await listBooks()).data,
  head: () => ({
    meta: [{ title: '도서 목록 — reclassic' }],
  }),
  component: BookList,
})

const LANG_LABEL: Record<string, string> = { ko: '한국어' }

function BookList() {
  const { items } = Route.useLoaderData()

  return (
    <main>
      <h1>도서 목록</h1>
      <p>공개된 번역만 있습니다. 목록에 없는 책은 아직 발견되지 않습니다.</p>

      {items.length === 0 ? (
        <p>아직 공개된 번역이 없습니다.</p>
      ) : (
        <ul className="book-list">
          {items.map((book) => (
            // 카드 전체가 링크다. 모바일에서 제목만 누르게 하면 표적이 너무 작다 (ADR-038).
            <li key={book.projectId} className="book-card">
              <Link
                to="/projects/$projectId/chapters/$idx"
                params={{ projectId: String(book.projectId), idx: '0' }}
              >
                <h2>{book.title}</h2>
                <p>
                  {book.author ? `${book.author} · ` : null}
                  {LANG_LABEL[book.targetLang] ?? book.targetLang} 번역 읽기 →
                </p>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}

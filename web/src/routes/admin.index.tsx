// 관리자 확인 큐 (CSR) — needs_review 도서 + 승계 고아.
// 둘 다 읽기만 한다. 상태를 바꾸는 버튼을 만들지 마라 (ADR-014·004).
import { createFileRoute } from '@tanstack/react-router'

import { useListNeedsReviewBooks, useListOrphanedSuccessions } from '#/api/gen/reclassic'
import type { NeedsReviewBook, OrphanedSuccession } from '#/api/gen/model'
import { Badge } from '#/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import {
  CHAPTER_LIMIT,
  PARAGRAPH_LIMIT,
  ScreenAlert,
  formatCount,
  formatCreatedAt,
  listErrorNotice,
} from '#/routes/admin'

export const Route = createFileRoute('/admin/')({
  component: AdminQueue,
})

function AdminQueue() {
  const booksQuery = useListNeedsReviewBooks()
  const orphansQuery = useListOrphanedSuccessions()

  return (
    <div className="mt-6 space-y-8">
      <section>
        <h2 className="text-xl font-semibold">합본 게이트에 걸린 도서</h2>
        <p className="text-sm text-muted-foreground">
          챕터 {formatCount(CHAPTER_LIMIT)} 또는 문단 {formatCount(PARAGRAPH_LIMIT)}을
          넘기면 여기로 온다 (ADR-014). 지금 할 수 있는 조작은 없다 — 보는 것이 몫이다.
        </p>
        {booksQuery.isPending ? (
          <p className="mt-4 text-sm text-muted-foreground">불러오는 중…</p>
        ) : booksQuery.isError ? (
          <ScreenAlert tone="error" title="불러오지 못했습니다">
            {listErrorNotice(booksQuery.error, '확인 큐를 불러오지 못했습니다.').text}
          </ScreenAlert>
        ) : (
          <NeedsReviewList
            items={booksQuery.data.status === 200 ? booksQuery.data.data.items : []}
          />
        )}
      </section>

      <section>
        <h2 className="text-xl font-semibold">승계 고아</h2>
        <p className="text-sm text-muted-foreground">
          사람이 쓴 번역이 재파싱 뒤 갈 곳을 잃은 기록이다 (ADR-004 불변식 1).
          읽기만 한다. 되살리는 조작은 stable_id 규칙에 닿는다.
        </p>
        {orphansQuery.isPending ? (
          <p className="mt-4 text-sm text-muted-foreground">불러오는 중…</p>
        ) : orphansQuery.isError ? (
          <ScreenAlert tone="error" title="불러오지 못했습니다">
            {listErrorNotice(orphansQuery.error, '승계 고아 목록을 불러오지 못했습니다.').text}
          </ScreenAlert>
        ) : (
          <OrphanList
            items={orphansQuery.data.status === 200 ? orphansQuery.data.data.items : []}
          />
        )}
      </section>
    </div>
  )
}

function NeedsReviewList({ items }: { items: NeedsReviewBook[] }) {
  if (items.length === 0) {
    return <p className="mt-4 text-sm text-muted-foreground">합본 게이트에 걸린 도서가 없습니다.</p>
  }

  return (
    <ul className="mt-4 list-none space-y-3 p-0">
      {items.map((book) => (
        <li key={book.gutenbergId}>
          <Card>
            <CardHeader>
              <CardTitle className="flex flex-wrap items-center gap-2">
                <span>{book.title}</span>
                <Badge variant="outline">{book.gutenbergId}</Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm">
              {book.author ? <p className="my-0 text-muted-foreground">{book.author}</p> : null}
              <p className="my-1">
                챕터 {formatCount(book.chapterCount)} / {formatCount(CHAPTER_LIMIT)}
                {' · '}
                문단 {formatCount(book.paragraphCount)} / {formatCount(PARAGRAPH_LIMIT)}
              </p>
            </CardContent>
          </Card>
        </li>
      ))}
    </ul>
  )
}

function OrphanList({ items }: { items: OrphanedSuccession[] }) {
  if (items.length === 0) {
    return (
      <p className="mt-4 text-sm text-muted-foreground">
        갈 곳을 잃은 번역이 없습니다. 재파싱 뒤에도 확정본이 모두 새 문단에 붙었습니다.
      </p>
    )
  }

  return (
    <ul className="mt-4 list-none space-y-3 p-0">
      {items.map((row) => (
        <li key={`${row.gutenbergId}-${row.createdAt}`}>
          <Card>
            <CardHeader>
              <CardTitle className="flex flex-wrap items-center gap-2">
                <span>{row.title}</span>
                <Badge variant="outline">{row.gutenbergId}</Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm">
              <p className="my-0">
                고아 {formatCount(row.orphaned)}건 · {formatCreatedAt(row.createdAt)}
              </p>
            </CardContent>
          </Card>
        </li>
      ))}
    </ul>
  )
}

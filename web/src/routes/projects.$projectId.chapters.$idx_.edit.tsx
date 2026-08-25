// 편집·검수 화면 (CSR) — /projects/$projectId/chapters/$idx/edit
//
// **파일 이름의 `$idx_` 는 오타가 아니다.** TanStack Router의 flat 라우팅은
// `projects.$projectId.chapters.$idx.edit.tsx` 를 보면 옆에 있는 읽기 라우트
// `projects.$projectId.chapters.$idx.tsx` 를 **부모 레이아웃으로 만든다.**
// 읽기 라우트는 `<Outlet/>` 을 렌더하지 않으므로 그대로 두면 이 화면이 아예 안 뜬다.
// 읽기 라우트는 고치지 않는다(ADR-035·007·023) — 그래서 세그먼트에 `_` 를 붙여
// 중첩을 끊었다. **URL은 `/projects/$projectId/chapters/$idx/edit` 그대로다**
// (routeTree.gen.ts 의 fullPath 확인).
//
// 데이터는 전부 react-query 훅으로 받는다 — `useQuery` 는 서버에서 돌지 않으므로
// 이 화면은 하이드레이션 후 클라이언트에서 채워진다 (ADR-035). 읽기 화면과 달리
// 자바스크립트 없이 뜰 필요가 없다 (ADR-006).
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Link, createFileRoute, useLoaderData } from '@tanstack/react-router'

import {
  getGetProjectChapterQueryKey,
  getListProposalsQueryKey,
  useCreateProposal,
  useGetProjectChapter,
  useListProposals,
  useReviewProposal,
} from '#/api/gen/reclassic'
import type {
  CurrentUser,
  Proposal,
  ProposalStatus,
  ReviewInputAction,
  TranslatedParagraph,
} from '#/api/gen/model'
import { ApiError } from '#/api/http'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '#/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '#/components/ui/collapsible'
import { Textarea } from '#/components/ui/textarea'

export const Route = createFileRoute('/projects/$projectId/chapters/$idx_/edit')({
  component: EditChapterRoute,
})

// ── 에러 → 화면이 보여줄 뜻 ────────────────────────────────────────────────
// `apiFetch` 는 비 2xx에서 ApiError를 **throw** 한다. 반환값이 아니라
// 잡힌 에러의 status로 분기한다. 에러를 삼키지 않는다 (명세 §4.5).

/** 화면에 띄우는 한 줄. `info` 는 실패가 아니다 — 409 회복이 여기 온다. */
export type Notice = { tone: 'info' | 'error'; text: string }

export function statusOf(err: unknown): number | undefined {
  return err instanceof ApiError ? err.status : undefined
}

const LOGIN_REQUIRED = '로그인이 필요합니다. 위 세션 바에서 로그인한 뒤 다시 시도하세요.'

/** 다른 검수자가 먼저 처리한 경우 (ADR-024). **실패가 아니다 — 재조회로 회복한다.** */
export const REVIEW_CONFLICT_TEXT =
  '다른 검수자가 먼저 처리했습니다. 최신 상태로 다시 불러왔습니다.'

export function proposalNotice(err: unknown): Notice {
  switch (statusOf(err)) {
    case 401:
      return { tone: 'error', text: LOGIN_REQUIRED }
    case 409:
      return {
        tone: 'error',
        text: '이미 대기 중인 제안이 있습니다. 검수가 끝난 뒤 다시 제안할 수 있습니다.',
      }
    default:
      return { tone: 'error', text: '제안을 저장하지 못했습니다.' }
  }
}

export function reviewNotice(err: unknown): Notice {
  switch (statusOf(err)) {
    case 401:
      return { tone: 'error', text: LOGIN_REQUIRED }
    case 403:
      // 여기까지 오면 화면이 그리지 말았어야 할 버튼이 눌린 것이다.
      // 권한 판정은 서버가 한다 — 화면은 반영할 뿐이다 (명세 §4.6).
      return { tone: 'error', text: '검수 권한이 없습니다.' }
    case 404:
      return { tone: 'error', text: '그 제안을 찾을 수 없습니다.' }
    case 409:
      return { tone: 'info', text: REVIEW_CONFLICT_TEXT }
    default:
      return { tone: 'error', text: '검수를 저장하지 못했습니다.' }
  }
}

/** 검수는 지금 관리자만 할 수 있다. `reviewer` 는 CHECK에만 있고 역할을 바꾸는 쿼리가
 *  없어 실제로 존재할 수 없다 (docs/tech-debt.md D3). 그래도 서버(`auth.CanReview`)와
 *  같은 기준으로 분기해 둔다 — 쿼리가 생기는 날 화면은 고칠 것이 없다. */
export function canReview(user: CurrentUser | null | undefined): boolean {
  return user?.role === 'admin' || user?.role === 'reviewer'
}

const PROPOSAL_STATUS_LABEL: Record<ProposalStatus, string> = {
  pending: '대기',
  approved: '승인',
  rejected: '반려',
  superseded: '대체됨',
  withdrawn: '철회',
}

export function formatCreatedAt(iso: string): string {
  const at = new Date(iso)
  return Number.isNaN(at.getTime())
    ? iso
    : at.toLocaleString('ko-KR', { dateStyle: 'medium', timeStyle: 'short' })
}

// ── 화면 ───────────────────────────────────────────────────────────────────

function EditChapterRoute() {
  const params = Route.useParams()
  const projectId = Number(params.projectId)
  const idx = Number(params.idx)
  const valid = Number.isInteger(projectId) && Number.isInteger(idx) && idx >= 0

  // __root__ 로더가 이미 getCurrentUser 결과를 들고 있다. 요청을 늘리지 않는다.
  const user = useLoaderData({ from: '__root__' })

  const chapterQuery = useGetProjectChapter(projectId, idx, {
    query: { enabled: valid },
  })

  if (!valid) {
    return (
      <main>
        <ScreenAlert tone="error" title="주소가 올바르지 않습니다">
          projectId와 idx는 정수여야 합니다.
        </ScreenAlert>
      </main>
    )
  }

  if (chapterQuery.isPending) {
    return (
      <main>
        <p className="text-muted-foreground">불러오는 중…</p>
      </main>
    )
  }

  if (chapterQuery.isError || chapterQuery.data.status !== 200) {
    const notFound = statusOf(chapterQuery.error) === 404
    return (
      <main>
        <ScreenAlert tone="error" title={notFound ? '그런 챕터가 없습니다' : '불러오지 못했습니다'}>
          {notFound
            ? '프로젝트 번호와 챕터 번호를 확인하세요.'
            : '잠시 뒤 다시 시도하세요.'}
        </ScreenAlert>
      </main>
    )
  }

  const view = chapterQuery.data.data

  return (
    <main>
      <ChapterHeader
        projectId={String(projectId)}
        idx={String(idx)}
        title={view.chapter.title}
        current={idx}
        totalChapters={view.totalChapters}
        approved={view.coverage.approved}
        total={view.coverage.total}
      />

      <PermissionNotice user={user} />

      <ol className="mt-6 list-none space-y-4 p-0">
        {view.paragraphs.map((paragraph, position) => (
          <li key={paragraph.stableId}>
            <ParagraphCard
              paragraph={paragraph}
              position={position}
              projectId={projectId}
              idx={idx}
              canReview={canReview(user)}
              isLoggedIn={Boolean(user)}
            />
          </li>
        ))}
      </ol>
    </main>
  )
}

export function ChapterHeader({
  projectId,
  idx,
  title,
  current,
  totalChapters,
  approved,
  total,
}: {
  projectId: string
  idx: string
  title: string
  current: number
  totalChapters: number
  approved: number
  total: number
}) {
  const percent = total === 0 ? 0 : Math.round((approved / total) * 100)

  return (
    <header>
      <p className="text-sm text-muted-foreground">
        {current + 1} / {totalChapters} · 확정 {approved}/{total} ({percent}%) ·{' '}
        <Link
          to="/projects/$projectId/chapters/$idx"
          params={{ projectId, idx }}
          className="underline underline-offset-4"
        >
          읽기 화면으로
        </Link>
      </p>
      <h1 className="text-2xl font-semibold">{title || `(제목 없음) ${current}`}</h1>
      <p className="text-sm text-muted-foreground">
        문단을 펼치면 그 문단의 제안만 불러옵니다.
      </p>
    </header>
  )
}

/** 로그인·권한 상태를 화면에 그대로 비춘다. 403을 받을 수 있는 버튼을 처음부터
 *  그리지 않는 것이 목적이지, 이것이 방어선은 아니다 (명세 §4.6). */
export function PermissionNotice({ user }: { user: CurrentUser | null | undefined }) {
  const loginUrl = import.meta.env.VITE_LOGIN_URL

  if (!user) {
    return (
      <ScreenAlert tone="info" title="읽기 전용입니다">
        번역을 제안하려면 로그인이 필요합니다.{' '}
        {loginUrl ? <a href={loginUrl}>Google로 로그인</a> : '위 세션 바에서 로그인하세요.'}
      </ScreenAlert>
    )
  }

  if (!canReview(user)) {
    return (
      <ScreenAlert tone="info" title={`${user.displayName} 님으로 제안할 수 있습니다`}>
        검수 권한이 없어 승인·반려는 표시되지 않습니다.
      </ScreenAlert>
    )
  }

  return (
    <ScreenAlert tone="info" title={`${user.displayName} 님은 검수할 수 있습니다`}>
      승인은 되돌릴 수 없습니다. 확정본은 문단당 하나입니다.
    </ScreenAlert>
  )
}

export function ScreenAlert({
  tone,
  title,
  children,
}: {
  tone: Notice['tone']
  title: string
  children?: React.ReactNode
}) {
  return (
    <Alert variant={tone === 'error' ? 'destructive' : 'default'} className="mt-4">
      <AlertTitle>{title}</AlertTitle>
      {children ? <AlertDescription>{children}</AlertDescription> : null}
    </Alert>
  )
}

// ── 문단 하나 ──────────────────────────────────────────────────────────────

export function ParagraphCard({
  paragraph,
  position,
  projectId,
  idx,
  canReview: reviewable,
  isLoggedIn,
}: {
  paragraph: TranslatedParagraph
  position: number
  projectId: number
  idx: number
  canReview: boolean
  isLoggedIn: boolean
}) {
  const [open, setOpen] = useState(false)
  const [notice, setNotice] = useState<Notice | null>(null)
  const queryClient = useQueryClient()

  // **펼친 문단만 부른다.** 계약에 myProposalStatus가 없어서 전부 부르면
  // 문단 수만큼 요청이 나간다 — 챕터 하나에 수백 개다.
  // 대기 제안이 0이면 아예 부르지 않는다 (명세 §4.5).
  const proposalsQuery = useListProposals(projectId, paragraph.stableId, {
    query: { enabled: open && paragraph.proposalCount > 0 },
  })

  // 제안 작성 뒤·검수 뒤에 **둘 다** 무효화한다.
  // 챕터 뷰를 빼먹으면 approvedTranslation과 proposalCount가 낡은 채 남는다 (명세 §4.5).
  async function refetchParagraph() {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: getListProposalsQueryKey(projectId, paragraph.stableId),
      }),
      queryClient.invalidateQueries({
        queryKey: getGetProjectChapterQueryKey(projectId, idx),
      }),
    ])
  }

  const create = useCreateProposal({
    mutation: {
      onSuccess: async () => {
        setNotice({ tone: 'info', text: '제안했습니다. 검수를 기다립니다.' })
        await refetchParagraph()
      },
      onError: (err) => setNotice(proposalNotice(err)),
    },
  })

  const review = useReviewProposal({
    mutation: {
      onSuccess: async () => {
        setNotice(null)
        await refetchParagraph()
      },
      onError: async (err) => {
        setNotice(reviewNotice(err))
        if (statusOf(err) === 409) {
          // ADR-024 — 다른 검수자가 먼저 처리했다. **실패로 끝내지 않는다.**
          // 무효화 후 재조회로 회복하고, 위 notice가 그 뜻을 화면에 보여준다 (명세 §5).
          await refetchParagraph()
        }
      },
    },
  })

  async function submitProposal(text: string): Promise<boolean> {
    setNotice(null)
    try {
      await create.mutateAsync({ projectId, stableId: paragraph.stableId, data: { text } })
      return true
    } catch {
      // 뜻은 onError가 이미 notice에 담았다. 여기서는 폼을 비우지 않기만 하면 된다.
      return false
    }
  }

  async function submitReview(proposalId: number, action: ReviewInputAction, note: string) {
    setNotice(null)
    try {
      await review.mutateAsync({ proposalId, data: note ? { action, note } : { action } })
    } catch {
      // 409는 회복 경로다. onError가 재조회까지 끝냈다.
    }
  }

  const proposals =
    proposalsQuery.data?.status === 200 ? proposalsQuery.data.data : undefined

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <span className="text-muted-foreground">#{position + 1}</span>
          {paragraph.approvedTranslation ? (
            <Badge variant="secondary">확정</Badge>
          ) : (
            <Badge variant="outline">미확정</Badge>
          )}
          {paragraph.proposalCount > 0 && (
            <Badge>대기 {paragraph.proposalCount}</Badge>
          )}
        </CardTitle>
      </CardHeader>

      <CardContent className="space-y-2">
        <p lang="en" className="my-0 whitespace-pre-wrap text-muted-foreground">
          {paragraph.sourceText}
        </p>
        {paragraph.approvedTranslation ? (
          <p lang="ko" className="my-0 whitespace-pre-wrap">
            {paragraph.approvedTranslation}
          </p>
        ) : (
          <p className="my-0 text-sm text-muted-foreground">확정 번역이 없습니다.</p>
        )}
      </CardContent>

      <Collapsible open={open} onOpenChange={setOpen}>
        <CardContent>
          <CollapsibleTrigger asChild>
            <Button variant="outline" size="sm">
              {open ? '접기' : `펼치기 (제안 ${paragraph.proposalCount})`}
            </Button>
          </CollapsibleTrigger>
        </CardContent>

        <CollapsibleContent>
          <CardContent className="space-y-3 pt-3">
            {notice && (
              <ScreenAlert
                tone={notice.tone}
                title={notice.tone === 'error' ? '처리하지 못했습니다' : '알림'}
              >
                {notice.text}
              </ScreenAlert>
            )}

            {paragraph.proposalCount === 0 ? (
              // 대기 제안이 0이면 목록을 부르지 않는다. 캐시에 남은 옛 목록도 그리지 않는다 —
              // 무효화해도 비활성 쿼리는 다시 받아오지 않아 낡은 채로 보이게 된다.
              <p className="text-sm text-muted-foreground">대기 중인 제안이 없습니다.</p>
            ) : proposalsQuery.isPending ? (
              <p className="text-sm text-muted-foreground">제안을 불러오는 중…</p>
            ) : proposalsQuery.isError ? (
              <p className="text-sm text-destructive">제안 목록을 불러오지 못했습니다.</p>
            ) : (
              <ul className="list-none space-y-2 p-0">
                {(proposals ?? []).map((proposal) => (
                  <li key={proposal.id}>
                    <ProposalRow
                      proposal={proposal}
                      canReview={reviewable}
                      pending={review.isPending}
                      onReview={submitReview}
                    />
                  </li>
                ))}
              </ul>
            )}

            {isLoggedIn ? (
              <ProposalForm pending={create.isPending} onSubmit={submitProposal} />
            ) : (
              <p className="text-sm text-muted-foreground">
                로그인하면 이 문단의 번역을 제안할 수 있습니다.
              </p>
            )}
          </CardContent>
        </CollapsibleContent>
      </Collapsible>
    </Card>
  )
}

// ── 제안 하나 ──────────────────────────────────────────────────────────────

export function ProposalRow({
  proposal,
  canReview: reviewable,
  pending,
  onReview,
}: {
  proposal: Proposal
  canReview: boolean
  pending: boolean
  onReview: (proposalId: number, action: ReviewInputAction, note: string) => void
}) {
  const [note, setNote] = useState('')

  // 검수 조작은 권한이 있고 아직 대기 중일 때만 그린다.
  // 403을 받을 수 있는 버튼을 처음부터 그리지 않는다 (명세 §4.6).
  const showReview = reviewable && proposal.status === 'pending'

  return (
    <article className="rounded-lg border border-border p-3">
      <p className="my-0 flex items-center gap-2 text-xs text-muted-foreground">
        <Badge variant={proposal.status === 'pending' ? 'default' : 'outline'}>
          {PROPOSAL_STATUS_LABEL[proposal.status]}
        </Badge>
        <span>{proposal.authorHandle}</span>
        <span>{formatCreatedAt(proposal.createdAt)}</span>
      </p>
      <p lang="ko" className="mt-2 mb-0 whitespace-pre-wrap">
        {proposal.text}
      </p>

      {showReview && (
        <div className="mt-3 space-y-2">
          <Textarea
            aria-label="검수 사유"
            placeholder="사유 (선택)"
            rows={2}
            value={note}
            onChange={(e) => setNote(e.target.value)}
          />
          <div className="flex gap-2">
            <Button
              type="button"
              size="sm"
              disabled={pending}
              onClick={() => onReview(proposal.id, 'approve', note.trim())}
            >
              승인
            </Button>
            <Button
              type="button"
              size="sm"
              variant="destructive"
              disabled={pending}
              onClick={() => onReview(proposal.id, 'reject', note.trim())}
            >
              반려
            </Button>
          </div>
        </div>
      )}
    </article>
  )
}

// ── 제안 작성 ──────────────────────────────────────────────────────────────
// 폼 라이브러리를 쓰지 않는다. 제어 컴포넌트로 직접 (ADR-035).

export function ProposalForm({
  pending,
  onSubmit,
}: {
  pending: boolean
  /** 성공했으면 true. 그때만 입력을 비운다 — 실패하면 쓴 것을 잃지 않는다. */
  onSubmit: (text: string) => Promise<boolean>
}) {
  const [text, setText] = useState('')
  const trimmed = text.trim()

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!trimmed || pending) return
    if (await onSubmit(trimmed)) setText('')
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-2">
      <Textarea
        aria-label="번역 제안"
        placeholder="이 문단의 번역을 제안하세요"
        rows={3}
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      <Button type="submit" size="sm" disabled={!trimmed || pending}>
        {pending ? '제안하는 중…' : '제안'}
      </Button>
    </form>
  )
}

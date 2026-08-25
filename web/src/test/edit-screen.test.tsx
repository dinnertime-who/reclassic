// 편집·검수 화면 테스트 — 명세 §4.7이 요구하는 네 가지.
//
// **DATABASE_URL도, API 서버도 없이 돈다.** 모의는 딱 한 곳, `apiFetch`다.
// 그 위(orval 생성 훅 · react-query · 쿼리 키 · 무효화 · 라우터)는 전부 진짜로 돈다.
// 훅을 통째로 모의하면 이 파일이 물어야 할 질문 — "제안 뒤에 **챕터 뷰까지** 다시
// 불렀는가"(§4.5), "409를 받고 **재조회**했는가"(§5) — 를 아예 물어볼 수 없다.
// 그래서 seam을 네트워크 직전 한 겹으로 내렸다.
import { QueryClient } from '@tanstack/react-query'
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('#/api/http', async (importOriginal) => {
  const actual = await importOriginal<typeof import('#/api/http')>()
  // ApiError는 진짜를 쓴다. 화면이 `err instanceof ApiError`로 상태 코드를 꺼내므로
  // 흉내 낸 클래스를 넣으면 테스트가 자기 자신을 검사하게 된다.
  return { ...actual, apiFetch: vi.fn() }
})

import { ApiError, apiFetch } from '#/api/http'
import type {
  CurrentUser,
  Proposal,
  ProjectChapterView,
  ProposalInput,
  TranslatedParagraph,
} from '#/api/gen/model'
import { routeTree } from '#/routeTree.gen'

// ── 가짜 API ───────────────────────────────────────────────────────────────
// 실제 라우팅 규칙은 orval이 만든 URL이 정한다. 여기서는 그 URL을 그대로 받아
// **호출을 기록**한다 — 이 파일의 단언 절반이 "몇 번 불렀나"에 걸려 있다.

const ME = '/auth/me'
const CHAPTER = '/projects/1/chapters/0'
const PROPOSALS = '/projects/1/paragraphs/p2/proposals'
const REVIEW = '/proposals/7/review'

type Method = 'GET' | 'POST'
type Reply = { status: number; data: unknown }
type Handler = (body: unknown) => Reply

const ok = (data: unknown): Reply => ({ status: 200, data })

function fakeApi() {
  const calls: { method: Method; url: string; body: unknown }[] = []
  const routes = new Map<string, Handler>()

  const impl = async (url: string, init?: RequestInit) => {
    const method = (init?.method ?? 'GET') as Method
    const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
    calls.push({ method, url, body })

    const handler = routes.get(`${method} ${url}`)
    if (!handler) throw new Error(`테스트가 준비하지 않은 요청: ${method} ${url}`)

    const reply = handler(body)
    // **apiFetch와 같은 규칙**: 비 2xx는 반환이 아니라 ApiError로 던진다.
    // 여기서 어긋나면 화면의 onError 경로가 테스트에서만 안 돌게 된다.
    if (reply.status < 200 || reply.status >= 300) {
      throw new ApiError(reply.status, `${method} ${url} → ${reply.status}`)
    }
    return { data: reply.data, status: reply.status, headers: new Headers() }
  }

  vi.mocked(apiFetch).mockImplementation(impl as unknown as typeof apiFetch)

  return {
    on(method: Method, url: string, handler: Handler) {
      routes.set(`${method} ${url}`, handler)
      return this
    },
    /** 그 요청이 몇 번 나갔나. "재조회했다"를 이걸로 단언한다. */
    count(method: Method, url: string) {
      return calls.filter((c) => c.method === method && c.url === url).length
    },
    bodyOf(method: Method, url: string) {
      return calls.find((c) => c.method === method && c.url === url)?.body
    },
  }
}

// ── 화면 띄우기 ────────────────────────────────────────────────────────────

function renderEditScreen() {
  // **QueryClient를 모듈 스코프에 두지 않는다** (ADR-035 · 명세 §5).
  // 프로덕션에서는 한 사람의 응답이 다른 사람에게 나가기 때문이고,
  // 여기서는 앞 테스트의 캐시가 뒤 테스트로 새기 때문이다. 이유는 하나다.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: 60_000, retry: false } },
  })

  const router = createRouter({
    routeTree,
    context: { queryClient },
    history: createMemoryHistory({
      initialEntries: ['/projects/1/chapters/0/edit'],
    }),
    // defaultPreload는 켜지 않는다. 읽기 라우트를 미리 불러오면 그쪽 로더가
    // 같은 GET /projects/1/chapters/0을 부르고, 이 파일이 세는 횟수가 오염된다.
  })
  // 프로덕션(router.tsx)과 같은 통합이다. QueryClientProvider를 여기서 직접
  // 감싸지 않는 것도 같다 — wrapQueryClient가 기본 true다.
  setupRouterSsrQueryIntegration({ router, queryClient })

  return render(<RouterProvider router={router} />)
}

/** 그 글이 들어 있는 가장 가까운 `selector` 요소. 단언을 한 조각 안으로 가둔다. */
function enclosing(text: string, selector: string): HTMLElement {
  const found = screen.getByText(text).closest(selector)
  if (!(found instanceof HTMLElement)) {
    throw new Error(`${selector}를 찾지 못했다: ${text}`)
  }
  return found
}

/** 원문이 들어 있는 문단 카드(<li>). */
const paragraphCard = (sourceText: string) => enclosing(sourceText, 'li')

/** 그 제안 하나짜리 항목(<article>). 옆 제안의 버튼을 잘못 누르지 않게 한다. */
const proposalRow = (text: string) => enclosing(text, 'article')

async function expand(card: HTMLElement) {
  await userEvent.setup().click(within(card).getByRole('button', { name: /^펼치기/ }))
}

// ── 붙박이 자료 ────────────────────────────────────────────────────────────

const ADMIN: CurrentUser = { handle: 'admin', displayName: '관리자', role: 'admin' }
const MEMBER: CurrentUser = { handle: 'reader', displayName: '독자', role: 'member' }

const SOURCE_1 = 'It was a bright cold day in April.'
const APPROVED_1 = '4월의 맑고 추운 날이었다.'
const SOURCE_2 = 'The clocks were striking thirteen.'
const OTHER_TEXT = '시계가 열세 시를 치고 있었다.'
const MY_TEXT = '시계들이 열세 번을 울리고 있었다.'

const CONFIRMED: TranslatedParagraph = {
  stableId: 'p1',
  sourceText: SOURCE_1,
  approvedTranslation: APPROVED_1,
  proposalCount: 0,
}

function unconfirmed(proposalCount = 0): TranslatedParagraph {
  return {
    stableId: 'p2',
    sourceText: SOURCE_2,
    approvedTranslation: null,
    proposalCount,
  }
}

function chapter(paragraphs: TranslatedParagraph[], approved: number): ProjectChapterView {
  return {
    chapter: { idx: 0, title: 'Chapter One' },
    paragraphs,
    totalChapters: 3,
    coverage: { total: paragraphs.length, approved, ratio: approved / paragraphs.length },
    indexable: false,
  }
}

function proposal(over: Partial<Proposal> & Pick<Proposal, 'id' | 'text'>): Proposal {
  return {
    projectId: 1,
    stableId: 'p2',
    authorHandle: 'someone',
    status: 'pending',
    createdAt: '2026-08-20T09:00:00Z',
    ...over,
  }
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

// ── 1. 문단 목록 ───────────────────────────────────────────────────────────

describe('문단 목록', () => {
  it('확정 번역과 원문을 옳게 가른다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ({ status: 401, data: { message: '로그인이 필요하다' } }))
    api.on('GET', CHAPTER, () => ok(chapter([CONFIRMED, unconfirmed()], 1)))

    renderEditScreen()
    await screen.findByRole('heading', { name: 'Chapter One' })

    // 문단은 받은 순서 그대로, 하나도 빠지지 않고 그려진다.
    const items = screen.getAllByRole('listitem')
    expect(items).toHaveLength(2)
    expect(items[0]).toHaveTextContent(SOURCE_1)
    expect(items[1]).toHaveTextContent(SOURCE_2)

    // 확정본이 있는 문단: 원문은 en, 확정 번역은 ko. **서로 다른 요소다.**
    const first = paragraphCard(SOURCE_1)
    expect(within(first).getByText(SOURCE_1)).toHaveAttribute('lang', 'en')
    expect(within(first).getByText(APPROVED_1)).toHaveAttribute('lang', 'ko')
    expect(within(first).getByText('확정')).toBeInTheDocument()
    expect(within(first).queryByText('확정 번역이 없습니다.')).not.toBeInTheDocument()

    // 확정본이 없는 문단: 원문만 있고, 없다는 사실을 말한다.
    // 원문을 확정 번역인 척 보여주지 않는다 — 읽기 화면과 다른 점이다.
    const second = paragraphCard(SOURCE_2)
    expect(within(second).getByText(SOURCE_2)).toHaveAttribute('lang', 'en')
    expect(within(second).getByText('확정 번역이 없습니다.')).toBeInTheDocument()
    expect(within(second).getByText('미확정')).toBeInTheDocument()
    expect(within(second).queryByText('확정')).not.toBeInTheDocument()
    expect(second.querySelector('[lang="ko"]')).toBeNull()
    // 옆 문단의 확정본이 새어 들어오지 않는다.
    expect(within(second).queryByText(APPROVED_1)).not.toBeInTheDocument()
  })
})

// ── 2. 제안 작성 ───────────────────────────────────────────────────────────

describe('제안 작성', () => {
  it('성공하면 제안 목록과 챕터 뷰가 함께 갱신된다', async () => {
    const api = fakeApi()

    // 서버 상태를 흉내 낸다. 제안이 하나 늘면 목록도 proposalCount도 같이 는다.
    let proposals = [proposal({ id: 7, text: OTHER_TEXT })]

    api.on('GET', ME, () => ok(MEMBER))
    api.on('GET', CHAPTER, () =>
      ok(chapter([CONFIRMED, unconfirmed(proposals.length)], 1)),
    )
    api.on('GET', PROPOSALS, () => ok(proposals))
    api.on('POST', PROPOSALS, (body) => {
      const mine = proposal({
        id: 8,
        text: (body as ProposalInput).text,
        authorHandle: MEMBER.handle,
      })
      proposals = [...proposals, mine]
      return { status: 201, data: mine }
    })

    renderEditScreen()
    await screen.findByRole('heading', { name: 'Chapter One' })

    const card = paragraphCard(SOURCE_2)
    await expand(card)
    await within(card).findByText(OTHER_TEXT)
    expect(within(card).getByText('대기 1')).toBeInTheDocument()

    const user = userEvent.setup()
    await user.type(within(card).getByLabelText('번역 제안'), MY_TEXT)
    await user.click(within(card).getByRole('button', { name: '제안' }))

    // 내 제안이 목록에 나타난다.
    expect(await within(card).findByText(MY_TEXT)).toBeInTheDocument()
    expect(api.bodyOf('POST', PROPOSALS)).toEqual({ text: MY_TEXT })

    // **챕터 뷰까지 무효화했는가.** 빼먹으면 proposalCount가 1에 멈춘다 (§4.5).
    expect(await within(card).findByText('대기 2')).toBeInTheDocument()
    await waitFor(() => expect(api.count('GET', CHAPTER)).toBe(2))
    expect(api.count('GET', PROPOSALS)).toBe(2)
    expect(api.count('POST', PROPOSALS)).toBe(1)

    expect(within(card).getByText('제안했습니다. 검수를 기다립니다.')).toBeInTheDocument()
    // 성공했으니 입력을 비운다.
    expect(within(card).getByLabelText('번역 제안')).toHaveValue('')
  })
})

// ── 3. 검수 409 ────────────────────────────────────────────────────────────

describe('reviewProposal이 409를 주면', () => {
  it('실패로 끝내지 않고 재조회해 최신 상태로 회복한다', async () => {
    const api = fakeApi()

    // ADR-024가 만드는 상황: 같은 문단의 **다른 제안**을 다른 검수자가 먼저 승인했다.
    // 그 트랜잭션이 커밋되면 내가 승인하려던 제안은 더 이상 pending이 아니다 → 409.
    let lost = false

    api.on('GET', ME, () => ok(ADMIN))
    api.on('GET', CHAPTER, () =>
      lost
        ? ok(chapter([CONFIRMED, { ...unconfirmed(0), approvedTranslation: OTHER_TEXT }], 2))
        : ok(chapter([CONFIRMED, unconfirmed(2)], 1)),
    )
    api.on('GET', PROPOSALS, () =>
      ok(
        lost
          ? [
              proposal({ id: 7, text: MY_TEXT, status: 'superseded' }),
              proposal({ id: 9, text: OTHER_TEXT, status: 'approved' }),
            ]
          : [
              proposal({ id: 7, text: MY_TEXT }),
              proposal({ id: 9, text: OTHER_TEXT }),
            ],
      ),
    )
    api.on('POST', REVIEW, () => {
      // 이 요청이 도착했을 때는 이미 다른 검수자가 커밋한 뒤다.
      lost = true
      return { status: 409, data: { message: '다른 검수자가 먼저 처리했다' } }
    })

    renderEditScreen()
    await screen.findByRole('heading', { name: 'Chapter One' })

    const card = paragraphCard(SOURCE_2)
    await expand(card)
    await within(card).findByText(MY_TEXT)
    expect(api.count('GET', CHAPTER)).toBe(1)
    expect(api.count('GET', PROPOSALS)).toBe(1)

    // 내가 승인하려는 것은 7번 제안이다. 같은 문단의 9번이 아니다.
    const mine = proposalRow(MY_TEXT)
    await userEvent.setup().click(within(mine).getByRole('button', { name: '승인' }))

    // **핵심: 다시 조회했다.** 에러가 안 났다가 아니라, 요청이 실제로 한 번 더 나갔다.
    await waitFor(() => expect(api.count('GET', CHAPTER)).toBe(2))
    await waitFor(() => expect(api.count('GET', PROPOSALS)).toBe(2))
    // 검수 요청 자체는 재시도하지 않는다. 재조회이지 재시도가 아니다.
    expect(api.count('POST', REVIEW)).toBe(1)

    // 실패로 끝내지 않는다 — 알림이지 오류가 아니다.
    expect(
      within(card).getByText('다른 검수자가 먼저 처리했습니다. 최신 상태로 다시 불러왔습니다.'),
    ).toBeInTheDocument()
    expect(within(card).getByText('알림')).toBeInTheDocument()
    expect(screen.queryByText('처리하지 못했습니다')).not.toBeInTheDocument()

    // 재조회한 결과가 화면에 반영된다: 남이 확정한 번역이 확정본으로 올라온다.
    await waitFor(() => {
      const refreshed = paragraphCard(SOURCE_2)
      expect(within(refreshed).getByText(OTHER_TEXT)).toHaveAttribute('lang', 'ko')
      expect(within(refreshed).getByText('확정')).toBeInTheDocument()
    })
    // 이미 끝난 제안에는 검수 조작을 다시 그리지 않는다.
    expect(screen.queryByRole('button', { name: '승인' })).not.toBeInTheDocument()
  })
})

// ── 4. 권한 ────────────────────────────────────────────────────────────────

describe('권한', () => {
  function withPendingProposal(user: CurrentUser | null) {
    const api = fakeApi()
    api.on('GET', ME, () =>
      user ? ok(user) : { status: 401, data: { message: '로그인이 필요하다' } },
    )
    api.on('GET', CHAPTER, () => ok(chapter([CONFIRMED, unconfirmed(1)], 1)))
    api.on('GET', PROPOSALS, () => ok([proposal({ id: 7, text: OTHER_TEXT })]))
    return api
  }

  it('비로그인이면 검수 조작도 제안 폼도 없다', async () => {
    withPendingProposal(null)

    renderEditScreen()
    await screen.findByRole('heading', { name: 'Chapter One' })

    const card = paragraphCard(SOURCE_2)
    await expand(card)
    // 읽기는 된다 — 제안 목록은 로그인 없이도 보인다.
    expect(await within(card).findByText(OTHER_TEXT)).toBeInTheDocument()

    expect(screen.getByText('읽기 전용입니다')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '승인' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '반려' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('검수 사유')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('번역 제안')).not.toBeInTheDocument()
    expect(
      within(card).getByText('로그인하면 이 문단의 번역을 제안할 수 있습니다.'),
    ).toBeInTheDocument()
  })

  it('일반 사용자는 제안은 하되 검수 조작은 보지 못한다', async () => {
    withPendingProposal(MEMBER)

    renderEditScreen()
    await screen.findByRole('heading', { name: 'Chapter One' })

    const card = paragraphCard(SOURCE_2)
    await expand(card)
    expect(await within(card).findByText(OTHER_TEXT)).toBeInTheDocument()

    expect(screen.getByText('독자 님으로 제안할 수 있습니다')).toBeInTheDocument()
    expect(within(card).getByLabelText('번역 제안')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '승인' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '반려' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('검수 사유')).not.toBeInTheDocument()
  })

  it('관리자에게는 검수 조작이 보인다 — 위 둘이 빈 화면을 보고 통과한 것이 아니다', async () => {
    withPendingProposal(ADMIN)

    renderEditScreen()
    await screen.findByRole('heading', { name: 'Chapter One' })

    const card = paragraphCard(SOURCE_2)
    await expand(card)
    expect(await within(card).findByText(OTHER_TEXT)).toBeInTheDocument()

    expect(screen.getByText('관리자 님은 검수할 수 있습니다')).toBeInTheDocument()
    expect(within(card).getByRole('button', { name: '승인' })).toBeInTheDocument()
    expect(within(card).getByRole('button', { name: '반려' })).toBeInTheDocument()
    expect(within(card).getByLabelText('검수 사유')).toBeInTheDocument()
  })
})

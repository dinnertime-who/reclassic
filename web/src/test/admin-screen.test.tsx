// 관리자 화면 — 권한 없으면 조작이 렌더되지 않는다.
// DATABASE_URL도 API 서버도 없이 돈다. 모의는 apiFetch 한 곳.
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
  return { ...actual, apiFetch: vi.fn() }
})

import { ApiError, apiFetch } from '#/api/http'
import type {
  CurrentUser,
  NeedsReviewBook,
  OrphanedSuccession,
  ProjectListItem,
  ProjectStatusInput,
  UserListItem,
  UserRoleInput,
} from '#/api/gen/model'
import { routeTree } from '#/routeTree.gen'
import {
  CHAPTER_LIMIT,
  PARAGRAPH_LIMIT,
  canChangeRole,
  formatCount,
} from '#/routes/admin'

const ME = '/auth/me'
const NEEDS_REVIEW = '/admin/books/needs-review'
const ORPHANS = '/admin/successions/orphans'
const USERS = '/admin/users'
const ADMIN_PROJECTS = '/admin/projects'

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
    count(method: Method, url: string) {
      return calls.filter((c) => c.method === method && c.url === url).length
    },
    bodyOf(method: Method, url: string) {
      return calls.find((c) => c.method === method && c.url === url)?.body
    },
  }
}

function renderAdmin(path: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: 60_000, retry: false } },
  })
  const router = createRouter({
    routeTree,
    context: { queryClient },
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  setupRouterSsrQueryIntegration({ router, queryClient })
  return render(<RouterProvider router={router} />)
}

function enclosing(text: string, selector: string): HTMLElement {
  const found = screen.getByText(text).closest(selector)
  if (!(found instanceof HTMLElement)) {
    throw new Error(`${selector}를 찾지 못했다: ${text}`)
  }
  return found
}

const ADMIN: CurrentUser = { handle: 'boss', displayName: '관리자', role: 'admin' }
const MEMBER: CurrentUser = { handle: 'reader', displayName: '독자', role: 'member' }

const SHAKESPEARE: NeedsReviewBook = {
  gutenbergId: 100,
  title: 'The Complete Works of William Shakespeare',
  author: 'William Shakespeare',
  chapterCount: 1684,
  paragraphCount: 39354,
}

const ORPHAN: OrphanedSuccession = {
  gutenbergId: 84,
  title: 'Frankenstein',
  orphaned: 3,
  createdAt: '2026-08-20T09:00:00Z',
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

describe('권한', () => {
  it('비로그인이면 관리 조작이 없고 관리자 API도 부르지 않는다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ({ status: 401, data: { message: '로그인이 필요하다' } }))

    renderAdmin('/admin')
    expect(await screen.findByText('로그인이 필요합니다')).toBeInTheDocument()
    expect(screen.queryByText('합본 게이트에 걸린 도서')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '도서 목록에 공개' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /로$/ })).not.toBeInTheDocument()
    expect(api.count('GET', NEEDS_REVIEW)).toBe(0)
    expect(api.count('GET', ORPHANS)).toBe(0)
    expect(api.count('GET', USERS)).toBe(0)
    expect(api.count('GET', ADMIN_PROJECTS)).toBe(0)
  })

  it('일반 사용자는 역할·공개 조작을 보지 못하고 API도 부르지 않는다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ok(MEMBER))

    renderAdmin('/admin/users')
    expect(await screen.findByText('관리 조작은 표시되지 않습니다')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'reviewer로' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'member로' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '도서 목록에 공개' })).not.toBeInTheDocument()
    expect(api.count('GET', USERS)).toBe(0)
    expect(api.count('GET', ADMIN_PROJECTS)).toBe(0)
  })
})

describe('확인 큐', () => {
  it('needs_review 도서에 챕터·문단 수와 임계값을 같이 보여 주고 조작 버튼은 없다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ok(ADMIN))
    api.on('GET', NEEDS_REVIEW, () => ok({ items: [SHAKESPEARE] }))
    api.on('GET', ORPHANS, () => ok({ items: [] }))

    renderAdmin('/admin')
    expect(
      await screen.findByText('The Complete Works of William Shakespeare'),
    ).toBeInTheDocument()

    const card = enclosing('The Complete Works of William Shakespeare', 'li')
    expect(card).toHaveTextContent(formatCount(SHAKESPEARE.chapterCount))
    expect(card).toHaveTextContent(formatCount(CHAPTER_LIMIT))
    expect(card).toHaveTextContent(formatCount(SHAKESPEARE.paragraphCount))
    expect(card).toHaveTextContent(formatCount(PARAGRAPH_LIMIT))

    expect(
      screen.getByText(
        '갈 곳을 잃은 번역이 없습니다. 재파싱 뒤에도 확정본이 모두 새 문단에 붙었습니다.',
      ),
    ).toBeInTheDocument()

    const main = screen.getByRole('main')
    expect(within(main).queryByRole('button')).not.toBeInTheDocument()
  })

  it('고아가 있으면 책·시각·건수를 보여 준다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ok(ADMIN))
    api.on('GET', NEEDS_REVIEW, () => ok({ items: [] }))
    api.on('GET', ORPHANS, () => ok({ items: [ORPHAN] }))

    renderAdmin('/admin')
    expect(await screen.findByText('Frankenstein')).toBeInTheDocument()
    const card = enclosing('Frankenstein', 'li')
    expect(card).toHaveTextContent(`고아 ${formatCount(ORPHAN.orphaned)}건`)
  })
})

describe('역할 부여', () => {
  const users: UserListItem[] = [
    { id: 1, handle: 'boss', displayName: '관리자', role: 'admin' },
    { id: 2, handle: 'reader', displayName: '독자', role: 'member' },
    { id: 3, handle: 'critic', displayName: '평론가', role: 'reviewer' },
  ]

  it('admin은 선택지에 없고 자기 자신과 admin 행에는 조작이 없다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ok(ADMIN))
    api.on('GET', USERS, () => ok({ items: users }))

    renderAdmin('/admin/users')
    expect(await screen.findByText(/admin은 ADMIN_EMAIL로만 정해집니다/)).toBeInTheDocument()
    await screen.findByText('독자')

    const self = enclosing('boss', 'li')
    expect(within(self).queryByRole('button')).not.toBeInTheDocument()
    expect(within(self).getByText('자기 자신의 역할은 바꿀 수 없습니다.')).toBeInTheDocument()

    const member = enclosing('독자', 'li')
    expect(within(member).getByRole('button', { name: 'reviewer로' })).toBeInTheDocument()
    expect(within(member).queryByRole('button', { name: 'member로' })).not.toBeInTheDocument()
    expect(within(member).queryByRole('button', { name: /admin/ })).not.toBeInTheDocument()

    const reviewer = enclosing('평론가', 'li')
    expect(within(reviewer).getByRole('button', { name: 'member로' })).toBeInTheDocument()
    expect(within(reviewer).queryByRole('button', { name: 'reviewer로' })).not.toBeInTheDocument()

    expect(screen.queryByRole('button', { name: /admin/ })).not.toBeInTheDocument()
  })

  it('다른 사용자를 reviewer로 올리면 목록을 다시 받는다', async () => {
    const api = fakeApi()
    let items = users.map((u) => ({ ...u }))
    api.on('GET', ME, () => ok(ADMIN))
    api.on('GET', USERS, () => ok({ items }))
    api.on('POST', '/admin/users/2/role', (body) => {
      const role = (body as UserRoleInput).role
      items = items.map((u) => (u.id === 2 ? { ...u, role } : u))
      return ok(items.find((u) => u.id === 2))
    })

    renderAdmin('/admin/users')
    await screen.findByText('독자')
    const member = enclosing('독자', 'li')
    await userEvent.setup().click(within(member).getByRole('button', { name: 'reviewer로' }))

    await waitFor(() => expect(api.count('GET', USERS)).toBe(2))
    expect(api.bodyOf('POST', '/admin/users/2/role')).toEqual({ role: 'reviewer' })
    expect(api.count('POST', '/admin/users/2/role')).toBe(1)

    const updated = enclosing('독자', 'li')
    expect(within(updated).getByText('reviewer')).toBeInTheDocument()
    expect(within(updated).getByRole('button', { name: 'member로' })).toBeInTheDocument()
  })

  it('canChangeRole은 서버 규칙을 화면에 반영한다', () => {
    const targetMember: UserListItem = {
      id: 2,
      handle: 'reader',
      displayName: '독자',
      role: 'member',
    }
    const targetSelf: UserListItem = {
      id: 1,
      handle: 'boss',
      displayName: '관리자',
      role: 'member',
    }
    const targetAdmin: UserListItem = {
      id: 4,
      handle: 'other-admin',
      displayName: '다른 관리자',
      role: 'admin',
    }

    expect(canChangeRole(ADMIN, targetMember)).toBe(true)
    expect(canChangeRole(ADMIN, targetSelf)).toBe(false)
    expect(canChangeRole(ADMIN, targetAdmin)).toBe(false)
    expect(canChangeRole(MEMBER, targetMember)).toBe(false)
    expect(canChangeRole(null, targetMember)).toBe(false)
  })
})

describe('프로젝트 공개', () => {
  const publishedAt = '2026-08-01T12:00:00Z'

  function project(over: Partial<ProjectListItem> & Pick<ProjectListItem, 'id' | 'title' | 'status'>): ProjectListItem {
    return {
      bookId: over.id,
      gutenbergId: over.id,
      author: 'Jane Austen',
      targetLang: 'ko',
      publishedAt: null,
      ...over,
    }
  }

  it('open도 보이고 published_at은 내려도 남는다', async () => {
    const api = fakeApi()
    let items: ProjectListItem[] = [
      project({ id: 1, title: 'Pride and Prejudice', status: 'published', publishedAt }),
      project({ id: 2, title: 'Emma', status: 'open' }),
      project({ id: 3, title: '보관된 책', status: 'archived', publishedAt }),
    ]

    api.on('GET', ME, () => ok(ADMIN))
    api.on('GET', ADMIN_PROJECTS, () => ok({ items }))
    api.on('POST', '/admin/projects/1/status', (body) => {
      const status = (body as ProjectStatusInput).status
      items = items.map((p) => (p.id === 1 ? { ...p, status } : p))
      return ok({
        id: 1,
        bookId: 1,
        targetLang: 'ko',
        status,
        publishedAt,
      })
    })

    renderAdmin('/admin/projects')
    expect(await screen.findByText('Pride and Prejudice')).toBeInTheDocument()
    expect(screen.getByText('Emma')).toBeInTheDocument()
    expect(screen.getByText('보관된 책')).toBeInTheDocument()

    const openCard = enclosing('Emma', 'li')
    expect(within(openCard).getByRole('button', { name: '도서 목록에 공개' })).toBeInTheDocument()
    expect(within(openCard).getByText('아직 공개된 적 없습니다')).toBeInTheDocument()

    const archived = enclosing('보관된 책', 'li')
    expect(within(archived).queryByRole('button', { name: '도서 목록에 공개' })).not.toBeInTheDocument()
    expect(within(archived).queryByRole('button', { name: '목록에서 내리기' })).not.toBeInTheDocument()
    expect(within(archived).getByText('이 슬라이스에서 옮길 수 없는 상태입니다.')).toBeInTheDocument()

    const published = enclosing('Pride and Prejudice', 'li')
    expect(within(published).getByText('공개')).toBeInTheDocument()
    await userEvent.setup().click(within(published).getByRole('button', { name: '목록에서 내리기' }))

    await waitFor(() => expect(api.count('GET', ADMIN_PROJECTS)).toBe(2))
    expect(api.bodyOf('POST', '/admin/projects/1/status')).toEqual({ status: 'open' })

    const lowered = enclosing('Pride and Prejudice', 'li')
    expect(within(lowered).getByText('작업 중')).toBeInTheDocument()
    expect(within(lowered).queryByText('아직 공개된 적 없습니다')).not.toBeInTheDocument()
    expect(within(lowered).getByText(/처음 공개:/)).toBeInTheDocument()
  })
})

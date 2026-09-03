// 도서 목록 — SSR 읽기 화면. published만 보여야 한다 (ADR-036).
// DATABASE_URL도 API 서버도 없이 돈다. 모의는 apiFetch 한 곳.
import { QueryClient } from '@tanstack/react-query'
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('#/api/http', async (importOriginal) => {
  const actual = await importOriginal<typeof import('#/api/http')>()
  return { ...actual, apiFetch: vi.fn() }
})

import { ApiError, apiFetch } from '#/api/http'
import type { BookListItem, ProjectListItem } from '#/api/gen/model'
import { routeTree } from '#/routeTree.gen'

const ME = '/auth/me'
const BOOKS = '/books'
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
  }
}

function renderBooks() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: 60_000, retry: false } },
  })
  const router = createRouter({
    routeTree,
    context: { queryClient },
    history: createMemoryHistory({ initialEntries: ['/books'] }),
  })
  setupRouterSsrQueryIntegration({ router, queryClient })
  return render(<RouterProvider router={router} />)
}

const PUBLISHED: BookListItem = {
  gutenbergId: 1342,
  title: 'Pride and Prejudice',
  author: 'Jane Austen',
  projectId: 1,
  targetLang: 'ko',
}

const ALSO_PUBLISHED: BookListItem = {
  gutenbergId: 84,
  title: 'Frankenstein',
  author: 'Mary Wollstonecraft Shelley',
  projectId: 2,
  targetLang: 'ko',
}

const OPEN_ONLY: ProjectListItem = {
  id: 9,
  bookId: 9,
  gutenbergId: 2701,
  title: 'Moby Dick — 아직 공개되지 않음',
  author: 'Herman Melville',
  targetLang: 'ko',
  status: 'open',
  publishedAt: null,
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

describe('도서 목록', () => {
  it('GET /books 의 항목만 보여 주고 관리자 목록은 부르지 않는다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ({ status: 401, data: { message: '로그인이 필요하다' } }))
    api.on('GET', BOOKS, () => ok({ items: [PUBLISHED, ALSO_PUBLISHED] }))
    // 부르면 테스트가 터진다. 일부러 핸들러를 두지 않는 쪽이 더 강하지만,
    // 실수로 호출됐을 때 메시지가 분명하도록 열어 두고 횟수로 막는다.
    api.on('GET', ADMIN_PROJECTS, () => ok({ items: [OPEN_ONLY] }))

    renderBooks()

    expect(await screen.findByRole('heading', { name: '도서 목록' })).toBeInTheDocument()
    expect(screen.getByText('Pride and Prejudice')).toBeInTheDocument()
    expect(screen.getByText('Frankenstein')).toBeInTheDocument()
    expect(screen.queryByText('Moby Dick — 아직 공개되지 않음')).not.toBeInTheDocument()

    // 카드 전체가 링크다 (ADR-038) — 접근성 이름에 제목·저자·안내가 함께 들어간다.
    // 이름을 통째로 고정하면 문구를 다듬을 때마다 테스트가 깨지므로 안내 문구만 본다.
    const links = screen.getAllByRole('link', { name: /번역 목차/ })
    expect(links).toHaveLength(2)
    // 1장이 아니라 목차로 간다 — 63장짜리 책을 이전·다음으로만 넘게 두지 않는다.
    expect(links[0]).toHaveAttribute('href', '/projects/1/chapters')
    expect(links[1]).toHaveAttribute('href', '/projects/2/chapters')
    // 제목이 링크 안에 있어야 카드를 눌러 그 책으로 간다.
    expect(links[0]).toHaveTextContent('Pride and Prejudice')
    expect(links[1]).toHaveTextContent('Frankenstein')

    expect(api.count('GET', BOOKS)).toBe(1)
    expect(api.count('GET', ADMIN_PROJECTS)).toBe(0)
  })

  it('공개된 번역이 없으면 빈 상태를 말한다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ({ status: 401, data: { message: '로그인이 필요하다' } }))
    api.on('GET', BOOKS, () => ok({ items: [] }))

    renderBooks()

    expect(await screen.findByText('아직 공개된 번역이 없습니다.')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /번역 목차/ })).not.toBeInTheDocument()
    expect(api.count('GET', BOOKS)).toBe(1)
  })
})

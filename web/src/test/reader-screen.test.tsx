// 읽기 화면 — SSR로 본문이 담겨 오고, 상단은 정보만 하단은 조작만이다 (ADR-038·039).
// DATABASE_URL도 API 서버도 없이 돈다. 모의는 apiFetch 한 곳.
//
// 여기서 지키려는 것은 **마크업의 문구가 아니라 성질**이다:
//   - 본문이 로더 데이터만으로 렌더된다 (react-query 훅 없이)
//   - 번역이 없는 문단은 `lang="en"`을 유지한다 — 스크린 리더가 영어로 읽어야 한다
//   - 장 이동 조작이 하단에 있고, 첫 장에는 "이전 장"이 없다
// 문구를 통째로 고정하지 않는다. 시안이 바뀌면 문구는 바뀌어도 이 성질은 남아야 한다.
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
import type { ChapterView, ProjectChapterView } from '#/api/gen/model'
import { routeTree } from '#/routeTree.gen'

const ME = '/auth/me'

type Reply = { status: number; data: unknown }

const ok = (data: unknown): Reply => ({ status: 200, data })

function fakeApi() {
  const calls: string[] = []
  const routes = new Map<string, () => Reply>()

  const impl = async (url: string, init?: RequestInit) => {
    const method = (init?.method ?? 'GET') as string
    calls.push(`${method} ${url}`)

    const handler = routes.get(`${method} ${url}`)
    if (!handler) throw new Error(`테스트가 준비하지 않은 요청: ${method} ${url}`)

    const reply = handler()
    if (reply.status < 200 || reply.status >= 300) {
      throw new ApiError(reply.status, `${method} ${url} → ${reply.status}`)
    }
    return { data: reply.data, status: reply.status, headers: new Headers() }
  }

  vi.mocked(apiFetch).mockImplementation(impl as unknown as typeof apiFetch)

  return {
    on(method: string, url: string, handler: () => Reply) {
      routes.set(`${method} ${url}`, handler)
      return this
    },
    count(method: string, url: string) {
      return calls.filter((c) => c === `${method} ${url}`).length
    },
  }
}

function renderAt(path: string) {
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

const TRANSLATED: ProjectChapterView = {
  chapter: { idx: 1, title: '이웃이 된 사람' },
  totalChapters: 63,
  coverage: { total: 2, approved: 1, ratio: 0.5 },
  indexable: false,
  paragraphs: [
    {
      stableId: 'p1',
      sourceText: 'It is a truth universally acknowledged.',
      approvedTranslation: '누구나 인정하는 진실이 하나 있다.',
      proposalCount: 0,
    },
    {
      stableId: 'p2',
      sourceText: 'However little known the feelings of such a man may be.',
      approvedTranslation: null,
      proposalCount: 2,
    },
  ],
}

const SOURCE_ONLY: ChapterView = {
  chapter: { idx: 0, title: 'Chapter 1' },
  totalChapters: 61,
  paragraphs: [{ stableId: 's1', sourceText: 'Call me Ishmael.' }],
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

describe('번역 읽기 화면', () => {
  it('로더 데이터만으로 본문·위치·장 이동을 렌더한다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ({ status: 401, data: { message: '로그인이 필요하다' } }))
    api.on('GET', '/projects/1/chapters/1', () => ok(TRANSLATED))

    renderAt('/projects/1/chapters/1')

    expect(
      await screen.findByRole('heading', { name: '이웃이 된 사람' }),
    ).toBeInTheDocument()
    expect(screen.getByText('누구나 인정하는 진실이 하나 있다.')).toBeInTheDocument()

    // 하단 위치 표시. 문구 전체가 아니라 "몇 번째 장인지"만 본다.
    expect(screen.getByText(/2 \/ 63장/)).toBeInTheDocument()
    // 읽는 시간은 문단 글자 수에서 나온다 — 값이 아니라 형태만 고정한다.
    expect(screen.getByText(/이 장 \d+분/)).toBeInTheDocument()

    // 진행률은 상단(정보)과 하단 막대(조작 영역)가 같은 값을 쓴다. 2/63 → 3%.
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '3')

    // 장 이동은 하단에만 있다. 이름은 접근성 이름으로 본다 — 아이콘이 바뀌어도 남아야 한다.
    expect(screen.getByRole('link', { name: '이전 장' })).toHaveAttribute(
      'href',
      '/projects/1/chapters/0',
    )
    expect(screen.getByRole('link', { name: /다음 장/ })).toHaveAttribute(
      'href',
      '/projects/1/chapters/2',
    )

    expect(api.count('GET', '/projects/1/chapters/1')).toBe(1)
  })

  it('확정 번역이 없는 문단은 원문을 lang="en"으로 유지하고 꼬리표를 단다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ({ status: 401, data: { message: '로그인이 필요하다' } }))
    api.on('GET', '/projects/1/chapters/1', () => ok(TRANSLATED))

    renderAt('/projects/1/chapters/1')

    const source = await screen.findByText(
      'However little known the feelings of such a man may be.',
    )
    // **이 속성이 이 테스트의 요점이다.** 문서는 ko인데 이 문단만 영문이다.
    expect(source).toHaveAttribute('lang', 'en')

    // 번역이 없다는 것과 제안이 쌓였다는 것을 둘 다 말해야 한다.
    expect(screen.getByText(/원문 · 번역 없음/)).toBeInTheDocument()
    expect(screen.getByText(/제안 2건/)).toBeInTheDocument()

    // 번역이 붙은 문단에는 꼬리표가 붙지 않는다.
    expect(screen.getAllByText(/원문 · 번역 없음/)).toHaveLength(1)
  })
})

describe('원문 읽기 화면', () => {
  it('첫 장에는 이전 장이 없고 다음 장만 있다', async () => {
    const api = fakeApi()
    api.on('GET', ME, () => ({ status: 401, data: { message: '로그인이 필요하다' } }))
    api.on('GET', '/books/1342/chapters/0', () => ok(SOURCE_ONLY))

    renderAt('/books/1342/chapters/0')

    expect(
      await screen.findByRole('heading', { name: 'Chapter 1' }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '이전 장' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: /다음 장/ })).toHaveAttribute(
      'href',
      '/books/1342/chapters/1',
    )
    // 원문 화면의 본문은 전부 영문이다 — 문단마다 lang이 붙어야 한다.
    expect(screen.getByText('Call me Ishmael.')).toHaveAttribute('lang', 'en')
  })
})

// 목차 — SSR 읽기 화면. 장 목록이 자바스크립트 없이 HTML에 담겨 나와야 한다
// (ADR-007·023의 SEO 전제). DATABASE_URL도 API 서버도 없이 돈다.
import { QueryClient } from '@tanstack/react-query'
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'
import { render, screen } from '@testing-library/react'
import { renderToStaticMarkup } from 'react-dom/server'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('#/api/http', async (importOriginal) => {
  const actual = await importOriginal<typeof import('#/api/http')>()
  return { ...actual, apiFetch: vi.fn() }
})

import { ApiError, apiFetch } from '#/api/http'
import type { ProjectChapterList } from '#/api/gen/model'
import { routeTree } from '#/routeTree.gen'

const ME = '/auth/me'
const CHAPTERS = '/projects/1/chapters'

type Reply = { status: number; data: unknown }

function fakeApi(replies: Record<string, Reply>) {
  const calls: string[] = []
  vi.mocked(apiFetch).mockImplementation((async (url: string) => {
    calls.push(url)
    const reply = replies[url]
    if (!reply) throw new Error(`테스트가 준비하지 않은 요청: ${url}`)
    if (reply.status < 200 || reply.status >= 300) {
      throw new ApiError(reply.status, `${url} → ${reply.status}`)
    }
    return { data: reply.data, status: reply.status, headers: new Headers() }
  }) as unknown as typeof apiFetch)
  return { calls }
}

function newRouter(path: string) {
  // QueryClient는 요청마다 새로 만든다 (ADR-035). 목차는 쓰지 않지만
  // 라우터 컨텍스트 타입이 요구하므로 준다 — 부르지 않는 것을 아래에서 확인한다.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: 60_000, retry: false } },
  })
  const router = createRouter({
    routeTree,
    context: { queryClient },
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  setupRouterSsrQueryIntegration({ router, queryClient })
  return router
}

const CONTENTS: ProjectChapterList = {
  book: { title: 'Pride and Prejudice', author: 'Jane Austen', targetLang: 'ko' },
  progress: { total: 100, approved: 82, ratio: 0.82 },
  items: [
    { idx: 0, title: 'CHAPTER I.', coverage: { total: 10, approved: 10, ratio: 1 } },
    { idx: 1, title: 'CHAPTER II.', coverage: { total: 10, approved: 4, ratio: 0.4 } },
    // 아직 번역이 없는 장. 원문은 있으므로 갈 수 있어야 한다.
    { idx: 2, title: 'CHAPTER III.', coverage: { total: 10, approved: 0, ratio: 0 } },
  ],
}

const LOGGED_OUT: Reply = { status: 401, data: { message: '로그인이 필요하다' } }

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

describe('목차', () => {
  it('장 목록이 스크립트 없는 HTML에 담겨 나온다', async () => {
    fakeApi({ [ME]: LOGGED_OUT, [CHAPTERS]: { status: 200, data: CONTENTS } })

    const router = newRouter('/projects/1/chapters')
    await router.load()
    // 하이드레이션 없이 문자열로 뽑는다. 여기 없는 것은 자바스크립트가 꺼진
    // 브라우저와 크롤러에도 없다 — 이 화면의 전제가 그것이다.
    const html = renderToStaticMarkup(<RouterProvider router={router} />)

    expect(html).not.toContain('<script')
    expect(html).toContain('Pride and Prejudice')
    expect(html).toContain('CHAPTER I.')
    expect(html).toContain('CHAPTER III.')
    expect(html).toContain('/projects/1/chapters/2')
    // 전체 진행도와 장별 진행도가 숫자로 들어 있어야 한다.
    expect(html).toContain('번역 82%')
    expect(html).toContain('100%')
    expect(html).toContain('0%')
    // 진행 막대도 HTML에 값이 실려 있어야 한다 — 스크립트가 나중에 채우지 않는다.
    // 화면은 숫자만 넘기고 단위(%)는 styles.css가 붙인다 (ADR-038).
    expect(html).toContain('--pct:82')
  })

  it('장마다 두 자리 번호·제목·진행도를 주고 0%인 장도 누를 수 있다', async () => {
    const api = fakeApi({ [ME]: LOGGED_OUT, [CHAPTERS]: { status: 200, data: CONTENTS } })

    render(<RouterProvider router={newRouter('/projects/1/chapters')} />)

    expect(
      await screen.findByRole('heading', { name: 'Pride and Prejudice' }),
    ).toBeInTheDocument()
    expect(screen.getByText('Jane Austen · 3장 · 한국어 번역 82%')).toBeInTheDocument()

    // 번호는 두 자리다. idx는 0부터지만 사람이 세는 번호는 1부터다.
    expect(screen.getByText('01')).toBeInTheDocument()
    expect(screen.getByText('03')).toBeInTheDocument()

    const links = screen.getAllByRole('link', { name: /CHAPTER/ })
    expect(links).toHaveLength(3)
    expect(links[0]).toHaveAttribute('href', '/projects/1/chapters/0')
    expect(links[2]).toHaveAttribute('href', '/projects/1/chapters/2')

    // 0%인 장은 흐리게 두되 링크는 살아 있어야 한다 — 원문은 있다.
    expect(links[2]).toHaveAttribute('data-untranslated')
    expect(links[0]).not.toHaveAttribute('data-untranslated')
    expect(screen.getByText('원문만 있음')).toBeInTheDocument()

    // 읽기 화면이다. 목차를 그리는 데 필요한 요청은 목차 하나뿐이다.
    expect(api.calls.filter((u) => u === CHAPTERS)).toHaveLength(1)
  })

  it('없는 프로젝트면 404 화면을 보여 준다', async () => {
    fakeApi({ [ME]: LOGGED_OUT, [CHAPTERS]: { status: 404, data: { message: '없다' } } })

    render(<RouterProvider router={newRouter('/projects/1/chapters')} />)

    expect(await screen.findByText('목차를 찾을 수 없습니다.')).toBeInTheDocument()
  })
})

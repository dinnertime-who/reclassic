// 읽기 설정 시트 — 화면에서의 성질만 본다 (ADR-040).
//
// 지키려는 것 셋:
//   - **본문은 설정과 무관하게 렌더된다.** 시트는 그 위에 얹히는 것이라 열려도 본문이 남는다.
//   - 단계를 바꾸면 `<html>`의 단계 번호와 쿠키가 **함께** 바뀐다. 페이지는 다시 뜨지 않는다.
//   - 나가는 값은 언제나 좁혀진 단계 번호다 — 쿠키에 무엇이 들어 있어도 그렇다.
// 문구를 통째로 고정하지 않는다. 접근성 이름으로 잡는다.
import { QueryClient } from '@tanstack/react-query'
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('#/api/http', async (importOriginal) => {
  const actual = await importOriginal<typeof import('#/api/http')>()
  return { ...actual, apiFetch: vi.fn() }
})

import { ApiError, apiFetch } from '#/api/http'
import type { ChapterView } from '#/api/gen/model'
import { READER_COOKIE, readCookie } from '#/lib/reader-prefs'
import { routeTree } from '#/routeTree.gen'

const ME = '/auth/me'
const CHAPTER = '/books/1342/chapters/0'

const SOURCE_ONLY: ChapterView = {
  chapter: { idx: 0, title: 'Chapter 1' },
  totalChapters: 61,
  paragraphs: [{ stableId: 's1', sourceText: 'Call me Ishmael.' }],
}

function fakeApi() {
  const replies: Record<string, { status: number; data: unknown }> = {
    [`GET ${ME}`]: { status: 401, data: { message: '로그인이 필요하다' } },
    [`GET ${CHAPTER}`]: { status: 200, data: SOURCE_ONLY },
  }

  const impl = async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`
    const reply = replies[key]
    if (!reply) throw new Error(`테스트가 준비하지 않은 요청: ${key}`)
    if (reply.status >= 300) throw new ApiError(reply.status, key)
    return { data: reply.data, status: reply.status, headers: new Headers() }
  }
  vi.mocked(apiFetch).mockImplementation(impl as unknown as typeof apiFetch)
}

function renderReader() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: 60_000, retry: false } },
  })
  const router = createRouter({
    routeTree,
    context: { queryClient },
    history: createMemoryHistory({ initialEntries: [CHAPTER] }),
  })
  setupRouterSsrQueryIntegration({ router, queryClient })
  return render(<RouterProvider router={router} />)
}

function setCookie(value: string) {
  document.cookie = `${READER_COOKIE}=${value}; Path=/`
}

function clearCookie() {
  document.cookie = `${READER_COOKIE}=; Path=/; Max-Age=0`
}

function step(name: string) {
  return document.documentElement.getAttribute(name)
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
  clearCookie()
  for (const name of ['data-reader-size', 'data-reader-leading', 'data-reader-margin']) {
    document.documentElement.removeAttribute(name)
  }
  fakeApi()
})

afterEach(() => {
  clearCookie()
})

describe('읽기 설정 시트', () => {
  it('설정 버튼은 하단에 있고, 처음에는 시트가 닫혀 있다', async () => {
    renderReader()

    const gear = await screen.findByRole('button', { name: '읽기 설정' })
    expect(gear).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    // 스크립트가 붙기 전에도 본문은 이미 있다 — 그것이 이 화면의 전제다 (ADR-007·023).
    expect(screen.getByText('Call me Ishmael.')).toBeInTheDocument()
  })

  it('시트를 열어도 읽던 본문이 뒤에 남는다', async () => {
    const user = userEvent.setup()
    renderReader()

    await user.click(await screen.findByRole('button', { name: '읽기 설정' }))

    expect(screen.getByRole('dialog', { name: '읽기 설정' })).toBeInTheDocument()
    // **이것이 요점이다.** 시트가 본문을 가리거나 지우면 고를 것이 보이지 않는다.
    expect(screen.getByText('Call me Ishmael.')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Chapter 1' })).toBeInTheDocument()
  })

  it('글자를 키우면 `<html>`의 단계와 쿠키가 함께 오른다', async () => {
    const user = userEvent.setup()
    setCookie('322')
    renderReader()

    await user.click(await screen.findByRole('button', { name: '읽기 설정' }))
    await user.click(screen.getByRole('button', { name: '글자 크게' }))

    expect(step('data-reader-size')).toBe('4')
    expect(readCookie(document.cookie, READER_COOKIE)).toBe('422')

    await user.click(screen.getByRole('button', { name: '글자 크게' }))
    expect(step('data-reader-size')).toBe('5')
    expect(readCookie(document.cookie, READER_COOKIE)).toBe('522')

    // 5단계가 끝이다. 더 누를 수 없어야 배율이 범위를 넘지 않는다.
    expect(screen.getByRole('button', { name: '글자 크게' })).toBeDisabled()
  })

  it('행간·여백은 단계 선택기다 — 라디오 그룹이고 고른 칸만 켜진다', async () => {
    const user = userEvent.setup()
    setCookie('322')
    renderReader()

    await user.click(await screen.findByRole('button', { name: '읽기 설정' }))

    const leading = screen.getByRole('radiogroup', { name: '행간' })
    const margin = screen.getByRole('radiogroup', { name: '여백' })
    expect(within(leading).getAllByRole('radio')).toHaveLength(3)
    expect(within(margin).getAllByRole('radio')).toHaveLength(3)

    await user.click(within(leading).getByRole('radio', { name: '넓게' }))
    expect(step('data-reader-leading')).toBe('3')

    await user.click(within(margin).getByRole('radio', { name: '좁게' }))
    expect(step('data-reader-margin')).toBe('1')

    expect(readCookie(document.cookie, READER_COOKIE)).toBe('331')
  })

  it('쿠키에 쓰레기가 들어 있으면 기본 단계에서 시작한다', async () => {
    const user = userEvent.setup()
    setCookie('9;color:red')
    renderReader()

    await user.click(await screen.findByRole('button', { name: '읽기 설정' }))

    // 기본은 3단계다. 여기서 한 칸 내리면 2단계여야 한다 — 쓰레기 값이 새어 나오지 않았다는 뜻이다.
    await user.click(screen.getByRole('button', { name: '글자 작게' }))
    expect(step('data-reader-size')).toBe('2')
    expect(readCookie(document.cookie, READER_COOKIE)).toBe('222')
  })

  it('기본값으로 되돌리면 가운데 단계로 간다', async () => {
    const user = userEvent.setup()
    setCookie('513')
    renderReader()

    await user.click(await screen.findByRole('button', { name: '읽기 설정' }))
    await user.click(screen.getByRole('button', { name: '기본값으로' }))

    expect(readCookie(document.cookie, READER_COOKIE)).toBe('322')
    expect(step('data-reader-size')).toBe('3')
    expect(step('data-reader-leading')).toBe('2')
    expect(step('data-reader-margin')).toBe('2')
  })

  it('닫으면 시트만 사라지고 본문은 그대로다', async () => {
    const user = userEvent.setup()
    renderReader()

    await user.click(await screen.findByRole('button', { name: '읽기 설정' }))
    await user.click(screen.getByRole('button', { name: '설정 닫기' }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByText('Call me Ishmael.')).toBeInTheDocument()
    // 초점은 기어로 돌아온다 — 키보드로 열었으면 자리를 잃지 않아야 한다.
    expect(screen.getByRole('button', { name: '읽기 설정' })).toHaveFocus()
  })
})

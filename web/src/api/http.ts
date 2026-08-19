// orval 생성 클라이언트가 실제 요청을 보낼 때 통과하는 유일한 지점.
// 베이스 URL 분기와 SSR 쿠키 전달을 여기 한 곳에서만 한다 (CONVENTIONS).
// 화면 코드는 직접 fetch를 쓰지 않는다.
import { createIsomorphicFn } from '@tanstack/react-start'
import { getRequestHeader } from '@tanstack/react-start/server'

// 상태 코드를 들고 다니는 에러. 라우트가 404와 그 밖의 실패를 구분해야 한다.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

// 본문이 없는 응답. JSON.parse를 시도하면 깨진다.
const NO_BODY_STATUS = new Set([204, 205, 304])

// 서버·클라이언트가 서로 다른 주소로 API를 부른다.
// createIsomorphicFn을 쓰면 클라이언트 번들에서 서버 구현이 제거된다.
const baseUrl = createIsomorphicFn()
  .server(() => {
    // SSR에서는 내부 주소로 부른다. Railway에서는 api.railway.internal이다.
    const host = process.env.API_INTERNAL_HOST
    const port = process.env.API_PORT
    if (!host || !port) {
      // 기본값을 조용히 채우지 않는다 (CONVENTIONS).
      throw new Error('API_INTERNAL_HOST / API_PORT가 비어 있다')
    }
    return `http://${host}:${port}`
  })
  .client(() => {
    const url = import.meta.env.VITE_API_URL
    if (!url) {
      throw new Error('VITE_API_URL이 비어 있다')
    }
    return url
  })

// SSR 중에는 브라우저 쿠키를 그대로 넘긴다.
// 빠뜨리면 SSR 결과가 항상 로그아웃 상태가 되고 하이드레이션 후 화면이 깜빡인다.
// 브라우저에서는 credentials: 'include'가 대신 처리한다.
const forwardedCookie = createIsomorphicFn()
  .server(() => getRequestHeader('cookie'))
  .client(() => undefined)

// orval의 fetch 클라이언트는 { data, status, headers } 형태를 기대한다.
// 생성 코드가 그 타입을 쓰므로 여기서 그대로 맞춘다.
export async function apiFetch<T>(url: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)

  const cookie = forwardedCookie()
  if (cookie) headers.set('cookie', cookie)

  const isServer = typeof window === 'undefined'
  const response = await fetch(new URL(url, baseUrl()), {
    ...init,
    headers,
    credentials: isServer ? undefined : 'include',
  })

  const raw = NO_BODY_STATUS.has(response.status) ? '' : await response.text()
  const data = raw ? JSON.parse(raw) : {}

  if (!response.ok) {
    // 실패를 조용히 삼키지 않는다. 라우트 로더가 에러 경계로 올린다.
    throw new ApiError(response.status, `${init?.method ?? 'GET'} ${url} → ${response.status}`)
  }

  return { data, status: response.status, headers: response.headers } as T
}

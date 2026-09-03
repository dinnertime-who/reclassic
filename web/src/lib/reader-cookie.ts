// SSR에서 읽기 설정 쿠키를 읽는 자리. 규격·검증·브라우저 쪽은 `reader-prefs.ts`에 있다.
//
// **서버가 읽는 것이 이 파일의 존재 이유다** — 서버가 쿠키를 읽어 `<html>`에 얹어야
// 첫 페인트부터 사용자의 값이다. localStorage를 기각한 이유가 그것이다 (ADR-040).
import { createIsomorphicFn } from '@tanstack/react-start'
import { getRequestHeader } from '@tanstack/react-start/server'

import { READER_COOKIE, parseReaderPrefs, readBrowserPrefs, readCookie } from './reader-prefs'

/**
 * 지금 요청(또는 이 브라우저)의 읽기 설정. 루트 로더가 부른다 —
 * 로더는 SSR에서 한 번 돌고, 클라이언트 이동에서 다시 돈다.
 *
 * 서버 갈래 안의 `typeof document` 검사는 군더더기가 아니다. **`createIsomorphicFn`의
 * 런타임 스텁은 갈래를 고를 때 서버 구현을 먼저 집는다** — Vite 플러그인이 갈래를 잘라내는
 * 빌드에서는 상관없지만, 플러그인 없이 도는 vitest에서는 서버 갈래가 그대로 실행되어
 * `getRequestHeader`가 요청 컨텍스트를 못 찾고 던진다. 요청이 없으면 브라우저 쿠키를 읽는
 * 것이 맞는 답이고, 실패를 조용히 삼키는 것보다 조건이 눈에 보이는 편이 낫다.
 */
export const readReaderPrefs = createIsomorphicFn()
  .client(readBrowserPrefs)
  .server(() =>
    typeof document === 'undefined'
      ? parseReaderPrefs(readCookie(getRequestHeader('cookie'), READER_COOKIE))
      : readBrowserPrefs(),
  )

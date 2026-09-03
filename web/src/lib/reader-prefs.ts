// 읽기 설정 — 쿠키 규격과 그 값을 좁히는 자리 (ADR-040).
//
// **여기가 신뢰 경계다.** 쿠키는 읽는 사람이 마음대로 고쳐 쓸 수 있는 값이고,
// 그 값이 곧바로 `<html>`을 타고 CSS까지 간다. 그래서 이 파일이 하는 일은 하나다 —
// **문자열을 단계 번호로 좁히고, 규격을 벗어나면 통째로 기본값으로 돌린다.**
//
// 치수(rem·배율)는 여기 없다. 단계 번호만 `<html>`에 얹고 실제 값은
// `styles.css`가 갖는다 (ADR-038). 문자열을 `style`에 이어 붙이면 CSS 주입이다.

export const READER_COOKIE = 'reclassic_reader'

/** 1년. 계정이 아니라 기기에 붙는 값이라 세션보다 오래 남아야 한다 (ADR-040). */
export const READER_COOKIE_MAX_AGE = 60 * 60 * 24 * 365

/** 항목별 단계 수. 글자 크기 5 · 행간 3 · 여백 3. 연속 슬라이더가 아니다 (ADR-040). */
export const READER_STEPS = { size: 5, leading: 3, margin: 3 } as const

export type ReaderPrefs = { size: number; leading: number; margin: number }

/** 가운데 단계가 기본이다. 쿠키가 없거나 규격을 벗어나면 여기로 떨어진다. */
export const READER_DEFAULT: ReaderPrefs = { size: 3, leading: 2, margin: 2 }

// 고정 폭 3자리. 앞에서부터 크기·행간·여백이고 각 자리가 그 항목의 단계 번호다.
// **부분 통과를 두지 않는다** — 한 자리라도 어긋나면 전체를 기본값으로 돌린다.
// 자릿수 자체가 허용 목록이라 `3;color:red` 같은 값은 정규식에서 그대로 떨어진다.
const COOKIE_VALUE = /^[1-5][1-3][1-3]$/

/** 단계 하나를 범위 안으로 좁힌다. 정수가 아니거나 범위 밖이면 기본값이다. */
function step(value: number, steps: number, fallback: number): number {
  return Number.isInteger(value) && value >= 1 && value <= steps ? value : fallback
}

/** 쿠키 값 문자열을 단계 셋으로 좁힌다. **믿을 수 없는 입력의 유일한 입구다.** */
export function parseReaderPrefs(raw: string | null | undefined): ReaderPrefs {
  if (!raw || !COOKIE_VALUE.test(raw)) return READER_DEFAULT
  return { size: +raw[0], leading: +raw[1], margin: +raw[2] }
}

/** 단계 셋을 고정 폭 문자열로. 범위 밖 값은 여기서도 기본값으로 접힌다. */
export function formatReaderPrefs(prefs: ReaderPrefs): string {
  return [
    step(prefs.size, READER_STEPS.size, READER_DEFAULT.size),
    step(prefs.leading, READER_STEPS.leading, READER_DEFAULT.leading),
    step(prefs.margin, READER_STEPS.margin, READER_DEFAULT.margin),
  ].join('')
}

/**
 * `Cookie` 헤더(또는 `document.cookie`)에서 이름이 정확히 일치하는 값을 꺼낸다.
 * 접두사만 같은 이름(`xreclassic_reader`)에 걸리지 않아야 한다.
 * 값은 디코드하지 않는다 — 우리가 굽는 값은 숫자뿐이고, 인코딩된 값은 규격에서 떨어지는 것이 맞다.
 */
export function readCookie(
  header: string | null | undefined,
  name: string,
): string | null {
  if (!header) return null
  for (const pair of header.split(';')) {
    const eq = pair.indexOf('=')
    if (eq < 0) continue
    if (pair.slice(0, eq).trim() !== name) continue
    return pair.slice(eq + 1).trim()
  }
  return null
}

/**
 * `document.cookie`에 넣을 한 줄. `HttpOnly`는 **없다** — 클라이언트가 써야 한다.
 * 비밀이 들어가지 않으므로 그래도 된다 (ADR-040).
 * `secure`는 호출부가 판정한다 — https에서만 켜야 http 로컬에서 조용히 버려지지 않는다.
 */
export function readerCookie(prefs: ReaderPrefs, secure: boolean): string {
  const parts = [
    `${READER_COOKIE}=${formatReaderPrefs(prefs)}`,
    'Path=/',
    `Max-Age=${READER_COOKIE_MAX_AGE}`,
    'SameSite=Lax',
  ]
  if (secure) parts.push('Secure')
  return parts.join('; ')
}

/**
 * `<html>`에 얹을 속성. **좁혀진 단계 번호만 나간다** — 값은 "1"~"5" 중 하나뿐이고
 * 대응하는 CSS 규칙이 `styles.css`에만 있다. 여기서 rem이나 색이 나가지 않는 것이 요점이다.
 */
export function readerAttrs(prefs: ReaderPrefs): Record<string, string> {
  return {
    'data-reader-size': String(step(prefs.size, READER_STEPS.size, READER_DEFAULT.size)),
    'data-reader-leading': String(
      step(prefs.leading, READER_STEPS.leading, READER_DEFAULT.leading),
    ),
    'data-reader-margin': String(
      step(prefs.margin, READER_STEPS.margin, READER_DEFAULT.margin),
    ),
  }
}

/**
 * 지금 브라우저에 저장된 설정. **브라우저에서만 부른다** — SSR은 요청 헤더를 읽는
 * `reader-cookie.ts`를 쓴다.
 */
export function readBrowserPrefs(): ReaderPrefs {
  return parseReaderPrefs(readCookie(document.cookie, READER_COOKIE))
}

/** 좁혀진 단계 번호를 `<html>`에 얹는다. SSR이 심어 둔 것과 같은 속성이다. */
export function applyReaderPrefs(prefs: ReaderPrefs): void {
  const root = document.documentElement
  for (const [name, value] of Object.entries(readerAttrs(prefs))) {
    root.setAttribute(name, value)
  }
}

/** 쿠키를 다시 굽고 `<html>`을 그 자리에서 갱신한다. 페이지를 다시 뜨우지 않는다. */
export function saveReaderPrefs(prefs: ReaderPrefs): void {
  // http로 열린 로컬에서 Secure를 붙이면 브라우저가 쿠키를 조용히 버린다.
  // 프로덕션은 https이므로 여기서 켜진다 (ADR-040 · CONVENTIONS "보안").
  document.cookie = readerCookie(prefs, location.protocol === 'https:')
  applyReaderPrefs(prefs)
}

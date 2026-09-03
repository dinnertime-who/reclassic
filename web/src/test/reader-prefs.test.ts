// 읽기 설정 쿠키 — **검증만 보는 테스트다** (ADR-040).
//
// 이 쿠키는 읽는 사람이 마음대로 고쳐 쓸 수 있고, 그 값이 서버를 지나 `<html>`까지 간다.
// 여기서 좁히지 못하면 **CSS 주입**이 된다. 그래서 정상 경로보다 **범위 밖·폭 밖·쓰레기
// 문자열**을 더 많이 넣는다. 통과 기준은 하나다 — 무엇을 넣어도 나오는 것은
// 미리 정해 둔 단계 번호뿐이다.
import { describe, expect, it } from 'vitest'

import {
  READER_COOKIE,
  READER_DEFAULT,
  READER_STEPS,
  formatReaderPrefs,
  parseReaderPrefs,
  readCookie,
  readerAttrs,
  readerCookie,
} from '#/lib/reader-prefs'

describe('쿠키 값 좁히기', () => {
  it('규격에 맞는 값만 그대로 통과한다', () => {
    expect(parseReaderPrefs('322')).toEqual({ size: 3, leading: 2, margin: 2 })
    expect(parseReaderPrefs('511')).toEqual({ size: 5, leading: 1, margin: 1 })
    expect(parseReaderPrefs('133')).toEqual({ size: 1, leading: 3, margin: 3 })
  })

  it('쿠키가 없으면 기본값이다', () => {
    expect(parseReaderPrefs(null)).toEqual(READER_DEFAULT)
    expect(parseReaderPrefs(undefined)).toEqual(READER_DEFAULT)
    expect(parseReaderPrefs('')).toEqual(READER_DEFAULT)
  })

  it('범위 밖 단계는 전부 기본값으로 접힌다', () => {
    // 글자 크기는 5단계, 행간·여백은 3단계다. 그 밖은 한 자리라도 걸리면 통째로 떨어진다.
    for (const raw of ['622', '022', '922', '342', '302', '324', '320', '-22']) {
      expect(parseReaderPrefs(raw), raw).toEqual(READER_DEFAULT)
    }
  })

  it('고정 폭을 벗어나면 기본값이다', () => {
    for (const raw of ['3', '32', '3222', ' 322', '322 ', '3 2 2']) {
      expect(parseReaderPrefs(raw), raw).toEqual(READER_DEFAULT)
    }
  })

  it('주입을 노린 문자열은 하나도 통과하지 못한다', () => {
    const attacks = [
      '3;color:red',
      '322;}html{display:none}',
      '322" onload="alert(1)',
      "322';--x:url(https://evil.example)",
      'expression(alert(1))',
      '<script>alert(1)</script>',
      'var(--link)',
      'calc(999rem)',
      '3\n22',
      '%33%32%32', // 퍼센트 인코딩도 디코드하지 않는다 — 규격에서 그대로 떨어져야 한다
      '٣٢٢', // 아랍-인도 숫자. \d 를 썼다면 여기서 새어 나간다
    ]
    for (const raw of attacks) {
      expect(parseReaderPrefs(raw), raw).toEqual(READER_DEFAULT)
    }
  })

  it('`<html>`에 나가는 값은 언제나 허용된 단계 번호뿐이다', () => {
    const allowed = {
      'data-reader-size': ['1', '2', '3', '4', '5'],
      'data-reader-leading': ['1', '2', '3'],
      'data-reader-margin': ['1', '2', '3'],
    }
    const junk = [
      '3;color:red',
      '999',
      '<script>',
      '',
      'abc',
      '322',
      '511',
      'expression(1)',
    ]
    for (const raw of junk) {
      for (const [name, value] of Object.entries(readerAttrs(parseReaderPrefs(raw)))) {
        expect(allowed[name as keyof typeof allowed], `${raw} → ${name}=${value}`).toContain(
          value,
        )
      }
    }
  })

  it('구조체로 들어온 범위 밖 값도 속성으로 나가지 못한다', () => {
    // 파서를 거치지 않고 부르는 경로가 생겨도 여기서 한 번 더 접힌다.
    const attrs = readerAttrs({ size: 99, leading: -1, margin: Number.NaN })
    expect(attrs).toEqual({
      'data-reader-size': '3',
      'data-reader-leading': '2',
      'data-reader-margin': '2',
    })
  })
})

describe('쿠키 헤더 읽기', () => {
  it('이름이 정확히 같은 것만 꺼낸다', () => {
    const header = 'xreclassic_reader=999; reclassic_reader=511; other=1'
    expect(readCookie(header, READER_COOKIE)).toBe('511')
  })

  it('접두사만 같은 이름에는 걸리지 않는다', () => {
    expect(readCookie('reclassic_reader_x=511', READER_COOKIE)).toBeNull()
    expect(readCookie('my_reclassic_reader=511', READER_COOKIE)).toBeNull()
  })

  it('헤더가 없거나 그 쿠키가 없으면 null이다', () => {
    expect(readCookie(undefined, READER_COOKIE)).toBeNull()
    expect(readCookie('', READER_COOKIE)).toBeNull()
    expect(readCookie('session=abc', READER_COOKIE)).toBeNull()
  })
})

describe('쿠키 굽기', () => {
  it('규격대로 굽는다 — HttpOnly는 없고 SameSite=Lax · Path=/ · 1년이다', () => {
    const line = readerCookie({ size: 5, leading: 1, margin: 3 }, false)
    expect(line).toContain(`${READER_COOKIE}=513`)
    expect(line).toContain('Path=/')
    expect(line).toContain('Max-Age=31536000')
    expect(line).toContain('SameSite=Lax')
    // 클라이언트가 써야 하는 값이라 HttpOnly를 붙이지 않는다 (ADR-040).
    expect(line).not.toContain('HttpOnly')
    // http에서는 Secure를 붙이지 않는다 — 붙이면 브라우저가 조용히 버린다.
    expect(line).not.toContain('Secure')
  })

  it('https에서는 Secure가 붙는다', () => {
    expect(readerCookie(READER_DEFAULT, true)).toContain('Secure')
  })

  it('범위 밖 값을 넘겨도 구워지는 것은 기본값이다', () => {
    expect(readerCookie({ size: 42, leading: 0, margin: 7 }, false)).toContain(
      `${READER_COOKIE}=322`,
    )
  })

  it('구운 값을 다시 읽으면 같은 단계가 나온다', () => {
    for (let size = 1; size <= READER_STEPS.size; size++) {
      for (let leading = 1; leading <= READER_STEPS.leading; leading++) {
        for (let margin = 1; margin <= READER_STEPS.margin; margin++) {
          const prefs = { size, leading, margin }
          expect(parseReaderPrefs(formatReaderPrefs(prefs))).toEqual(prefs)
        }
      }
    }
  })
})

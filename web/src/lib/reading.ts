// 이 장을 읽는 데 걸리는 시간. **문단 텍스트에서만 계산한다** — API가 주지 않는 값을
// 지어내지 않는다. 로더가 이미 받아 둔 문단을 세는 것이라 서버 렌더에서 끝나고,
// 자바스크립트 없이도 화면에 찍힌다 (ADR-007·023).
//
// 한글은 글자 수로, 라틴 문자는 낱말 수로 센다. **하나의 상수로 둘을 재면 한쪽이 두 배씩
// 틀린다** — 같은 뜻을 담는 데 한글은 글자가 적고 영문은 많다. 번역이 덜 된 장은 두 글이
// 섞여 있으므로 문단을 나누지 않고 문자 단위로 갈라 센다.
//
// 가정은 속도 상수 둘뿐이고, 그마저 "대략 몇 분"을 말하는 데만 쓴다.
const KO_CHARS_PER_MIN = 500
const EN_WORDS_PER_MIN = 220

// 한글 음절·자모와 한자. 원문(라틴)과 번역(한글)을 가르는 기준이다.
const CJK = /[ᄀ-ᇿ㄰-㆏가-힣぀-ヿ一-鿿]/g

/** 문단들을 읽는 데 걸리는 어림 시간(분). 최소 1분이다. */
export function readingMinutes(texts: readonly string[]): number {
  let chars = 0
  let words = 0

  for (const text of texts) {
    chars += text.match(CJK)?.length ?? 0
    // CJK를 공백으로 지우고 남은 것이 라틴 낱말이다.
    const latin = text.replace(CJK, ' ').trim()
    if (latin) words += latin.split(/\s+/).length
  }

  const minutes = chars / KO_CHARS_PER_MIN + words / EN_WORDS_PER_MIN
  return Math.max(1, Math.round(minutes))
}

/** 이 장이 책의 몇 %인지. 상단 표시와 하단 진행 막대가 같은 값을 쓴다. */
export function chapterProgress(current: number, totalChapters: number): number {
  if (totalChapters <= 0) return 0
  return Math.round(((current + 1) / totalChapters) * 100)
}

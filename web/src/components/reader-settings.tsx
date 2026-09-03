// 읽기 설정 시트 (ADR-040). 읽기 라우트에 처음으로 얹히는 스크립트다.
//
// **본문은 이 파일이 없어도 렌더된다.** 여기가 하는 일은 이미 서버가 `<html>`에 얹어 둔
// 단계를 바꾸는 것뿐이고, 스크립트가 없으면 기어 버튼이 아무 일도 하지 않을 뿐이다
// (ADR-007·023의 SEO 전제는 본문이 HTML에 담겨 오는 것이지 스크립트가 없는 것이 아니다).
//
// react-query 훅도 shadcn 컴포넌트도 쓰지 않는다 — 그 두 금지는 그대로다 (ADR-035·040).
// 치수와 색은 하나도 여기 없다. 전부 `styles.css`에 있다 (ADR-038).
import { useEffect, useRef, useState } from 'react'

import {
  READER_DEFAULT,
  READER_STEPS,
  type ReaderPrefs,
  readBrowserPrefs,
  saveReaderPrefs,
} from '#/lib/reader-prefs'

/** 행간 3단계 — 줄 사이가 벌어지는 것을 아이콘이 그대로 보여준다. */
const LEADING_ICONS = ['M5 8h14M5 12h14M5 16h14', 'M5 6h14M5 12h14M5 18h14', 'M5 4h14M5 12h14M5 20h14']
const LEADING_NAMES = ['좁게', '보통', '넓게']

/** 좌우 여백 3단계 — 글줄이 짧아지는 것을 아이콘이 그대로 보여준다. */
const MARGIN_ICONS = ['M4 7h16M4 12h16M4 17h16', 'M6 7h12M6 12h12M6 17h12', 'M8 7h8M8 12h8M8 17h8']
const MARGIN_NAMES = ['좁게', '보통', '넓게']

function Lines({ d }: { d: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      aria-hidden="true"
    >
      <path d={d} />
    </svg>
  )
}

/**
 * 단계 하나를 고르는 가로 선택기. 값이 유한하므로 라디오 그룹이 맞는 의미다 —
 * 연속 슬라이더로 두지 않는 것이 ADR-040의 결정이고, 마크업도 그것을 말해야 한다.
 */
function StepPicker({
  label,
  value,
  names,
  icons,
  onPick,
}: {
  label: string
  value: number
  names: readonly string[]
  icons: readonly string[]
  onPick: (step: number) => void
}) {
  return (
    <div className="reader-set-row">
      <span className="reader-set-label">{label}</span>
      <div className="reader-set-seg" role="radiogroup" aria-label={label}>
        {icons.map((d, i) => (
          <button
            key={d}
            type="button"
            role="radio"
            aria-checked={value === i + 1}
            aria-label={names[i]}
            onClick={() => onPick(i + 1)}
          >
            <Lines d={d} />
          </button>
        ))}
      </div>
    </div>
  )
}

export function ReaderSettings() {
  const [open, setOpen] = useState(false)
  // 쿠키는 **브라우저에서만** 읽는다. 서버 렌더에는 시트가 아예 없으므로 이 값이 마크업에
  // 나가지 않고, 따라서 하이드레이션이 어긋날 자리도 없다 — 첫 페인트의 글자 크기는
  // 이미 서버가 `<html>`에 얹어 두었다 (ADR-040).
  const [prefs, setPrefs] = useState<ReaderPrefs>(() =>
    typeof document === 'undefined' ? READER_DEFAULT : readBrowserPrefs(),
  )
  const gear = useRef<HTMLButtonElement>(null)

  // 시트는 읽던 화면을 덮지 않는다. 그래서 닫는 길이 여럿 있어야 한다 — Esc·막·닫기 버튼.
  useEffect(() => {
    if (!open) return
    function onKey(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      setOpen(false)
      gear.current?.focus()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open])

  function close() {
    setOpen(false)
    gear.current?.focus()
  }

  function change(next: ReaderPrefs) {
    setPrefs(next)
    // 쿠키를 다시 굽고 `<html>`을 그 자리에서 갱신한다. 페이지는 다시 뜨지 않는다.
    saveReaderPrefs(next)
  }

  const smallest = prefs.size <= 1
  const largest = prefs.size >= READER_STEPS.size

  return (
    <>
      <button
        ref={gear}
        type="button"
        className="btn btn-icon reader-set-open"
        aria-expanded={open}
        aria-label="읽기 설정"
        onClick={() => setOpen((was) => !was)}
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.7"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
        </svg>
      </button>

      {open && (
        <>
          {/* 막은 거의 투명하다. **읽던 자리가 뒤에 그대로 남아야 한다** — 시트를 열어 놓고
              글자 크기를 바꾸는 것이 이 화면의 쓰임이라 본문이 보이지 않으면 고를 수가 없다. */}
          <div className="reader-set-veil" onClick={close} />

          <div className="reader-set-sheet" role="dialog" aria-label="읽기 설정">
            <div className="reader-set-grip" aria-hidden="true" />

            <div className="reader-set-head">
              <h2>읽기 설정</h2>
              <button
                type="button"
                className="reader-set-reset"
                onClick={() => change(READER_DEFAULT)}
              >
                기본값으로
              </button>
              <button
                type="button"
                className="btn btn-icon reader-set-close"
                aria-label="설정 닫기"
                onClick={close}
              >
                <svg
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.7"
                  strokeLinecap="round"
                  aria-hidden="true"
                >
                  <path d="M6 6l12 12M18 6L6 18" />
                </svg>
              </button>
            </div>

            {/* 글자 크기만 5단계라 -/+ 로 옮긴다. 다섯 칸을 늘어놓으면 폰에서 표적이 좁아진다. */}
            <div className="reader-set-row">
              <span className="reader-set-label">글자 크기</span>
              <div className="reader-set-size">
                <button
                  type="button"
                  className="btn btn-icon reader-set-smaller"
                  aria-label="글자 작게"
                  disabled={smallest}
                  onClick={() => change({ ...prefs, size: prefs.size - 1 })}
                >
                  가
                </button>
                <div className="reader-set-dots" aria-hidden="true">
                  {Array.from({ length: READER_STEPS.size }, (_, i) => (
                    <span key={i} data-on={prefs.size === i + 1 ? '' : undefined} />
                  ))}
                </div>
                <span className="visually-hidden" aria-live="polite">
                  글자 크기 {prefs.size} / {READER_STEPS.size} 단계
                </span>
                <button
                  type="button"
                  className="btn btn-icon reader-set-bigger"
                  aria-label="글자 크게"
                  disabled={largest}
                  onClick={() => change({ ...prefs, size: prefs.size + 1 })}
                >
                  가
                </button>
              </div>
            </div>

            <StepPicker
              label="행간"
              value={prefs.leading}
              names={LEADING_NAMES}
              icons={LEADING_ICONS}
              onPick={(leading) => change({ ...prefs, leading })}
            />

            <StepPicker
              label="여백"
              value={prefs.margin}
              names={MARGIN_NAMES}
              icons={MARGIN_ICONS}
              onPick={(margin) => change({ ...prefs, margin })}
            />

            <p className="reader-set-note">
              설정은 이 기기에만 남습니다. 읽기 방식은 스크롤로 고정입니다.
            </p>
          </div>
        </>
      )}
    </>
  )
}

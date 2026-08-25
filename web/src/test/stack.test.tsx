import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

// 이 두 줄이 이 테스트의 요점이다.
// 별칭 `#/*` → `./src/*`는 package.json의 imports와 tsconfig의 paths 양쪽에 있고
// Vite가 번들 시점에 푼다. **vitest도 같은 방식으로 풀어야 한다** —
// 여기서 어긋나면 tsc는 통과하는데 테스트만 "모듈을 찾을 수 없다"로 죽는다.
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'

describe('ADR-035 스택', () => {
  it('별칭 #/* 가 vitest에서도 풀린다', () => {
    expect(typeof cn).toBe('function')
    expect(cn('px-2', 'px-4')).toBe('px-4')
  })

  it('shadcn 버튼이 jsdom에서 렌더된다 — API도 DATABASE_URL도 없이', () => {
    render(<Button variant="secondary">승인</Button>)

    const button = screen.getByRole('button', { name: '승인' })
    expect(button).toBeInTheDocument()
    expect(button).toHaveAttribute('data-slot', 'button')
    expect(button).toHaveAttribute('data-variant', 'secondary')
  })
})

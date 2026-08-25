import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// toBeInTheDocument 같은 DOM 매처를 vitest의 expect에 붙인다.
// 타입 확장도 이 import가 가져온다.
import '@testing-library/jest-dom/vitest'

// globals: false라 testing-library의 자동 정리가 걸리지 않는다. 직접 건다.
afterEach(() => {
  cleanup()
})

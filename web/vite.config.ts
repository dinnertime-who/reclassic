import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, relative } from 'node:path'

import { defineConfig } from 'vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { nitro } from 'nitro/vite'

const require = createRequire(import.meta.url)

// Fontsource가 주는 CSS는 두 가지가 우리 결정과 다르다.
//
// 1. `font-display: swap` 이 박혀 있다. **ADR-039가 요구한 것은 `optional`이다** —
//    첫 페인트를 폰트가 붙잡지 못하게 하는 것이 ADR-038이 웹폰트를 다시 여는 조건이었다.
//    `@font-face` 디스크립터는 캐스케이드로 덮이지 않는다. 같은 규칙 124벌을 다시 쓰는 것이
//    유일한 대안이라 여기서 고친다.
// 2. woff2 옆에 woff 폴백을 같이 싣는다. `unicode-range`를 이해하는 브라우저는 예외 없이
//    woff2를 읽으므로 **아무도 받지 않는 파일이다.** 그대로 두면 빌드 산출물이
//    4.3MB에서 10.9MB로 는다.
//
// styles.css에서 `@import` 하지 않는 이유가 이것이다 — 거기 두면 postcss-import가 먼저
// 디스크에서 읽어 인라인해 버려서 이 훅이 파일을 보지 못한다. `enforce: 'pre'` 라
// Vite의 CSS 파이프라인보다 앞서 돌고, url()은 뒤이어 Vite가 해시 자산으로 바꾼다.
function fontsourceDisplayOptional() {
  const FONT_CSS = /(^|\/)src\/fonts\.css$/

  return {
    name: 'reclassic:fontsource-display-optional',
    enforce: 'pre' as const,
    transform(code: string, id: string) {
      const file = id.split('?')[0]
      if (!FONT_CSS.test(file)) return null

      const base = dirname(file)
      const expanded = code.replace(
        /@import\s+['"]([^'"]+)['"];/g,
        (_match, spec: string) => {
          const resolved = require.resolve(spec, { paths: [base] })
          // url(./files/…) 는 그 CSS 기준의 상대 경로다. fonts.css 기준으로 다시 쓴다.
          const prefix = relative(base, dirname(resolved))
          return readFileSync(resolved, 'utf8')
            .replace(/font-display:\s*swap/g, 'font-display: optional')
            .replace(/,\s*url\([^)]+\)\s*format\('woff'\)/g, '')
            .replace(/url\(\.\//g, `url(${prefix}/`)
        },
      )
      return { code: expanded, map: null }
    },
  }
}

export default defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [
    fontsourceDisplayOptional(),
    nitro(),
    tailwindcss(),
    tanstackStart(),
    viteReact(),
  ],
})

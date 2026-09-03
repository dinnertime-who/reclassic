import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'

// flat config. `make lint`가 tsc 뒤에 이것을 돌린다.
// 타입 정보가 필요한 규칙은 켜지 않는다 — 타입 검사는 이미 tsc가 한다.
export default tseslint.config(
  {
    // 생성 코드는 손으로 고치지 않으므로 린트하지 않는다 (작업 규칙 4).
    // shadcn 산출물은 여기 없다 — 그건 우리 소스다 (ADR-035).
    ignores: [
      'node_modules/**',
      'dist/**',
      '.output/**',
      '.nitro/**',
      '.tanstack/**',
      'src/api/gen/**',
      'src/routeTree.gen.ts',
    ],
  },
  tseslint.configs.recommended,
  reactHooks.configs.flat['recommended-latest'],
  {
    // 읽기 라우트 순수성 가드. 여기에 react-query 훅이나 shadcn 컴포넌트가 들어가면
    // "자바스크립트 없이 뜨는" 성질이 깨지는데 SSR은 멀쩡해 보여서 배포는 성공으로 찍힌다
    // — ADR-007·023의 SEO 전제가 여기 걸려 있다 (ADR-035, AGENTS.md "절대 깨지 말 것").
    // 편집 라우트(`*.edit.tsx`)는 CSR이므로 대상이 아니다. `__root.tsx`도 아니다 — 타입만 연다.
    files: [
      'src/routes/index.tsx',
      'src/routes/books.index.tsx',
      'src/routes/books.*.tsx',
      'src/routes/projects.$projectId.chapters.$idx.tsx',
    ],
    ignores: ['src/routes/*.edit.tsx'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@tanstack/react-query*',
                '@tanstack/react-query*/**',
                '@tanstack/react-router-ssr-query',
              ],
              message:
                '읽기 라우트는 로더로만 데이터를 받는다. 훅을 쓰면 JS 없이 뜨지 않는다 (ADR-035·007·023).',
            },
            {
              // `group`은 gitignore 문법으로 해석된다. `#`으로 시작하면 주석으로 먹혀
              // 규칙이 조용히 사라지므로 백슬래시로 escape 한다.
              group: ['\\#/components/ui/*'],
              message:
                '읽기 라우트에 shadcn 컴포넌트를 넣지 않는다 — 클라이언트 렌더에 기댄다 (ADR-035).',
            },
          ],
        },
      ],
    },
  },
  {
    files: ['**/*.{ts,tsx}'],
    rules: {
      // 별칭 `#/*`는 Vite가 번들 시점에 푼다. 린터가 모듈 해석에 관여하지 않게 둔다.
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
    },
  },
)

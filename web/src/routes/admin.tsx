// 관리자 화면 레이아웃 (CSR) — /admin
//
// 편집·검수와 같다. react-query 훅, shadcn, 뮤테이션 뒤 무효화.
// 낙관적 갱신은 하지 않는다 (ADR-035). 읽기 화면이 아니다.
import { Link, Outlet, createFileRoute, useLoaderData } from '@tanstack/react-router'

import { ApiError } from '#/api/http'
import type { CurrentUser, UserListItem } from '#/api/gen/model'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'

export const Route = createFileRoute('/admin')({
  component: AdminLayout,
})

export type Notice = { tone: 'info' | 'error'; text: string }

export function statusOf(err: unknown): number | undefined {
  return err instanceof ApiError ? err.status : undefined
}

export const LOGIN_REQUIRED =
  '로그인이 필요합니다. 위 세션 바에서 로그인한 뒤 다시 시도하세요.'

/** ADR-014 합본 게이트. 화면에 숫자와 같이 보여 판단할 수 있게 한다. */
export const CHAPTER_LIMIT = 200
export const PARAGRAPH_LIMIT = 15_000

export function isAdmin(user: CurrentUser | null | undefined): boolean {
  return user?.role === 'admin'
}

/** 화면이 권한을 판정하지 않는다 — 서버가 준 role을 반영할 뿐이다.
 *  admin 부여는 계약에 없고, 자기 자신 강등은 서버가 409로 거절한다.
 *  버튼을 아예 안 그리는 것은 사용자를 함정에 빠뜨리지 않기 위해서다. */
export function canChangeRole(
  actor: CurrentUser | null | undefined,
  target: UserListItem,
): boolean {
  if (!isAdmin(actor)) return false
  if (target.role === 'admin') return false
  if (actor?.handle === target.handle) return false
  return true
}

export function formatCount(n: number): string {
  return n.toLocaleString('ko-KR')
}

export function formatCreatedAt(iso: string): string {
  const at = new Date(iso)
  return Number.isNaN(at.getTime())
    ? iso
    : at.toLocaleString('ko-KR', { dateStyle: 'medium', timeStyle: 'short' })
}

export function ScreenAlert({
  tone,
  title,
  children,
}: {
  tone: Notice['tone']
  title: string
  children?: React.ReactNode
}) {
  return (
    <Alert variant={tone === 'error' ? 'destructive' : 'default'} className="mt-4">
      <AlertTitle>{title}</AlertTitle>
      {children ? <AlertDescription>{children}</AlertDescription> : null}
    </Alert>
  )
}

export function listErrorNotice(err: unknown, fallback: string): Notice {
  switch (statusOf(err)) {
    case 401:
      return { tone: 'error', text: LOGIN_REQUIRED }
    case 403:
      return { tone: 'error', text: '관리 권한이 없습니다.' }
    default:
      return { tone: 'error', text: fallback }
  }
}

function AdminLayout() {
  const user = useLoaderData({ from: '__root__' })
  const loginUrl = import.meta.env.VITE_LOGIN_URL

  return (
    <main>
      <h1 className="text-2xl font-semibold">관리</h1>

      {!user ? (
        <ScreenAlert tone="info" title="로그인이 필요합니다">
          관리 화면은 로그인 뒤에 열립니다.{' '}
          {loginUrl ? <a href={loginUrl}>Google로 로그인</a> : '위 세션 바에서 로그인하세요.'}
        </ScreenAlert>
      ) : !isAdmin(user) ? (
        <ScreenAlert tone="info" title="관리 조작은 표시되지 않습니다">
          서버가 이 계정에 관리 권한을 주지 않았습니다. 화면이 권한을 판정하지 않습니다.
        </ScreenAlert>
      ) : (
        <>
          <nav className="mt-4 flex gap-4 text-sm">
            <Link to="/admin" className="underline underline-offset-4">
              확인 큐
            </Link>
            <Link to="/admin/users" className="underline underline-offset-4">
              역할
            </Link>
            <Link to="/admin/projects" className="underline underline-offset-4">
              공개
            </Link>
          </nav>
          <Outlet />
        </>
      )}
    </main>
  )
}

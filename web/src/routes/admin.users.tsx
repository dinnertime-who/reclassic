// 역할 부여 (CSR) — member ↔ reviewer 만. admin은 선택지에도 없다 (ADR-027).
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useLoaderData } from '@tanstack/react-router'

import {
  getListUsersQueryKey,
  useListUsers,
  useSetUserRole,
} from '#/api/gen/reclassic'
import type {
  CurrentUser,
  UserListItem,
  UserListItemRole,
  UserRoleInputRole,
} from '#/api/gen/model'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import {
  LOGIN_REQUIRED,
  ScreenAlert,
  canChangeRole,
  listErrorNotice,
  statusOf,
  type Notice,
} from '#/routes/admin'

export const Route = createFileRoute('/admin/users')({
  component: AdminUsers,
})

const ROLE_LABEL: Record<UserListItemRole, string> = {
  member: 'member',
  reviewer: 'reviewer',
  admin: 'admin',
}

export function roleNotice(err: unknown): Notice {
  switch (statusOf(err)) {
    case 401:
      return { tone: 'error', text: LOGIN_REQUIRED }
    case 403:
      return { tone: 'error', text: '관리 권한이 없습니다.' }
    case 404:
      return { tone: 'error', text: '그 사용자를 찾을 수 없습니다.' }
    case 409:
      return { tone: 'error', text: '자기 자신의 역할은 바꿀 수 없습니다.' }
    default:
      return { tone: 'error', text: '역할을 바꾸지 못했습니다.' }
  }
}

function AdminUsers() {
  const actor = useLoaderData({ from: '__root__' })
  const queryClient = useQueryClient()
  const [notice, setNotice] = useState<Notice | null>(null)
  const usersQuery = useListUsers()

  const setRole = useSetUserRole({
    mutation: {
      onSuccess: async () => {
        setNotice(null)
        await queryClient.invalidateQueries({ queryKey: getListUsersQueryKey() })
      },
      onError: (err) => setNotice(roleNotice(err)),
    },
  })

  async function changeRole(userId: number, role: UserRoleInputRole) {
    setNotice(null)
    try {
      await setRole.mutateAsync({ userId, data: { role } })
    } catch {
      // 뜻은 onError가 notice에 담았다.
    }
  }

  return (
    <div className="mt-6 space-y-4">
      <h2 className="text-xl font-semibold">역할 부여</h2>
      <p className="text-sm text-muted-foreground">
        member와 reviewer만 바꿀 수 있습니다. admin은 ADMIN_EMAIL로만 정해집니다
        (ADR-027). 이 화면의 선택지에도 없습니다.
      </p>

      {notice && (
        <ScreenAlert
          tone={notice.tone}
          title={notice.tone === 'error' ? '처리하지 못했습니다' : '알림'}
        >
          {notice.text}
        </ScreenAlert>
      )}

      {usersQuery.isPending ? (
        <p className="text-sm text-muted-foreground">불러오는 중…</p>
      ) : usersQuery.isError ? (
        <ScreenAlert tone="error" title="불러오지 못했습니다">
          {listErrorNotice(usersQuery.error, '사용자 목록을 불러오지 못했습니다.').text}
        </ScreenAlert>
      ) : (
        <UserList
          items={usersQuery.data.status === 200 ? usersQuery.data.data.items : []}
          actor={actor}
          pending={setRole.isPending}
          onChangeRole={changeRole}
        />
      )}
    </div>
  )
}

function UserList({
  items,
  actor,
  pending,
  onChangeRole,
}: {
  items: UserListItem[]
  actor: CurrentUser | null | undefined
  pending: boolean
  onChangeRole: (userId: number, role: UserRoleInputRole) => void
}) {
  if (items.length === 0) {
    return <p className="text-sm text-muted-foreground">아직 사용자가 없습니다.</p>
  }

  return (
    <ul className="list-none space-y-3 p-0">
      {items.map((user) => (
        <li key={user.id}>
          <UserRow
            user={user}
            changeable={canChangeRole(actor, user)}
            isSelf={user.handle === actor?.handle}
            pending={pending}
            onChangeRole={onChangeRole}
          />
        </li>
      ))}
    </ul>
  )
}

export function UserRow({
  user,
  changeable,
  isSelf,
  pending,
  onChangeRole,
}: {
  user: UserListItem
  changeable: boolean
  isSelf: boolean
  pending: boolean
  onChangeRole: (userId: number, role: UserRoleInputRole) => void
}) {
  const nextRole: UserRoleInputRole | null =
    user.role === 'member' ? 'reviewer' : user.role === 'reviewer' ? 'member' : null

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex flex-wrap items-center gap-2">
          <span>{user.displayName}</span>
          <Badge variant={user.role === 'admin' ? 'secondary' : 'outline'}>
            {ROLE_LABEL[user.role]}
          </Badge>
          <span className="text-sm font-normal text-muted-foreground">{user.handle}</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {changeable && nextRole ? (
          <Button
            type="button"
            size="sm"
            variant={nextRole === 'member' ? 'outline' : 'default'}
            disabled={pending}
            onClick={() => onChangeRole(user.id, nextRole)}
          >
            {nextRole}로
          </Button>
        ) : isSelf ? (
          <p className="my-0 text-sm text-muted-foreground">
            자기 자신의 역할은 바꿀 수 없습니다.
          </p>
        ) : user.role === 'admin' ? (
          <p className="my-0 text-sm text-muted-foreground">
            admin은 ADMIN_EMAIL로만 정해집니다.
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}

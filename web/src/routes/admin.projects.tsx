// 프로젝트 공개 전이 (CSR) — open ↔ published. archived는 이 슬라이스의 전이가 아니다.
// published_at은 내려와도 남는다 (ADR-036). 화면이 그 사실을 지우지 않는다.
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import {
  getListAdminProjectsQueryKey,
  useListAdminProjects,
  useSetProjectStatus,
} from '#/api/gen/reclassic'
import type {
  ProjectListItem,
  ProjectListItemStatus,
  ProjectStatusInputStatus,
} from '#/api/gen/model'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import {
  LOGIN_REQUIRED,
  ScreenAlert,
  formatCreatedAt,
  listErrorNotice,
  statusOf,
  type Notice,
} from '#/routes/admin'

export const Route = createFileRoute('/admin/projects')({
  component: AdminProjects,
})

const STATUS_LABEL: Record<ProjectListItemStatus, string> = {
  open: '작업 중',
  published: '공개',
  archived: '보관',
}

export function canTogglePublish(project: ProjectListItem): boolean {
  return project.status === 'open' || project.status === 'published'
}

export function statusNotice(err: unknown): Notice {
  switch (statusOf(err)) {
    case 401:
      return { tone: 'error', text: LOGIN_REQUIRED }
    case 403:
      return { tone: 'error', text: '관리 권한이 없습니다.' }
    case 404:
      return { tone: 'error', text: '그 프로젝트를 찾을 수 없습니다.' }
    default:
      return { tone: 'error', text: '공개 상태를 바꾸지 못했습니다.' }
  }
}

function AdminProjects() {
  const queryClient = useQueryClient()
  const [notice, setNotice] = useState<Notice | null>(null)
  const projectsQuery = useListAdminProjects()

  const setStatus = useSetProjectStatus({
    mutation: {
      onSuccess: async () => {
        setNotice(null)
        await queryClient.invalidateQueries({ queryKey: getListAdminProjectsQueryKey() })
      },
      onError: (err) => setNotice(statusNotice(err)),
    },
  })

  async function changeStatus(projectId: number, status: ProjectStatusInputStatus) {
    setNotice(null)
    try {
      await setStatus.mutateAsync({ projectId, data: { status } })
    } catch {
      // 뜻은 onError가 notice에 담았다.
    }
  }

  return (
    <div className="mt-6 space-y-4">
      <h2 className="text-xl font-semibold">프로젝트 공개</h2>
      <p className="text-sm text-muted-foreground">
        published의 의미는 하나다 — 도서 목록에 노출되는가 (ADR-036). 관리자에게는
        open도 보여야 공개할 것을 고를 수 있다. 처음 공개된 시각은 내려와도 남는다.
      </p>

      {notice && (
        <ScreenAlert
          tone={notice.tone}
          title={notice.tone === 'error' ? '처리하지 못했습니다' : '알림'}
        >
          {notice.text}
        </ScreenAlert>
      )}

      {projectsQuery.isPending ? (
        <p className="text-sm text-muted-foreground">불러오는 중…</p>
      ) : projectsQuery.isError ? (
        <ScreenAlert tone="error" title="불러오지 못했습니다">
          {listErrorNotice(projectsQuery.error, '프로젝트 목록을 불러오지 못했습니다.').text}
        </ScreenAlert>
      ) : (
        <ProjectList
          items={projectsQuery.data.status === 200 ? projectsQuery.data.data.items : []}
          pending={setStatus.isPending}
          onChangeStatus={changeStatus}
        />
      )}
    </div>
  )
}

function ProjectList({
  items,
  pending,
  onChangeStatus,
}: {
  items: ProjectListItem[]
  pending: boolean
  onChangeStatus: (projectId: number, status: ProjectStatusInputStatus) => void
}) {
  if (items.length === 0) {
    return <p className="text-sm text-muted-foreground">아직 번역 프로젝트가 없습니다.</p>
  }

  return (
    <ul className="list-none space-y-3 p-0">
      {items.map((project) => (
        <li key={project.id}>
          <ProjectRow
            project={project}
            pending={pending}
            onChangeStatus={onChangeStatus}
          />
        </li>
      ))}
    </ul>
  )
}

export function ProjectRow({
  project,
  pending,
  onChangeStatus,
}: {
  project: ProjectListItem
  pending: boolean
  onChangeStatus: (projectId: number, status: ProjectStatusInputStatus) => void
}) {
  const toggleable = canTogglePublish(project)
  const nextStatus: ProjectStatusInputStatus | null =
    project.status === 'open'
      ? 'published'
      : project.status === 'published'
        ? 'open'
        : null

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex flex-wrap items-center gap-2">
          <span>{project.title}</span>
          <Badge variant={project.status === 'published' ? 'default' : 'outline'}>
            {STATUS_LABEL[project.status]}
          </Badge>
          <span className="text-sm font-normal text-muted-foreground">
            {project.targetLang}
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2 text-sm">
        {project.author ? <p className="my-0 text-muted-foreground">{project.author}</p> : null}
        <p className="my-0 text-muted-foreground">
          {project.publishedAt
            ? `처음 공개: ${formatCreatedAt(project.publishedAt)}`
            : '아직 공개된 적 없습니다'}
        </p>
        {toggleable && nextStatus ? (
          <Button
            type="button"
            size="sm"
            variant={nextStatus === 'open' ? 'outline' : 'default'}
            disabled={pending}
            onClick={() => onChangeStatus(project.id, nextStatus)}
          >
            {nextStatus === 'published' ? '도서 목록에 공개' : '목록에서 내리기'}
          </Button>
        ) : (
          <p className="my-0 text-muted-foreground">이 슬라이스에서 옮길 수 없는 상태입니다.</p>
        )}
      </CardContent>
    </Card>
  )
}

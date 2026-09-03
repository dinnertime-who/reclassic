import { Link, createFileRoute } from '@tanstack/react-router'

// 정문이다. 슬라이스 1에서는 `/healthz`의 값을 그대로 나열했는데,
// 그건 배포 커밋 해시까지 공개 화면에 내보내는 디버그 화면이었다 (ADR-038).
// 지금은 서비스가 무엇인지만 말한다 — API를 부르지 않으므로 로더도 없다.
export const Route = createFileRoute('/')({
  head: () => ({
    meta: [
      { title: 'reclassic — 퍼블릭 도메인 고전을 함께 옮깁니다' },
      {
        name: 'description',
        content:
          'Project Gutenberg의 퍼블릭 도메인 고전을 문단 단위로 나누고, 제안과 검수를 거쳐 문단마다 확정된 번역 하나를 공개합니다.',
      },
    ],
  }),
  component: Home,
})

function Home() {
  return (
    <main>
      <div className="hero">
        <h1>퍼블릭 도메인 고전을, 문단 하나씩 함께 옮깁니다</h1>
        <p>
          Project Gutenberg의 원문을 장과 문단으로 나눕니다. 누구나 문단마다 번역을
          제안하고, 검수를 거친 하나가 그 문단의 확정본이 됩니다.
        </p>
        <div className="hero-actions">
          <Link to="/books" className="btn btn-primary">
            도서 목록 보기
          </Link>
        </div>
      </div>

      <ol className="steps">
        <li>
          <span className="step-mark">01</span>
          <h2>원문을 나눕니다</h2>
          <p>
            책을 장과 문단으로 쪼개고, 문단마다 본문에서 뽑은 고유한 키를 붙입니다.
            원문이 다시 처리돼도 쌓인 번역은 같은 문단에 그대로 남습니다.
          </p>
        </li>
        <li>
          <span className="step-mark">02</span>
          <h2>번역을 제안합니다</h2>
          <p>
            로그인한 사람은 어느 문단에든 번역을 제안할 수 있습니다. 한 문단에 여러
            제안이 쌓여도 괜찮습니다 — 고르는 것은 다음 단계입니다.
          </p>
        </li>
        <li>
          <span className="step-mark">03</span>
          <h2>검수가 하나를 확정합니다</h2>
          <p>
            검수자가 제안 하나를 승인하면 그것이 문단의 확정본이 됩니다. 문단당
            확정본은 언제나 하나입니다.
          </p>
        </li>
      </ol>
    </main>
  )
}

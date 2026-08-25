# reclassic

Project Gutenberg의 퍼블릭 도메인 도서를 장·문단 단위로 분해하고,
사용자가 문단별 번역을 제안하면 검수를 거쳐 **문단당 확정본 1개**를 공개하는 서비스.

**떠 있는 주소** — 웹 [reclassic.dinnertimes.app](https://reclassic.dinnertimes.app) ·
API [api-reclassic.dinnertimes.app](https://api-reclassic.dinnertimes.app)

## 시작하기

```bash
make doctor   # 개발 환경 점검
make help     # 사용 가능한 명령

make dev      # Postgres + MinIO
make migrate  # 스키마
make run-api  # :8080
make run-web  # :3100 (SSR)
```

`.env`에 `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`이 있어야 API가 기동한다.
Google Cloud Console에서 OAuth 클라이언트를 만들고, 승인된 리디렉션 URI에
`GOOGLE_REDIRECT_URL` 값을 그대로 등록한다.

로컬에 도서를 채우려면:

```bash
make fetch-corpus   # 검증용 도서 내려받기
make ingest         # 파싱해 DB에 적재 (멱등)
# http://localhost:3100/books/1342/chapters/5
```

## 문서

| 문서 | 내용 |
|---|---|
| [`AGENTS.md`](AGENTS.md) | **작업 지침 — 사람과 AI 모두 여기서 시작** |
| [`docs/index.md`](docs/index.md) | 지식 베이스 전체 지도 |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 시스템 구조, 데이터 모델, 핵심 불변식 |
| [`docs/decisions/index.md`](docs/decisions/index.md) | 설계 결정 이력 (ADR) |
| [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) | 코딩 규약, 기여 흐름 |
| [`docs/slices/index.md`](docs/slices/index.md) | 진행 상황 — 무엇이 끝났고 무엇을 하는 중인지 |
| [`docs/tech-debt.md`](docs/tech-debt.md) | 알면서 남겨둔 것 |

**진행 상황은 [`docs/slices/index.md`](docs/slices/index.md) 한 곳에만 적는다.**
여기에도 적으면 반드시 갈라진다.

## AI 에이전트 지침

여러 AI 도구를 함께 쓰기 때문에, 지침은 **`AGENTS.md` 하나로 통일**한다.
Claude Code, Codex, Cursor, Augment 등이 공통으로 읽는 파일이다.

도구별 지침 파일(`CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md` 등)은
**만들지 않는다.** 내용이 갈라지는 순간 도구마다 다르게 동작하게 된다.

`AGENTS.md`는 **지도이지 백과사전이 아니다.** 세부는 `docs/`에 두고 거기서 링크한다.
`make docs-check`가 이 구조를 기계적으로 검사한다.

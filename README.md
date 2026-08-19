# reclassic

Project Gutenberg의 퍼블릭 도메인 도서를 장·문단 단위로 분해하고,
사용자가 문단별 번역을 제안하면 검수를 거쳐 확정본을 공개하는 서비스.

## 시작하기

```bash
make doctor   # 개발 환경 점검
make help     # 사용 가능한 명령
```

## 문서

| 문서 | 내용 |
|---|---|
| [`AGENTS.md`](AGENTS.md) | **작업 지침 — 사람과 AI 모두 여기서 시작** |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 시스템 구조, 데이터 모델, 핵심 불변식 |
| [`docs/DECISIONS.md`](docs/DECISIONS.md) | 설계 결정 이력 (ADR) |
| [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) | 코딩 규약 |
| [`docs/SPIKE_PARSER.md`](docs/SPIKE_PARSER.md) | 파서 검증 스파이크 명세 |
| [`docs/PARSER_REPORT.md`](docs/PARSER_REPORT.md) | 파서 검증 결과와 권고 |
| [`docs/SLICE_SKELETON.md`](docs/SLICE_SKELETON.md) | 프로젝트 골격 명세 (완료) |
| [`docs/SLICE_READ_PATH.md`](docs/SLICE_READ_PATH.md) | 읽기 경로 명세 (완료) |

## AI 에이전트 지침

여러 AI 도구를 함께 쓰기 때문에, 지침은 **`AGENTS.md` 하나로 통일**한다.
Claude Code, Codex, Cursor, Augment 등이 공통으로 읽는 파일이다.

도구별 지침 파일(`CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md` 등)은
**만들지 않는다.** 내용이 갈라지는 순간 도구마다 다르게 동작하게 된다.

## 현재 상태

**파서 검증 스파이크 완료** — [`docs/PARSER_REPORT.md`](docs/PARSER_REPORT.md).
공식 신뢰도는 22/22 자동 구간이고 본문 커버리지는 높다.
파서 후처리(ADR-013)와 `stable_id` 등장 순서(ADR-016)는 구현·검증이 끝났다.

**골격 슬라이스 완료** — [`docs/SLICE_SKELETON.md`](docs/SLICE_SKELETON.md).
goose · sqlc · oapi-codegen · orval · docker compose · TanStack Start이 한 저장소에서 맞물려 돈다.
`GET /healthz` 하나가 Go API에서 SSR 화면까지 흐르고, `openapi.yaml`을 고치면
Go와 TS 양쪽이 컴파일 에러를 낸다(ADR-009 검증).

```bash
make dev        # Postgres
make migrate    # 스키마
make run-api    # :8080
make run-web    # :3000 (SSR)
```

**읽기 경로 슬라이스 완료** — [`docs/SLICE_READ_PATH.md`](docs/SLICE_READ_PATH.md).
22권이 DB에 적재되고 golden과 수가 일치한다. 책 한 권이 자바스크립트 없이 브라우저에 뜬다.

```bash
make ingest       # 캐시된 22권 적재 (멱등)
make succession   # stable_id 승계율 측정
# http://localhost:3000/books/1342/chapters/5
```

**`stable_id` 승계율은 파서 미변경 상태에서 21권 37,125문단 100%다.**
ADR-004가 전제한 "해시 일치 → 자동 승계"가 처음으로 수치로 확인됐다.

**다음은 수집 자동화(River + R2)이고, 그다음이 번역이다.**

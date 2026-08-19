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
| [`docs/SLICE_SKELETON.md`](docs/SLICE_SKELETON.md) | 현재 작업 — 프로젝트 골격 명세 |
| [`docs/SLICE_READ_PATH.md`](docs/SLICE_READ_PATH.md) | 다음 작업 — 읽기 경로 명세 |

## AI 에이전트 지침

여러 AI 도구를 함께 쓰기 때문에, 지침은 **`AGENTS.md` 하나로 통일**한다.
Claude Code, Codex, Cursor, Augment 등이 공통으로 읽는 파일이다.

도구별 지침 파일(`CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md` 등)은
**만들지 않는다.** 내용이 갈라지는 순간 도구마다 다르게 동작하게 된다.

## 현재 상태

**파서 검증 스파이크 결과** — [`docs/PARSER_REPORT.md`](docs/PARSER_REPORT.md).

공식 신뢰도는 22/22 자동 구간이고 본문 커버리지는 높다.
파서 후처리(ADR-013)와 `stable_id` 등장 순서(ADR-016)는 구현·검증이 끝났다.

**다음은 프로젝트 골격이다** — [`docs/SLICE_SKELETON.md`](docs/SLICE_SKELETON.md).
지금 서 있는 것은 파서뿐이라, 도구 체인이 맞물려 도는지 엔드포인트 하나로 먼저 확인한다.
그다음이 읽기 경로([`SLICE_READ_PATH.md`](docs/SLICE_READ_PATH.md)) — 적재부터 SSR 화면까지 한 번에 —
그리고 수집 자동화, 번역 순이다.

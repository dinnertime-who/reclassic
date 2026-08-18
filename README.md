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
| [`docs/SPIKE_PARSER.md`](docs/SPIKE_PARSER.md) | 파서 검증 스파이크 명세 (진행 중) |

## AI 에이전트 지침

여러 AI 도구를 함께 쓰기 때문에, 지침은 **`AGENTS.md` 하나로 통일**한다.
Claude Code, Codex, Cursor, Augment 등이 공통으로 읽는 파일이다.

도구별 지침 파일(`CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md` 등)은
**만들지 않는다.** 내용이 갈라지는 순간 도구마다 다르게 동작하게 된다.

## 현재 상태

**파서 검증 스파이크 진행 중** — 명세는 [`docs/SPIKE_PARSER.md`](docs/SPIKE_PARSER.md).

Gutenberg HTML의 자동 장·문단 분리 성공률을 먼저 측정한다.
그 결과가 관리자 보정 UI의 범위와 스키마 구조를 결정하므로,
스파이크가 끝나기 전에는 DB·API·웹 코드를 작성하지 않는다. 배경은 ADR-011 참조.

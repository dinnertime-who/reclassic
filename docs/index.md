# 지식 베이스

**`AGENTS.md`가 지도이고, 여기가 그 지도의 목적지 목록이다.**
전부 읽지 않는다. 지금 하는 일에 해당하는 것만 연다.

## 무엇을 하려는가 → 무엇을 읽는가

| 하려는 일 | 읽을 것 |
|---|---|
| **코드를 쓰기 전** | [ARCHITECTURE.md](ARCHITECTURE.md) — 특히 "핵심 불변식" 절. 여기만은 건너뛰지 않는다 |
| 코드 스타일·브랜치·PR | [CONVENTIONS.md](CONVENTIONS.md) |
| "왜 이렇게 돼 있지?" | [decisions/index.md](decisions/index.md) — ADR 목차. 해당 번호만 연다 |
| 지금 무슨 작업 중인지 | [slices/index.md](slices/index.md) |
| 알려진 구멍·미검증·부채 | [tech-debt.md](tech-debt.md) |
| 파서를 건드릴 때 | [references/parser-report.md](references/parser-report.md) — 파서 판단의 근거가 되는 수치 |
| 배포·Railway 조작 | [slices/completed/deploy.md](slices/completed/deploy.md) — §5 CLI, §6 사람이 직접 |

## 구조

```
AGENTS.md                    지도. 100줄 남짓. 여기서 시작한다
docs/
├── index.md                 이 파일
├── ARCHITECTURE.md          시스템 구조, 데이터 모델, 핵심 불변식
├── CONVENTIONS.md           코딩 규약, 기여 흐름
├── tech-debt.md             알면서 남겨둔 것
├── decisions/               ADR — 하나당 파일 하나
│   ├── index.md             목차
│   └── ADR-001.md …
├── slices/                  작업 단위. 명세는 착수 전에 쓴다
│   ├── index.md             진행 상황
│   ├── active/              지금 하는 것
│   └── completed/           끝난 것
└── references/              한 번 만들고 계속 참조하는 자료
    ├── parser-report.md     파서 검증 결과와 권고
    └── spike-parser.md      파서 검증 스파이크 명세
```

## 규칙

- **에이전트가 실행 중에 읽을 수 없는 것은 존재하지 않는 것이다.**
  결정이 대화나 사람 머릿속에만 있으면 다음 작업에서 사라진다. 저장소에 넣는다.
- **한 사실은 한 곳에만 둔다.** 같은 내용을 두 문서에 적으면 반드시 갈라진다.
  요약이 필요하면 원본을 링크한다.
- **`AGENTS.md`는 백과사전이 아니라 목차다.** 세부는 여기 문서들로 내린다.
- **문서 구조는 `make docs-check`가 기계적으로 검사한다.** 링크가 끊기거나
  ADR 번호가 비면 실패한다.

# 기술 부채 추적기

**알면서 남겨둔 것만 적는다.** 모르는 문제는 여기 없다.

각 항목은 **언제 아플 것인가**를 함께 적는다. 그것이 없으면 목록이 그냥 길어지기만 한다.
해결하면 줄을 지우지 말고 "해결" 절로 옮긴다 — 왜 아팠는지가 다음 판단의 근거다.

---

## 결정은 있는데 코드가 없는 것

ADR이나 스키마가 이미 정한 것 중 구현이 비어 있는 자리다.

| | 무엇 | 근거 | 지금 상태 | 언제 아픈가 |
|---|---|---|---|---|
| D6 | **`book_glossary`가 테이블뿐이다** | [ADR-010](decisions/ADR-010.md) | 쿼리 0건 | **의도된 미구현.** 여러 사람이 같은 책을 번역해 인명·호칭이 갈리기 시작할 때 |
| D7 | **문단마다 "내 제안 상태"를 주는 필드가 없다** | [ARCHITECTURE.md](ARCHITECTURE.md) 스케치의 `my_proposal_status` | `TranslatedParagraph`는 `stableId`·`sourceText`·`approvedTranslation`·`proposalCount`뿐이다. 인증이 없어 [translation.md](slices/completed/translation.md)에서 미뤘고 아직 계약에 없다 | **이미 아프다.** 편집 화면이 **펼친 문단의 제안만 부른다**([editor.md](slices/completed/editor.md) §4.5) — 전부 부르면 챕터 하나에 요청이 수백 개다. 내 제안이 어디에 있는지 한눈에 볼 방법이 없다 |

## 만들어 놓고 부르지 않는 것

| | 무엇 | 언제 아픈가 |
|---|---|---|
| U1 | **`DeleteExpiredSessions`를 아무도 부르지 않는다** | 세션 수명 30일·갱신 없음([ADR-027](decisions/ADR-027.md))인데 만료 행이 영구히 쌓인다. 당장은 무해하고 사용자가 늘면 아프다 |
| U2 | `CountBooks`·`ChapterCoverage` 미사용 | 슬라이스 8에서 다시 봤다. **못 쓴다.** `CountBooks`는 전체 `books` 행 수인데, 공개 목록은 published 프로젝트와의 조인이고 페이지네이션도 없다(ADR-037) — `len(items)`가 곧 총량이다. `ChapterCoverage`는 챕터 단위 **번역** 커버리지(ADR-023)인데, `needs_review` 큐가 필요한 숫자는 책 단위 **파싱** 챕터·문단 수(ADR-014)다. 다른 숫자라 갖다 붙이면 게이트 판단이 틀린다 |

## 검증하지 못한 것

**프로덕션에서 확인할 성질이 아니다.** 별도 환경이 생길 때 함께 본다.

| | 무엇 | 근거 |
|---|---|---|
| V1 | **마이그레이션 실패 게이트** — 실패하면 새 버전이 안 뜨는지 | [ADR-030](decisions/ADR-030.md), [deploy.md](slices/completed/deploy.md) §8 |
| V2 | **파서 메모리 피크** — 셰익스피어 전집 파싱이 힙 310MB를 쓴다는 수치 | [ADR-029](decisions/ADR-029.md), [deploy.md](slices/completed/deploy.md) §4.2 |

## 도구·파이프라인

| | 무엇 | 언제 아픈가 |
|---|---|---|
| T1 | **`make lint`가 `golangci-lint` 없이 `go vet`으로 대체된다** | 의도한 강도가 아니다. **CI를 넣을 때 린터 버전 고정을 첫 항목으로 둔다** |
| T2 | **CI가 없다** | 지금은 범위 밖. `make lint && make test`가 사람 손에 달려 있다 |
| T3 | **`make fmt`의 web 몫이 사실상 비어 있다** — 슬라이스 7에서 `eslint . --fix`를 넣어 Go 전용은 아니게 됐지만, **ESLint 9+ 코어에는 포매팅 규칙이 없어 실제로 재포맷되는 것이 없다.** 정석은 prettier인데 [ADR-035](decisions/ADR-035.md)가 고정한 스택 밖이라 **의존성을 늘리는 결정은 `Proposed` ADR 감이다**(AGENTS.md 작업 규칙 2). 슬라이스 7에서는 넣지 않기로 정했다 | **이미 아프다.** web 코드가 늘어나는데 포맷 기준이 사람 에디터 설정에 달려 있다. **`make fmt`가 성공해서 포맷 게이트가 있다고 착각한다** — 실제로는 비어 있다 |
| T4 | **`web-install`의 재설치 판정은 mtime 스탬프다** — `web/node_modules/.install-stamp`가 `pnpm-lock.yaml`·`package.json`보다 오래되면 다시 깐다. 그래서 **lockfile은 그대로인 채 `node_modules` 안만 망가진 상태는 잡지 못한다** | 손으로 패키지를 지웠을 때. 증상이 나면 `rm -rf web/node_modules && make web-install`로 복구한다 (스탬프도 같이 지워진다) |
| T5 | **web 의존성을 늘릴 make 경로가 없다** — `web-install`은 `--frozen-lockfile` 전용이고([ADR-019](decisions/ADR-019.md)) lockfile을 갱신하는 타깃이 없다. `make ui-add`(shadcn)만 예외적으로 자기 의존성을 깐다 | 다음에 web 의존성을 늘리는 순간. `pnpm install`을 직접 쳐야 해서 **"Makefile이 유일한 진입점"이 깨진다.** 의존성 추가 자체가 ADR을 요구하는 일이라 지금은 타깃을 만들지 않았다 |

## 로컬에만 있는 것

운영 데이터가 아니다. 로컬 DB를 지우고 다시 올리면 사라진다.

| | 무엇 | 언제 아픈가 |
|---|---|---|
| L1 | 슬라이스 8 검증용으로 **가짜 승계 고아 행 1건**을 주입했다 (1342 Pride and Prejudice, 고아 2건) | 로컬 관리자 화면의 고아 목록이 항상 1건으로 보인다. **프로덕션에는 없다.** 다음에 로컬에서 "고아 0건" 빈 화면을 확인하려다 데이터가 있다고 착각할 때. 그 행을 지우거나 DB를 다시 올리면 사라진다 |

## 디자인에서 미룬 것

[ADR-038](decisions/ADR-038.md)이 여는 조건까지 같이 적어 둔 것들이다.

| | 무엇 | 언제 아픈가 |
|---|---|---|
| S1 | **한글 웹폰트가 없다** — 시스템 스택이라 기기마다 본문이 다르게 보인다 | **결정은 끝났다** ([ADR-039](decisions/ADR-039.md): Literata + Noto Serif KR, Fontsource 자체 호스팅). **막고 있는 것은 T5다** — web 의존성을 더할 make 경로가 없다. 그 타깃부터 만든 뒤 얹는다 |
| S2 | **다크 모드가 없다** — `.dark` 토큰은 shadcn 때문에 있지만 전환 수단이 없다 | 밤에 읽는 사람이 생길 때. **읽기 화면에는 자바스크립트가 없어서**(ADR-007·023) 토글을 달 수 없다. `prefers-color-scheme`만으로 열면 토큰 두 벌을 처음부터 같이 관리해야 한다 |
| S3 | **읽기 HTML이 읽는 사람마다 갈린다** — 읽기 설정 쿠키를 SSR이 읽어 렌더한다 ([ADR-040](decisions/ADR-040.md)) | 지금은 HTML을 캐시하지 않아 무해하다(ADR-034). **HTML 캐시를 켜는 순간 `reclassic_reader` 쿠키가 `Vary` 대상이 된다** — 빠뜨리면 남의 글자 크기가 캐시에서 나온다. 캐시를 켤 때 이 줄을 먼저 볼 것 |

## 프록시 뒤라서 생긴 것

| | 무엇 | 언제 아픈가 |
|---|---|---|
| P1 | **`r.RemoteAddr`이 방문자 IP가 아니다** — Cloudflare 주소다 | 지금은 IP를 읽는 곳이 없어 무해하다. **레이트리밋·남용 로깅·지역 판별을 넣는 순간 전부 걸린다.** 방문자 IP는 `CF-Connecting-IP`에 있다 ([ADR-034](decisions/ADR-034.md), `CONVENTIONS.md` "클라이언트 IP") |

---

## 해결됨

| | 무엇 | 어떻게 |
|---|---|---|
| D1 | `needs_review` 큐를 읽을 방법이 없다 | `ListBooks` 상태 필터 + `GET /admin/books/needs-review`. 챕터·문단 수를 최신 revision에서 붙인다. 화면은 다음 태스크 |
| D2 | 승계 고아 번역을 볼 수 없다 | `ListOrphanedSuccessions` + `GET /admin/successions/orphans`. 읽기만. 부분 인덱스 `revision_successions_orphaned`를 탄다 |
| D3 | `reviewer` 역할을 만들 방법이 없다 | `SetUserRole` + `POST /admin/users/{id}/role`. `member ↔ reviewer`만. 재로그인은 `Promote()`가 지킨다 |
| D4 | `translation_projects.status`가 `published`로 갈 수 없다 | `SetProjectStatus` + `POST /admin/projects/{id}/status`. `published_at`은 처음 공개 시각을 남기고 내릴 때 비우지 않는다 (ADR-036) |
| D5 | 목록 API가 하나도 없다 | `GET /books` · `GET /projects` · 관리자 목록들. 응답은 `{ items }` (ADR-037) |

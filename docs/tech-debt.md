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
| D8 | **원문 쪽 목차(`GET /books/{gutenbergId}/chapters`)가 없다** — 그리고 잠정 엔드포인트 `GET /books/{gutenbergId}/chapters/{idx}`를 남길지 없앨지가 아직 미결이다 | [openapi.yaml](../openapi.yaml)의 그 경로 주석 — "번역 슬라이스에서 교체하며, 그때 이 엔드포인트를 남길지 없앨지 함께 정한다" | 목차는 번역 프로젝트(`GET /projects/{projectId}/chapters`)만 연다. 원문 읽기 화면(`/books/…`)은 살아 있지만 `/books` 목록이 번역 프로젝트로만 링크를 걸어 **들어갈 입구가 없다** | **원문 읽기 화면에 입구를 만들려는 순간.** 그때 원문 목차가 필요해지는데, 원문에는 진행도가 없어 `ProjectChapterList`를 그대로 쓸 수 없다 — 스키마를 가르든 엔드포인트를 없애든 그 자리에서 정해야 한다. **목차 PR 안에서 정하지 않은 이유가 이것이다**: 계약을 두 번 고치게 된다 |
| D9 | **번역 프로젝트를 만드는 화면이 없다** | `POST /admin/projects`(`createProject`)가 계약·서버에 모두 있고 `adminOperations`로 잠겨 있다 | **웹에서 그것을 부르는 곳이 0건이다.** 관리 화면 셋(확인 큐·역할·공개)은 목록 조회와 상태 변경만 부른다. 만들려면 관리자 세션 쿠키로 API를 직접 불러야 한다 | **이미 아프다.** 프로덕션에 공개된 번역이 0건인 채로 있고, 그래서 목차·읽기 화면을 실물 데이터로 검증할 수가 없다 — PR 두 개가 연속으로 같은 벽에 막혔다. 슬라이스 8이 남긴 "프로덕션에서 제안 → 승인" 숙제도 이것 때문에 못 닫힌다. **슬라이스 8이 D4로 `open ↔ published` 전이를 열면서 정작 만드는 경로를 빠뜨린 자리다** |

## 만들어 놓고 부르지 않는 것

| | 무엇 | 언제 아픈가 |
|---|---|---|
| U1 | **`DeleteExpiredSessions`를 아무도 부르지 않는다** | 세션 수명 30일·갱신 없음([ADR-027](decisions/ADR-027.md))인데 만료 행이 영구히 쌓인다. 당장은 무해하고 사용자가 늘면 아프다 |
| U2 | `CountBooks`·`ChapterCoverage` 미사용 | 슬라이스 8에서 다시 봤다. **못 쓴다.** `CountBooks`는 전체 `books` 행 수인데, 공개 목록은 published 프로젝트와의 조인이고 페이지네이션도 없다(ADR-037) — `len(items)`가 곧 총량이다. `ChapterCoverage`는 챕터 단위 **번역** 커버리지(ADR-023)인데, `needs_review` 큐가 필요한 숫자는 책 단위 **파싱** 챕터·문단 수(ADR-014)다. 다른 숫자라 갖다 붙이면 게이트 판단이 틀린다. **목차 화면이 쓰는 것은 `ListProjectChapterCoverage`(`:many`)다** — 이름이 비슷하지만 프로젝트의 장 전체를 한 번에 읽는 다른 쿼리이고, `ChapterCoverage`(`:one`)는 여전히 부르는 곳이 없다 |

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

## 로컬에만 있는 것

운영 데이터가 아니다. 로컬 DB를 지우고 다시 올리면 사라진다.

| | 무엇 | 언제 아픈가 |
|---|---|---|
| L1 | 슬라이스 8 검증용으로 **가짜 승계 고아 행 1건**을 주입했다 (1342 Pride and Prejudice, 고아 2건) | 로컬 관리자 화면의 고아 목록이 항상 1건으로 보인다. **프로덕션에는 없다.** 다음에 로컬에서 "고아 0건" 빈 화면을 확인하려다 데이터가 있다고 착각할 때. 그 행을 지우거나 DB를 다시 올리면 사라진다 |

## 디자인에서 미룬 것

[ADR-038](decisions/ADR-038.md)·[ADR-039](decisions/ADR-039.md)가 여는 조건까지 같이 적어 둔 것들이다.

| | 무엇 | 언제 아픈가 |
|---|---|---|
| S2 | **다크 모드가 없다** — `.dark` 토큰은 shadcn 때문에 있지만 전환 수단이 없다 | 밤에 읽는 사람이 생길 때. **읽기 화면에는 자바스크립트가 없어서**(ADR-007·023) 토글을 달 수 없다. `prefers-color-scheme`만으로 열면 토큰 두 벌을 처음부터 같이 관리해야 한다 |
| S3 | **읽기 HTML이 읽는 사람마다 갈린다** — 읽기 설정 쿠키를 SSR이 읽어 렌더한다 ([ADR-040](decisions/ADR-040.md)) | 지금은 HTML을 캐시하지 않아 무해하다(ADR-034). **HTML 캐시를 켜는 순간 `reclassic_reader` 쿠키가 `Vary` 대상이 된다** — 빠뜨리면 남의 글자 크기가 캐시에서 나온다. 캐시를 켤 때 이 줄을 먼저 볼 것 |
| S5 | **폰트 시트가 모든 화면에 실린다** — `fonts.css`를 `__root.tsx`가 링크한다. 본문 서체를 쓰는 것은 읽기 화면뿐인데(ADR-039) 목록·관리·편집도 @font-face 선언 **gzip 24.9KB**를 함께 받는다. 폰트 파일 자체는 `unicode-range`가 막아 받지 않는다 | 관리 화면이 무거워질 때. 고치는 방법은 링크를 읽기 라우트의 `head()`로 내리는 것인데, **SSR HTML과 클라이언트 이동 양쪽에서 링크가 실제로 붙는지 확인해야 한다** — 빠지면 서체만 조용히 사라지고 빌드는 성공한다 |
| S6 | **한글에 굵은 글씨가 없다** — Noto Serif KR은 400 한 벌만 싣는다. 700을 더하면 산출물이 3.5MB에서 8MB로 는다(내려받는 양이 아니라 이미지 크기다). 그래서 읽기 화면의 제목도 400이다 | 본문 안에 `<strong>`이 필요해질 때. 지금은 라틴만 진짜 굵어지고 한글은 브라우저가 합성한다 — **한글에서 획이 뭉갠다** |
| S7 | **챕터 계약에 책 제목이 없다** — `ChapterView`·`ProjectChapterView` 어디에도 없어 읽기 화면 상단에 지금 읽는 책 이름을 쓸 수 없다. 시안이 요구하는 자리다 | **이미 아프다.** 장을 여럿 오가면 어느 책인지가 화면에서 사라진다. 고치려면 `openapi.yaml` → 핸들러 → `books` 조인까지 내려가야 해서 서체 PR에 넣지 않았다 |
| S8 | **설정 버튼이 장 이동 격자 밖에 있다** — 시안은 하단 한 줄에 `[목차] [⚙] … [‹] [다음 장]`을 같이 세우는데, [ADR-040](decisions/ADR-040.md) 작업은 `.reader-nav` 격자를 건드리지 않으려고 ⚙를 위치 표시 줄(`.reader-tools`)에 붙였다. 목차 링크(S4)가 그 격자를 2열에서 다시 짜는 중이라 같은 자리를 두 작업이 동시에 고치는 것을 피했다 | 목차 링크가 들어와 격자가 정리된 **직후**. 그때 ⚙를 같은 줄로 옮기면 시안과 맞고 하단이 두 줄에서 한 줄로 준다. **미루면 조용히 굳는다** — 지금 모양도 동작은 하므로 고칠 이유가 화면에 드러나지 않는다 |
| S9 | **자바스크립트 없는 장 이동은 방향이 항상 "다음"이다** — 교차 문서 전환을 그리는 것은 도착한 문서인데, 그 문서의 CSS에는 **떠나온 장 번호가 없다.** 알아낼 길이 `pagereveal` 스크립트뿐이라 JS 없는 경로는 기본값(오른쪽에서 들어옴)으로 간다 ([ADR-041](decisions/ADR-041.md)) | 스크립트를 끈 채, 또는 하이드레이션이 끝나기 전에 **"이전 장"을 누를 때** 본문이 반대로 밀린다. 하이드레이션 뒤 이동에는 라우터가 방향을 넘기므로(`viewTransition.types`) 평소에는 드러나지 않는다. **닫는 자리는 읽기 설정 스크립트([ADR-040](decisions/ADR-040.md))가 생기는 순간이다** — 거기에 `pagereveal`에서 `document.referrer`의 장 번호를 비교해 타입을 얹으면 끝난다 |
| S10 | **"읽던 자리" 표시가 없다** — 목차에도 도서 목록에도 어디까지 읽었는지가 나오지 않는다. 시안이 요구하는 자리다 (S4에서 갈라져 나왔다) | 책을 며칠에 걸쳐 읽을 때. 다시 들어오면 몇 장이었는지를 사람이 기억해야 한다. **쿠키([ADR-040](decisions/ADR-040.md))가 먼저 있어야 한다** — 읽기 화면에는 서버가 아는 "나"가 없다 |

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
| T5 | web 의존성을 늘릴 make 경로가 없다 | `make web-add PKG=… (DEV=1)`. **lockfile을 고치는 자리가 여기 하나다** — `web-install`은 `--frozen-lockfile` 전용으로 남았다 (ADR-019). 설치 스탬프를 같이 앞세워 뒤이은 `web-install`이 통째로 다시 깔지 않는다 |
| S1 | 한글 웹폰트가 없어 기기마다 본문이 다르다 | Fontsource 자체 호스팅 (ADR-039). Literata 7벌 + Noto Serif KR 124벌, 전부 `unicode-range`·`font-display: optional`. `korean.css`(통짜 949KB)는 쓰지 않는다 — **그것을 import하면 화면은 똑같고 청구서만 는다** |
| S4 | 읽기 화면에서 목차로 돌아가는 링크가 없다 | `.reader-nav`에 `[목차]`를 더했다. 격자를 flex로 다시 짜서 칸 수가 화면마다 달라지는 것을 흡수한다 — 원문 읽기 화면에는 목차가 없고(D8) 첫 장·마지막 장에는 이전·다음이 없다. **원문 쪽에는 만들지 않았다** — 갈 곳이 없다. 시안의 ⚙ 자리는 비워 두었다 ([ADR-040](decisions/ADR-040.md)의 몫이다) |

# 스파이크 명세 — Gutenberg 파서 검증

**작업 종류:** 스파이크 (탐색적 구현)
**선행 문서:** `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/decisions/index.md` (특히 ADR-011)
**작업 시작 전에 위 세 문서를 반드시 읽을 것.**

---

## 1. 이 작업이 답해야 할 질문

> **Project Gutenberg의 HTML은 장·문단으로 자동 분리되는가? 몇 퍼센트나?**

이 숫자 하나가 제품 설계를 가른다.

| 자동 분리 성공률 | 설계에 미치는 영향 |
|---|---|
| 85% 이상 | 관리자 수동 검토는 예외 처리 수준. "실패 목록 보고 재시도" 화면이면 충분 |
| 60~85% | 수동 보정 UI를 제대로 만들어야 함. 문단 병합·분할, 챕터 경계 조정 필요 |
| 60% 미만 | 스키마에 "관리자 보정 레이어"가 필요. 원본 파싱 결과 위에 덮어쓰는 구조로 재설계 |

**따라서 이 스파이크의 진짜 산출물은 코드가 아니라 리포트다.**
코드는 그 숫자를 얻기 위한 수단이며, 결론이 담긴 `docs/references/parser-report.md`가 최종 결과물이다.

---

## 2. 범위

### 하는 것
- Gutenberg 도서 HTML 수집 (로컬 캐시)
- 본문 영역 식별 및 장·문단 분리
- 추출 전략 여러 개 + 신뢰도 점수
- 권별 추출 결과 스냅샷(golden) 생성
- 사람이 눈으로 품질을 확인할 수 있는 HTML 리포트
- 결론과 권고가 담긴 `docs/references/parser-report.md`

### 하지 않는 것 — 건드리면 안 됨
- 데이터베이스, 마이그레이션, sqlc
- River 잡 큐
- S3 / R2 업로드
- HTTP API, `openapi.yaml`
- 웹 프론트엔드
- 번역·제안·검수 관련 일체

스파이크의 목적은 **하나의 불확실성만 제거**하는 것이다. 범위를 넓히지 말 것.

### 의존성 제한

```
github.com/PuerkitoBio/goquery
golang.org/x/net/html
```

이 둘과 표준 라이브러리만 사용한다. 다른 패키지가 필요하다고 판단되면
추가하기 전에 `docs/decisions/index.md`에 `Proposed` ADR로 남기고 사람에게 확인할 것.

---

## 3. 사전 준비

```bash
make doctor          # go 설치 확인. 없으면: brew install go
go mod init github.com/dinnertime/reclassic
go get github.com/PuerkitoBio/goquery
```

> **확인 필요:** 모듈 경로를 `github.com/dinnertime/reclassic`로 가정했다.
> GitHub 소유자명이 다르면 시작 전에 사람에게 확인할 것.

---

## 4. 산출물

| 경로 | 커밋 | 내용 |
|---|---|---|
| `cmd/parsecheck/main.go` | O | CLI 진입점 |
| `internal/parse/extractor.go` | O | `Extractor` 인터페이스 + 전략 체인 |
| `internal/parse/strategy_*.go` | O | 전략별 구현 |
| `internal/parse/normalize.go` | O | 텍스트 정규화, `stable_id` 생성 |
| `internal/parse/confidence.go` | O | 신뢰도 계산 |
| `internal/parse/testdata/corpus.json` | O | 검증 대상 도서 목록 |
| `internal/parse/testdata/golden/*.json` | O | 권별 추출 결과 스냅샷 |
| `internal/parse/parse_test.go` | O | golden 회귀 테스트 |
| `docs/references/parser-report.md` | O | **결론과 권고** |
| `.cache/gutenberg/*.html` | **X** | 내려받은 원본. gitignore 대상 |
| `.cache/report.html` | **X** | 상세 리포트 (로컬 확인용) |

---

## 5. 구현 명세

### 5.1 CLI

```
parsecheck fetch  -corpus=<path> -cache=<dir>
    corpus.json의 도서를 내려받아 캐시에 저장한다. 이미 있으면 건너뛴다.

parsecheck report -corpus=<path> -cache=<dir> [-out=.cache/report.html]
    캐시된 원본을 추출하고 HTML 리포트와 표준출력 요약을 생성한다.

parsecheck golden -corpus=<path> -cache=<dir> [-update]
    golden 스냅샷과 현재 추출 결과를 비교한다. -update 시 갱신한다.
```

`make fetch-corpus`, `make parsecheck`가 앞의 두 개를 감싼다.

### 5.2 `corpus.json`

```json
{
  "books": [
    {
      "gutenberg_id": 1342,
      "expected_title": "Pride and Prejudice",
      "category": "standard-novel",
      "note": "표준 장편. Chapter I, II ... 형식"
    }
  ]
}
```

`category`는 `§6`의 분류를 쓴다. `expected_title`은 fetch 시 검증에 사용한다 —
**ID가 틀렸는데 조용히 엉뚱한 책을 받는 것을 막기 위함**이다.
제목이 일치하지 않으면 경고를 출력하고 `corpus.json`을 수정한 뒤 다시 받는다.

### 5.3 fetch

원문 URL은 아래 순서로 시도한다.

```
1. https://www.gutenberg.org/cache/epub/{id}/pg{id}-images.html
2. https://www.gutenberg.org/cache/epub/{id}/pg{id}.html
3. https://www.gutenberg.org/files/{id}/{id}-h/{id}-h.htm
```

**수집 규칙 — 반드시 지킬 것:**
- **직렬 요청.** 병렬 금지. goroutine으로 동시 요청하지 말 것.
- **요청 간 최소 1초 대기.** 재시도 시에는 지수 백오프.
- **User-Agent 명시.** `.env`의 `GUTENBERG_USER_AGENT`를 쓰고, 없으면 실행을 거부한다.
- **캐시에 이미 있으면 재요청하지 않는다.**
- 4xx는 재시도하지 않는다. 5xx만 최대 3회 재시도.

Gutenberg는 공격적인 크롤러의 IP를 차단하며 복구가 어렵다. 이 규칙을 완화하지 말 것.

캐시 파일명은 `{id}.html`, 함께 `{id}.meta.json`에 최종 URL과 `sha256`을 기록한다.

### 5.4 Boilerplate 제거

Gutenberg 파일에는 라이선스 텍스트가 본문 앞뒤에 붙어 있다.
**DOM 휴리스틱보다 이 텍스트 마커가 우선이다.**

```
*** START OF THE PROJECT GUTENBERG EBOOK ... ***
*** END OF THE PROJECT GUTENBERG EBOOK ... ***
```

표기 변형(`START OF THIS PROJECT GUTENBERG EBOOK`, 별표 개수 차이, 대소문자)이 있으므로
정규식으로 유연하게 매칭한다. 마커를 찾지 못한 경우 `warnings`에 기록하고
전체 문서를 대상으로 진행한다 — 실패로 처리하지 않는다.

마커 제거 후의 본문 텍스트 길이를 `§5.8`의 커버리지 분모로 쓴다.

### 5.5 `Extractor` 인터페이스

```go
package parse

type Confidence float64 // 0.0 ~ 1.0

type Paragraph struct {
    Idx      int
    Text     string // 정규화된 평문
    HTML     string // 원본 조각
    StableID string // §5.7
}

type Chapter struct {
    Idx        int
    Title      string
    Anchor     string
    Paragraphs []Paragraph
}

type Result struct {
    Chapters   []Chapter
    Strategy   string
    Confidence Confidence
    Warnings   []string
}

type Extractor interface {
    Name() string
    Extract(doc *goquery.Document) (*Result, error)
}
```

전략 체인은 **모든 전략을 실행한 뒤 신뢰도가 가장 높은 결과를 채택**한다.
첫 성공에서 멈추지 않는다 — 스파이크의 목적이 전략별 성능 비교이기 때문이다.
리포트에는 채택된 전략뿐 아니라 **모든 전략의 점수**를 기록한다.

### 5.6 전략

| 이름 | 대상 | 방법 |
|---|---|---|
| `section-chapter` | 최신 전사본 (2020년대) | `section.chapter`, `div.chapter` 단위. 내부 `h2`가 제목, `p`가 문단 |
| `heading-split` | 평문형 (90~2000년대) | body를 순회하며 `h1`~`h3`를 만나면 새 챕터 시작. 사이의 `p`를 수집 |
| `anchor-toc` | 목차가 있는 책 | 목차의 `<a href="#...">` 타겟 앵커를 챕터 경계로 사용 |
| `single-chapter` | 짧은 에세이·시집 | 전체를 한 챕터로. 폴백. 신뢰도 상한 0.5 |

각 전략은 독립 파일(`strategy_section_chapter.go` 등)에 둔다.

**문단 수집 시 제외할 것:** `nav`, `header`, `footer`, `.toc`, `.pg-boilerplate`,
`.tnote`(역주), `figcaption`, `table` 내부 텍스트.
제외 규칙은 상수로 모아두고 리포트에서 조정 효과를 볼 수 있게 한다.

### 5.7 정규화와 `stable_id`

**ARCHITECTURE.md의 핵심 불변식 1번에 해당한다. 이 규칙은 나중에 바꾸기 매우 어렵다.**

정규화 순서:

```
1. HTML 엔티티 디코드
2. Unicode NFC 정규화
3. 곡선 따옴표 → ASCII  ( ' ' → '   /   " " → " )
4. 대시류 → ASCII 하이픈  ( — – ‐ → - )
5. 줄바꿈·탭 → 공백
6. 연속 공백 → 단일 공백
7. 앞뒤 공백 제거
```

```go
StableID = hex(sha256(normalized))[:16]
```

대소문자는 변환하지 않는다. 정규화 함수는 순수 함수여야 하며 단위 테스트를 붙인다.

> **스파이크 이후 보완 (ADR-016).** 한 책에 같은 본문이 반복되면 위 정의만으로는
> 번역 행이 공유된다. 2번째 등장부터 `sha256(normalized + "#" + n)`으로 분리하기로 했다.
> 순서 부여는 파서 후처리에서 하며, 이 절의 `Normalize`는 순수 함수로 그대로 둔다.

### 5.8 신뢰도 계산

측정 가능한 신호로만 계산한다. 상수는 `confidence.go` 상단에 모은다.

| 신호 | 정의 |
|---|---|
| `coverage` | 추출된 문단 텍스트 총 길이 ÷ boilerplate 제거 후 본문 텍스트 길이 |
| `titleRatio` | 제목이 비어있지 않은 챕터 비율 |
| `chapterSanity` | 챕터 수 ≥ 1 **그리고** 챕터당 평균 문단 수 ≥ 3 |
| `paraSanity` | 문단 길이 중앙값이 40~3000자 범위 |
| `shortRatio` | 20자 미만 문단의 비율 |
| `noiseTitle` | 챕터 제목에 `CONTENTS`/`INDEX`/`ILLUSTRATIONS` 포함 여부 |

```
confidence = clamp(
      0.45 * coverage
    + 0.20 * titleRatio
    + 0.15 * bool(chapterSanity)
    + 0.10 * bool(paraSanity)
    + 0.10 * (1 - min(shortRatio / 0.3, 1))
    - 0.15 * bool(noiseTitle)
, 0, 1)
```

**`coverage`가 가장 중요하다.** "파서가 본문 절반을 조용히 놓쳤다"는 가장 위험한 실패를
잡아내는 신호이기 때문에 가중치를 가장 크게 뒀다.

판정 구간:

```
>= 0.85   자동 확정 가능
0.60 ~ 0.85   관리자 검토 필요
<  0.60   실패
```

이 수치는 검증 결과를 보고 조정할 수 있다. 조정했다면 근거를 리포트에 남길 것.

### 5.9 golden 스냅샷

`internal/parse/testdata/golden/{id}.json`.

**원문 텍스트를 넣지 말 것.** 파일이 커지고 저장소가 비대해진다.
길이와 해시만 기록하면 회귀 검증 목적은 그대로 달성된다.

```json
{
  "gutenberg_id": 1342,
  "title": "Pride and Prejudice",
  "source_sha256": "…",
  "strategy": "heading-split",
  "confidence": 0.91,
  "coverage": 0.97,
  "chapter_count": 61,
  "paragraph_count": 2431,
  "total_chars": 684213,
  "warnings": [],
  "chapters": [
    { "idx": 0, "title": "Chapter I", "paragraph_count": 24,
      "chars": 5120, "text_sha256": "…" }
  ]
}
```

**타임스탬프를 넣지 말 것.** 골든 파일은 결정적이어야 하며,
같은 입력에 같은 출력이 나와야 diff가 의미를 갖는다.

`parse_test.go`는 캐시가 있을 때만 실행하고, 없으면 `t.Skip`한다
(CI에서 Gutenberg에 요청이 나가면 안 된다).

### 5.10 리포트

**`.cache/report.html`** — 로컬 확인용. 권별로 다음을 보여준다.
- 판정(자동/검토/실패), 채택 전략, 신뢰도, coverage
- **모든 전략의 점수 비교표**
- 챕터 목록과 챕터별 문단 수
- **각 챕터 첫 문단 앞 200자** — 실제 추출 품질을 눈으로 확인하는 용도
- 경고 목록

**`docs/references/parser-report.md`** — 커밋 대상. 이것이 최종 산출물이다.
`§8`의 형식을 따른다.

---

## 6. 검증 대상 도서

구조 다양성이 목적이다. 인기도가 아니라 **구조가 겹치지 않도록** 골랐다.

| 분류 | 도서 (Gutenberg ID) | 확인하려는 것 |
|---|---|---|
| `standard-novel` | 1342 오만과 편견 / 84 프랑켄슈타인 / 345 드라큘라 | 기본 `Chapter N` 구조 |
| `book-chapter` | 2701 모비 딕 / 98 두 도시 이야기 | `Book I → Chapter` 중첩 구조 |
| `odd-heading` | 46 크리스마스 캐럴 | 장 명칭이 `Stave`. 제목 패턴 의존성 검증 |
| `short-stories` | 1661 셜록 홈즈의 모험 / 2814 더블린 사람들 | 단편집. 장 = 독립 작품 |
| `epistolary` | 84 프랑켄슈타인 (letter 부분) / 345 드라큘라 | 서간·일기체. 날짜가 제목처럼 보임 |
| `long-classic` | 1400 위대한 유산 / 74 톰 소여 / 76 허클베리 핀 | 분량 스트레스 |
| `essay-short` | 1080 겸손한 제안 | 챕터 없는 짧은 산문. `single-chapter` 폴백 검증 |
| `play` | 2542 인형의 집 / 1524 햄릿 | 희곡. **깨끗한 실패**가 정답 |
| `verse` | 1727 오디세이아 | 운문. 행 단위라 문단 개념이 다름. **깨끗한 실패**가 정답 |
| `mega` | 100 셰익스피어 전집 | 초대형 합본. 메모리·성능 한계 |
| `hard` | 4300 율리시스 | 최난도. 실패해도 무방 |
| `nonfiction` | 205 월든 / 1232 군주론 | 논픽션 구조 |
| `novella` | 5200 변신 / 219 어둠의 심연 / 174 도리언 그레이의 초상 | 중편 |

**희곡과 운문은 실패해도 된다.** 다만 **조용히 잘못된 결과를 내놓지 말고
낮은 신뢰도로 명확히 실패해야 한다.** 이걸 확인하는 것이 목적이므로 반드시 포함할 것.

> **ID 검증 필수:** 위 ID는 확인이 필요하다. fetch 시 `expected_title`과 대조하고,
> 불일치하면 실제 ID를 찾아 `corpus.json`을 고친 뒤 진행할 것. 임의로 넘어가지 말 것.

---

## 7. 반드시 지킬 것

1. **Gutenberg 요청은 직렬, 최소 1초 간격, User-Agent 명시.** 병렬 요청 절대 금지.
2. **원본 HTML을 커밋하지 말 것.** `.cache/`는 gitignore 대상이다.
3. **golden에 원문 텍스트와 타임스탬프를 넣지 말 것.**
4. **범위 밖(DB·API·웹)을 건드리지 말 것.**
5. **판정 기준을 맞추려고 수치를 조작하지 말 것.** 성공률이 낮게 나오는 것도 유효한 결과다.
   오히려 그 경우가 설계에 더 큰 정보를 준다.
6. 작업 종료 전 `make lint && make test` 통과.
7. 새 설계 판단이 필요하면 임의로 정하지 말고 `docs/decisions/index.md`에 `Proposed` ADR로 남길 것.

---

## 8. 완료 조건

- [x] `corpus.json`의 모든 도서가 캐시에 존재하고 `expected_title`이 검증됨
- [x] 4개 전략이 모두 구현되고 권별로 전 전략 점수가 기록됨
- [x] 권별 golden 스냅샷 생성, `make test` 통과
- [x] `.cache/report.html` 생성 — 챕터별 첫 문단 미리보기 포함
- [x] `docs/references/parser-report.md` 작성 (아래 형식)
- [x] `make lint && make test` 통과

### `docs/references/parser-report.md` 형식

```markdown
# 파서 검증 결과

## 요약
- 검증 도서: N권
- 자동 확정 가능(≥0.85): N권 (NN%)
- 검토 필요(0.60~0.85): N권 (NN%)
- 실패(<0.60): N권 (NN%)

## 전략별 성능
| 전략 | 채택 횟수 | 평균 신뢰도 | 평균 coverage |

## 분류별 결과
| 분류 | 도서 수 | 자동 | 검토 | 실패 |

## 실패 유형 분석
실패한 각 권에 대해: 무엇이 실패했고, 원인이 무엇이며,
전략 추가로 해결 가능한지 아니면 구조적으로 불가능한지.

## 권고
1. 관리자 수동 보정 UI가 어느 수준으로 필요한가 (§1 표 기준)
2. 스키마에 보정 레이어가 필요한가
3. 지원 대상에서 제외할 도서 분류가 있는가 (예: 희곡, 운문)
4. 신뢰도 임계값을 조정했다면 그 근거

## 후속 ADR 제안
위 권고 중 설계 결정이 필요한 항목을 `Proposed` ADR 초안으로.
```

---

## 9. 작업 완료 후

`docs/references/parser-report.md`의 **요약**과 **권고**를 사람에게 보고할 것.
코드 설명보다 **숫자와 판단**이 중요하다.
이 스파이크의 목적은 다음 설계 결정을 내리는 것이지 파서를 완성하는 것이 아니다.

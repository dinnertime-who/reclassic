# 슬라이스 명세 — 번역 (제안 · 검수 · 공개)

**작업 종류:** 구현 슬라이스 (수직)
**선행 슬라이스:** `docs/SLICE_INGEST_AUTOMATION.md`
**선행 문서:** `AGENTS.md`, `docs/ARCHITECTURE.md` (특히 "핵심 불변식", "공개와 SEO"),
`docs/DECISIONS.md` (특히 ADR-004 / 005 / 007 / 010 / 016)
**다음 슬라이스:** `docs/SLICE_AUTH.md` — 세션 인증

---

## 1. 이 작업이 답해야 할 질문

지금까지 만든 것은 전부 **원문**이다. 원문 페이지는 `noindex`라 검색에서 보이지 않는다(ADR-007).
이 프로젝트의 고유 콘텐츠는 번역문 하나뿐이고, 아직 하나도 없다.

> **1. 문단당 확정본 1개가 동시 승인에서도 지켜지는가?**
> **2. 재파싱했을 때 번역이 실제로 승계되는가 — 측정이 아니라 실행?**
> **3. 커버리지 기준으로 색인 여부가 갈리는가?**

1번이 가장 위험하다. ADR-005는 복합 PK와 `WHERE status = 'pending'`으로
동시 승인을 막는다고 적었다. **두 검수자가 동시에 눌렀을 때 실제로 어떻게 되는지는 확인된 적이 없다.**
승인 트랜잭션은 이 프로젝트에서 유일하게 데이터가 조용히 틀어질 수 있는 지점이다.

2번은 읽기 경로 슬라이스에서 **측정**만 했다(21권 100%). 이번엔 **실행**한다.

---

## 2. 범위

### 하는 것

- 번역 테이블 4개 + `users` 뼈대
- 번역 프로젝트 생성 (관리자)
- 문단 번역 **제안** 등록·수정·철회
- **검수** — 승인 한 트랜잭션 3단계 (ARCHITECTURE 불변식 3)
- **승계 실행** — revision 전환 시 `stable_id` 일치분 자동 승계
- 프로젝트 기반 챕터 조회 API — 잠정 엔드포인트를 교체한다
- SSR 읽기 화면에 확정 번역 표시. **커버리지 80% 기준 색인 분기**
- 사이트맵 생성 (워커, R2)

### 하지 않는 것 — 건드리면 안 됨

- **세션 인증.** 다음 슬라이스다. 이번엔 임시 헤더로 신원을 받는다 (§4.7)
- **LLM 번역 제안.** ADR-010이 초기 범위에서 뺐다
- **편집·검수 CSR 화면.** API까지만. 화면은 인증이 있어야 의미가 있다
- **용어집(`book_glossary`) UI.** 테이블만 만든다
- 파서·수집·적재 로직 변경

### 의존성

**새로 추가하지 않는다.**

---

## 3. 산출물

| 경로 | 내용 |
|---|---|
| `internal/db/migrations/00002_translations.sql` | 번역 테이블 4개 + users |
| `internal/db/queries/translations.sql` | 제안·검수·조회 쿼리 |
| `internal/translate/` | 제안·검수 도메인, 승인 트랜잭션, 승계 실행 |
| `internal/book/succession.go` | 측정에 더해 **실행** 추가 |
| `internal/jobs/sitemap.go` | 사이트맵 생성 잡 |
| `openapi.yaml` | 프로젝트·제안·검수 엔드포인트 |
| `web/` | 번역 표시 읽기 라우트 |
| Makefile · `AGENTS.md` | 함께 갱신 |

---

## 4. 구현 명세

### 4.1 스키마

`ARCHITECTURE.md` 데이터 모델 그대로다. 두 가지만 구체화한다.

- `users`는 뼈대만이다: `id, handle UNIQUE, display_name, role, created_at`.
  `role`은 `member | reviewer | admin`. 세션은 다음 슬라이스다.
- `translation_proposals.status`는 `pending|approved|rejected|superseded|withdrawn`.
  **CHECK 제약으로 강제한다.**

### 4.2 승인 트랜잭션 — 불변식 3

`ARCHITECTURE.md`의 SQL 세 단계를 그대로 구현한다.

```sql
BEGIN;
  UPDATE translation_proposals SET status='superseded'
   WHERE id = (SELECT proposal_id FROM paragraph_translations
                WHERE project_id=$1 AND paragraph_stable_id=$2);

  UPDATE translation_proposals
     SET status='approved', reviewed_by=$3, reviewed_at=now()
   WHERE id=$4 AND status='pending';        -- 0 rows면 롤백 후 409

  INSERT INTO paragraph_translations (...) VALUES (...)
  ON CONFLICT (project_id, paragraph_stable_id) DO UPDATE SET ...;
COMMIT;
```

**`AND status='pending'`을 빼지 말 것.** 두 검수자가 서로 다른 제안을 동시에 승인하는 것을
이 조건 하나가 막는다. 영향 행이 0이면 롤백하고 409다.

**동시 승인 테스트를 반드시 쓴다.** 고루틴 둘이 서로 다른 제안을 같은 문단에 동시에 승인하고,
정확히 하나만 성공하며 `paragraph_translations`에 행이 하나인 것을 확인한다.
이것이 이 슬라이스의 핵심 검증이다.

> **결과 — 3단계만으로는 부족했다.** 확정본은 1행이었지만 `approved` 제안이 2개 남았다.
> 두 트랜잭션이 모두 1단계에서 "확정본 없음"을 보기 때문이다.
> **단발 테스트로는 통과한다** — 처음에 그렇게 놓쳤고, 40라운드 반복에서 드러났다.
> 승인 트랜잭션 맨 앞에 문단 단위 자문 잠금을 추가했다. 근거는 **ADR-024**.
> `TestConcurrentApprovalStress`의 라운드 수를 줄이지 말 것.

### 4.3 승계 실행

읽기 경로 슬라이스의 `MeasureSuccession`은 읽기 전용이었다. 이번엔 실행한다.

- 새 revision이 활성화될 때, 이전 활성 revision과 `stable_id`가 **일치하는 문단의 확정 번역을 그대로 둔다.**
  `paragraph_translations`가 `paragraph_stable_id`를 참조하므로 **실제로는 아무것도 옮기지 않는다** —
  이것이 ADR-004의 설계 의도다.
- **소실된 `stable_id`(저장본에만 있던 것)에 확정 번역이 붙어 있으면 관리자 확인 큐로 보낸다.**
  조용히 버리지 않는다. 번역은 사람이 쓴 것이다.
- 승계 결과를 `revision_succession` 로그로 남긴다: revision 쌍, 일치·소실 수, 고아 번역 수.

### 4.4 커버리지와 색인 — 임계값 80%

ADR-007이 "수치 미정"으로 남긴 항목을 **80%로 정한다** (ADR-023).

```
coverage = 승인된 문단 수 / 챕터의 전체 문단 수
coverage >= 0.80  →  index, follow   (canonical)
coverage <  0.80  →  noindex, follow
```

- 원문 전용 페이지는 커버리지와 무관하게 언제나 `noindex`다. 변하지 않는다.
- 미확정 문단은 읽기 화면에서 원문으로 노출하고 진행률을 표시한다.
  부분 공개를 허용한다 — 100%를 기다리면 아무 책도 공개하지 못한다.

### 4.5 챕터 조회 API 교체

```
GET /projects/{projectId}/chapters/{idx}      ← 최종 계약 (ARCHITECTURE)
→ { chapter, paragraphs: [{ stableId, sourceText, approvedTranslation,
                            proposalCount }], totalChapters, coverage }
```

- 잠정 엔드포인트 `GET /books/{gutenbergId}/chapters/{idx}`는 **남긴다.**
  원문만 읽는 경로가 여전히 필요하고(번역 프로젝트가 없는 책), 이미 SSR 라우트가 쓰고 있다.
- `myProposalStatus`는 인증이 있어야 의미가 있다. 다음 슬라이스로 미룬다.

### 4.6 사이트맵

- 워커가 생성해 R2에 올린다. 색인 대상(§4.4 통과)만 넣는다.
- 파일당 URL 50,000 / 50MB 제한을 지켜 분할하고 인덱스 사이트맵을 만든다.

### 4.7 신원 — 임시

> **이 절은 이미 걷어냈다 (ADR-027).** `X-User-Handle`은 코드에 없다.
> 신원은 Google 로그인 세션 쿠키에서 온다. 아래는 당시 기록이다.

세션 인증은 다음 슬라이스다. 이번엔 `X-User-Handle` 헤더로 신원을 받고
`users`에서 조회한다. 없으면 401.

**이것은 인증이 아니다.** 위조가 자명하게 가능하다.
프로덕션 배포 전에 반드시 걷어낸다. 그래서 슬라이스 5가 있다.

### 4.8 테스트

| 종류 | 대상 | DB |
|---|---|---|
| 단위 | 커버리지 계산, 색인 판정 | 아니오 |
| 통합 | **동시 승인**, 승인 3단계, 승계 실행, 고아 번역 감지 | 예 |

---

## 5. 반드시 지킬 것

1. **`paragraph_translations`에 문단당 2행 이상 넣지 않는다.** 복합 PK가 막는다. 의도된 제약이다.
2. **승인 UPDATE의 `AND status='pending'`을 빼지 않는다.** 동시 승인이 뚫린다.
3. **번역은 `paragraph_stable_id`에 붙는다.** `paragraphs.id`를 참조하지 않는다.
4. **소실된 번역을 조용히 버리지 않는다.** 관리자 확인 큐로 보낸다.
5. **원문 전용 페이지의 `noindex`를 건드리지 않는다.**
6. **API 변경은 `openapi.yaml`부터.**
7. **`DATABASE_URL` 없이 `make test`가 통과해야 한다.**
8. 작업 종료 전 `make lint && make test` 통과.

---

## 6. 완료 조건

- [x] `make migrate`가 번역 테이블 4개와 `users`를 만든다
- [x] 제안 등록 → 검수 승인 → 읽기 화면에 번역문이 보인다
- [x] **두 검수자가 같은 문단의 서로 다른 제안을 동시에 승인하면 하나만 성공하고 409가 하나 난다**
      (고루틴 테스트로 확인)
- [x] 승인된 제안을 다른 제안으로 교체하면 이전 것이 `superseded`가 된다
- [x] 재파싱으로 revision을 바꿔도 `stable_id`가 같은 문단의 번역이 그대로 보인다
- [x] 소실된 `stable_id`에 번역이 붙어 있으면 고아로 기록된다
- [x] 커버리지 80% 미만 챕터는 `noindex`, 이상은 `index`
- [x] 사이트맵에 색인 대상만 들어간다
- [x] `DATABASE_URL` 없이 `make test` 통과
- [x] `make lint && make test` 통과

---

## 7. 다음

`docs/SLICE_AUTH.md` — 세션 인증. §4.7의 임시 헤더를 걷어낸다.
그 전까지 **프로덕션에 배포하지 않는다.**

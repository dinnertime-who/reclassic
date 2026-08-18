# 설계 결정 이력 (ADR)

새 결정은 맨 아래에 추가한다. 기존 항목은 지우지 않는다.
결정을 뒤집을 때는 해당 항목의 상태를 `Superseded by ADR-NNN`으로 바꾸고 새 항목을 추가한다.

상태: `Accepted` / `Proposed` / `Superseded` / `Rejected`

---

## ADR-001 — 백엔드는 Go, MSA로 쪼개지 않는다
**상태:** Accepted

**맥락:** MSA 구성을 검토했다. 도메인이 수집·번역 둘뿐이고 1인 개발 규모다.

**결정:** Go 모듈 하나에 `cmd/api`, `cmd/worker` 두 바이너리. 서비스를 더 쪼개지 않는다.
확장이 필요하면 같은 바이너리를 다른 큐만 소비하도록 추가 기동한다.

**결과:** 배포 단위가 적어 Railway 운영이 단순하다. 도메인 경계가 검증된 뒤에야 분리를 재검토한다.

---

## ADR-002 — 배포는 Railway
**상태:** Accepted

**맥락:** 초기 규모에 맞는 관리형 플랫폼이 필요했다.

**결정:** Railway. api / worker / postgres / web 네 서비스.

**결과:** k8s 프리미티브(서비스 메시, 카나리)는 쓸 수 없다. 관측성은 외부 도구(OTel)를 붙여야 한다.
프라이빗 네트워크가 IPv6 전용이라 `[::]` 바인딩이 강제된다.

---

## ADR-003 — 잡 큐는 River (Redis 아님)
**상태:** Accepted

**맥락:** asynq(Redis)와 River(Postgres)를 비교했다.

**결정:** River. Postgres 트랜잭션 안에서 enqueue가 가능한 것이 결정적이었다.
"책 레코드는 생성됐는데 잡 등록이 실패"하는 틈이 생기지 않는다.

**결과:** Railway 서비스가 하나 줄어든다(Redis 불필요). 처리량 한계는 Postgres에 종속된다.

---

## ADR-004 — 번역은 `paragraph_stable_id`에 결합한다
**상태:** Accepted

**맥락:** 파서를 개선해 재파싱하면 문단 행 ID가 밀린다. 번역이 `paragraphs.id`를 참조하면
모든 번역이 엉뚱한 문단에 붙는다.

**결정:** `stable_id = sha256(정규화된 본문)[:16]`. 번역 테이블은 FK 대신 이 값을 참조한다.
파싱 결과는 불변 `book_revisions`로 쌓는다.

**결과:** FK 무결성을 포기하는 대신 재파싱이 안전해진다. revision 전환 시 해시 매칭으로 승계한다.
이 프로젝트에서 가장 되돌리기 어려운 지점이므로 우선순위가 높다.

---

## ADR-005 — 문단당 확정 번역 1개를 스키마로 강제한다
**상태:** Accepted

**맥락:** 여러 사용자가 제안하고 검수를 거쳐 하나만 공개하는 모델이다.

**결정:** `translation_proposals`(N개)와 `paragraph_translations`(0 또는 1개)를 분리하고,
후자에 복합 PK `(project_id, paragraph_stable_id)`를 건다.
승인은 한 트랜잭션 안에서 처리하며 `WHERE status = 'pending'` 조건으로 동시 승인을 막는다.

**결과:** 불변식이 애플리케이션이 아니라 DB에서 지켜진다.

---

## ADR-006 — 웹은 TanStack Start (SSR)
**상태:** Accepted

**맥락:** 검색 노출이 필요하다. Go 템플릿 + 별도 SPA, Next.js, TanStack Start를 비교했다.

**결정:** TanStack Start. 2026-03 v1.0 릴리스로 안정화됐다.
읽기 화면은 SSR, 편집·검수 화면은 CSR로 **같은 라우터 안에서** 라우트별로 나눈다.

**결과:** 프론트를 두 벌 만들지 않아도 된다. 대신 Node 서비스가 하나 늘고,
API 베이스 URL을 서버·클라이언트에서 분기해야 한다.
비즈니스 로직이 server function으로 새지 않도록 규칙을 둔다(AGENTS.md 참조).

---

## ADR-007 — 원문 페이지는 색인하지 않는다
**상태:** Accepted

**맥락:** Gutenberg 원문은 이미 수백 개 사이트에 존재한다. 신생 도메인이 중복 경쟁에서 이길 수 없다.

**결정:** 원문 전용 페이지는 `noindex, follow`. 번역 페이지가 canonical이자 색인 대상.
승인 커버리지가 임계값 미만인 챕터도 `noindex`.

**결과:** 크롤 예산이 고유 콘텐츠(번역문)에 집중된다. 임계값 수치는 미정(ADR 추가 필요).

---

## ADR-008 — 스토리지는 Cloudflare R2
**상태:** Accepted

**맥락:** Railway에는 오브젝트 스토리지가 없다. 원본 HTML 스냅샷을 보관해야 하고,
재파싱 때마다 다시 읽는다.

**결정:** Cloudflare R2. S3 호환이라 `aws-sdk-go-v2`를 그대로 쓴다.

**결과:** egress가 무료라 재파싱 비용 부담이 없다.

---

## ADR-009 — API는 스펙 우선 (OpenAPI)
**상태:** Accepted

**맥락:** Go 백엔드와 TS 프론트의 타입을 일치시켜야 한다.

**결정:** `openapi.yaml`이 단일 원본. `oapi-codegen`으로 Go 인터페이스를,
`orval`로 TS 클라이언트와 react-query 훅을 생성한다.

**결과:** 계약 변경이 양쪽에서 컴파일 에러로 드러난다. 생성 코드는 손으로 고치지 않는다.

---

## ADR-010 — LLM 번역 제안은 초기 범위에서 제외한다
**상태:** Accepted

**맥락:** 문단별 LLM 번역 제안을 검토했으나 초기 기획에서 뺐다.
개인 사용자는 LLM 요청 권한이 없다.

**결정:** 제안 생성은 전부 사람 손. `llm_suggestions` 테이블과 SSE 스트리밍,
레이트리밋, Anthropic SDK 의존성 모두 넣지 않는다.

**결과:** API가 평범한 JSON 엔드포인트만으로 구성된다.
`book_glossary`는 남긴다 — 여러 사용자가 같은 책을 번역할 때 인명·호칭 일관성 문제가
사람 번역에서도 동일하게 발생하며, 나중에 LLM을 붙일 때 프롬프트 프리픽스로 재사용된다.

---

## ADR-011 — 파서 검증을 스켈레톤보다 먼저 한다
**상태:** Accepted

**맥락:** Gutenberg HTML의 자동 분리 성공률이 설계를 바꾼다.
성공률이 높으면 수동 검토는 예외 처리 수준이면 되고, 낮으면 관리자 보정 UI가 핵심 기능이 되며
스키마에 보정 레이어가 필요해진다.

**결정:** `cmd/parsecheck` 스파이크로 실제 도서 15~20권을 추출해 리포트를 먼저 낸다.
그 결과를 보고 스켈레톤을 만든다.

**결과:** 스파이크의 `extractor.go`는 버리지 않고 `internal/parse/`로 이관한다.
원본 HTML은 커밋하지 않고 `corpus.json`(도서 ID)과 `golden/`(추출 결과 요약)만 남긴다.

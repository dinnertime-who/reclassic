<!--
PR 제목은 Conventional Commits 를 따릅니다 — squash 머지라 그대로 커밋 제목이 됩니다.
  feat: 세션 인증 — Google 로그인으로 임시 헤더를 걷어낸다
규칙 전문: docs/CONVENTIONS.md "기여 흐름"
-->

## 왜

<!-- 무엇을 했는지가 아니라 왜 했는지. "무엇"은 diff 가 말해줍니다. -->

## 검증한 것

<!--
수치로 적습니다. "테스트 통과"가 아니라:
  golden 22권 일치 / 승계율 100% (소실 0) / 동시 승인 40라운드 중 어긋남 0
실물로 확인한 것과 테스트로만 덮은 것을 구분해 주세요.
-->

## 남는 위험 · 미검증

<!-- 없으면 "없음". 있으면 나중에 이것만 읽게 됩니다. -->

## 체크리스트

- [ ] `make lint && make test && make docs-check` 통과
- [ ] **`DATABASE_URL` 없이도** `make test` 통과
- [ ] 설계 판단이 있었으면 `docs/decisions/ADR-NNN.md` 추가 + 색인에 줄 추가
- [ ] 알면서 남긴 것이 있으면 `docs/tech-debt.md`에 기록
- [ ] `openapi.yaml`을 고쳤으면 `make generate` 산출물까지 커밋
- [ ] 마이그레이션을 추가했다면 **기존 파일은 수정하지 않았다**
- [ ] 파서를 고쳤으면 golden 눈 검증 + `make succession`으로 승계 영향 측정
- [ ] 새 환경변수를 `.env.example`에 추가
- [ ] 새 명령을 Makefile과 `AGENTS.md` 명령어 표에 반영

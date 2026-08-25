# 슬라이스 명세 — 세션 인증 (Google 로그인)

**작업 종류:** 구현 슬라이스 (수직)
**선행 슬라이스:** `docs/slices/completed/translation.md` — `users`와 역할 검사가 이미 있다.
이 슬라이스는 그것을 **진짜로 작동하게** 만든다.
**선행 문서:** `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/decisions/index.md` (특히 ADR-005 / 026)
**다음 슬라이스:** `docs/slices/completed/deploy.md` — 이 슬라이스가 끝나야 배포할 수 있다.

---

## 1. 이 작업이 답해야 할 질문

권한 체계는 슬라이스 4에서 이미 만들었다. `role`이 `member|reviewer|admin`이고
검수 핸들러가 실제로 403을 낸다. **그런데 모래 위에 서 있다** —
신원이 `X-User-Handle` 헤더라 아무나 `bob`을 자칭하면 검수자가 된다.

> **1. 이 사람이 정말 그 사람인가?**
> **2. 권한을 즉시 회수할 수 있는가?**

2번이 세션 저장소를 가른 이유다. 검수 권한이 있는 서비스에서
"이 계정 지금 끊어라"가 안 되면 사고가 났을 때 할 수 있는 게 없다.

**이 슬라이스가 끝나야 프로덕션 배포가 가능하다.** 지금은 문서 세 곳이 금지하고 있다.

---

## 2. 범위

### 하는 것

- Google OAuth 로그인 (ADR-027). **비밀번호를 다루지 않는다**
- Postgres 세션 테이블 + HttpOnly 쿠키
- 첫 로그인 시 계정 자동 생성 (`member`)
- **지정된 Google 계정에 `admin` 부여** — 마스터 계정
- `X-User-Handle`과 `X-Admin-Token`을 **걷어낸다**
- 관리자 엔드포인트를 세션 + 역할 검사로 바꾼다
- 로그인/로그아웃 화면 하나씩. 현재 사용자 표시

### 하지 않는 것 — 건드리면 안 됨

- **다른 로그인 수단.** Google 하나다. 비밀번호·매직링크·다른 OAuth 제공자 모두 없다
- **역할 관리 UI.** `reviewer` 승격은 당분간 SQL로 한다
- **편집·검수 CSR 화면.** 인증이 생겼다고 화면까지 만들지 않는다. 다음 슬라이스
- **회원 탈퇴, 프로필 편집**
- 파서·수집·번역 로직 변경

### 의존성

| 패키지 | 근거 |
|---|---|
| `golang.org/x/oauth2` (+ `/google`) | 인가 코드 교환. 손으로 만들지 않는다 |

**ID 토큰(JWT)을 직접 검증하지 않는다.** 코드 교환은 서버 대 서버 TLS이므로,
받은 액세스 토큰으로 Google userinfo를 한 번 부르면 된다.
JWKS 캐싱과 서명 검증을 들이지 않는 만큼 틀릴 여지가 준다.

---

## 3. 산출물

| 경로 | 내용 |
|---|---|
| `internal/db/migrations/00003_auth.sql` | `sessions` 테이블 + `users`에 Google 식별자 |
| `internal/auth/` | OAuth 흐름, 세션 발급·검증, 쿠키 |
| `internal/api/` | 로그인/콜백/로그아웃/me 핸들러, 세션 미들웨어 |
| `openapi.yaml` | `/auth/*`. 관리자 엔드포인트의 security 교체 |
| `web/` | 로그인 화면, 현재 사용자 표시 |
| `.env.example` | Google 클라이언트, 마스터 이메일, 쿠키 설정 |
| Makefile · `AGENTS.md` | 함께 갱신 |

---

## 4. 구현 명세

### 4.1 스키마

```sql
ALTER TABLE users ADD COLUMN email      TEXT;
ALTER TABLE users ADD COLUMN google_sub TEXT;   -- Google의 안정 식별자
```

- **`google_sub`으로 사용자를 찾는다. 이메일이 아니다.** 이메일은 바뀔 수 있고
  Google이 재사용하지 않는다고 보장하는 것은 `sub`뿐이다.
- `handle`은 이메일 로컬 파트에서 만들고 충돌하면 숫자를 붙인다.

```sql
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,      -- 토큰의 sha256. 토큰 원본이 아니다
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_agent  TEXT        NOT NULL DEFAULT ''
);
```

**세션 토큰을 그대로 저장하지 않는다.** 해시를 저장한다.
DB가 유출돼도 그 값으로 로그인할 수 없다. 비밀번호와 같은 이유다.

### 4.2 OAuth 흐름

```
GET  /auth/google/start     → state 쿠키 발급 후 Google로 리다이렉트
GET  /auth/google/callback  → state 대조 → 코드 교환 → userinfo → 세션 발급 → 웹으로
POST /auth/logout           → 세션 삭제
GET  /auth/me               → 현재 사용자 (비로그인 401)
```

- **`state`는 CSRF 방어다.** 짧은 수명 쿠키에 담고 콜백에서 상수 시간 비교한다.
  없거나 다르면 즉시 실패. 이걸 빼면 로그인 CSRF가 열린다.
- 콜백은 API(:8080)가 받는다. 클라이언트 시크릿을 쓰기 때문이다.
  처리 후 웹으로 리다이렉트한다.

### 4.3 마스터 관리자

`ADMIN_EMAIL`과 일치하는 Google 계정으로 로그인하면 `admin`을 부여한다.

- **매 로그인마다 확인한다.** 환경변수에서 빼면 다음 로그인에 권한이 사라진다.
- 이메일 대조는 Google이 `email_verified: true`를 준 경우에만 한다.
- 비어 있으면 기동 실패. 관리자가 아무도 없는 배포는 사고다.

### 4.4 세션 쿠키

| 속성 | 값 | 이유 |
|---|---|---|
| `HttpOnly` | 항상 | JS가 읽으면 XSS 하나로 세션이 털린다 |
| `Secure` | `COOKIE_SECURE`로 제어 | 로컬 http에서는 꺼야 붙는다. **프로덕션에서 켜는 것을 잊지 말 것** |
| `SameSite` | `Lax` | OAuth 리다이렉트가 top-level GET이라 Lax로 충분하다 |
| `Domain` | `COOKIE_DOMAIN` | 웹과 API가 다른 서브도메인이라 상위 도메인으로 공유해야 한다 |
| `Path` | `/` | |

만료는 30일. 갱신은 하지 않는다 — 필요해지면 그때 넣는다.

### 4.5 임시 장치 제거

- **`X-User-Handle`을 없앤다.** 세션에서 사용자를 읽는다.
- **`X-Admin-Token`을 없앤다.** 관리자 엔드포인트는 세션 + `role='admin'`이다.
  ADR-026의 `AllowedHeaders`에서도 뺀다.
- `ADMIN_TOKEN` 환경변수를 지운다.

**둘 다 남겨두지 않는다.** 우회로를 하나라도 두면 인증을 붙인 의미가 없다.

### 4.6 화면

- `/login` — "Google로 로그인" 링크 하나
- 루트 레이아웃에 현재 사용자 표시와 로그아웃
- **로그인 화면을 색인시키지 않는다** (`noindex`)
- 읽기 화면은 비로그인도 볼 수 있다. 번역 제안만 로그인이 필요하다

### 4.7 테스트

| 종류 | 대상 | 필요 |
|---|---|---|
| 단위 | state 검증, handle 생성·충돌, 쿠키 속성, 마스터 이메일 판정 | 없음 |
| 통합 | 세션 발급·조회·만료·삭제, 역할 부여, 만료 세션 거부 | Postgres |

**Google을 실제로 부르는 테스트는 만들지 않는다.** userinfo 응답을 대역으로 바꾼다.

---

## 5. 반드시 지킬 것

1. **세션 토큰을 평문으로 저장하지 않는다.** 해시만 저장한다.
2. **`state` 검증을 빼지 않는다.** 로그인 CSRF가 열린다.
3. **쿠키는 `HttpOnly`.** 프로덕션에서는 `Secure`도 켠다.
4. **`google_sub`으로 사용자를 찾는다.** 이메일은 바뀐다.
5. **임시 헤더 둘을 완전히 제거한다.** 우회로를 남기지 않는다.
6. **`ADMIN_EMAIL`이 없으면 기동하지 않는다.**
7. **`DATABASE_URL` 없이 `make test`가 통과해야 한다.**
8. 작업 종료 전 `make lint && make test` 통과.

---

## 6. 완료 조건

- [x] `make migrate`가 `sessions`와 `users`의 Google 컬럼을 만든다
- [x] Google로 로그인하면 계정이 자동 생성된다 (`handle`은 이메일 로컬 파트에서 파생)
- [x] `ADMIN_EMAIL` 계정으로 로그인하면 `admin`이 된다
- [x] 세션 쿠키가 `HttpOnly`이고, DB에는 토큰이 아니라 해시가 있다
- [x] `state`가 없거나 틀리면 콜백이 실패한다
- [x] 만료된 세션으로는 인증되지 않는다
- [x] **로그아웃하면 그 세션이 즉시 무효가 된다** (권한 즉시 회수)
- [x] `X-User-Handle`과 `X-Admin-Token`이 코드·문서·환경변수에서 사라졌다
- [x] 비로그인으로 검수하면 401, `member`가 검수하면 403
- [x] 비로그인도 읽기 화면은 볼 수 있다
- [x] `DATABASE_URL` 없이 `make test` 통과
- [x] `make lint && make test` 통과

---

> **실물 검증 (2026-08-20).** 실제 Google 클라이언트로 로그인해 확인했다.
>
> | 확인 항목 | 결과 |
> |---|---|
> | 계정 자동 생성 | `dinnertime-dev` / `TaeHun Bak` / `google_sub` 저장됨 |
> | 마스터 승격 | `ADMIN_EMAIL` 일치 → `role=admin` |
> | 세션 저장 형태 | id 길이 64 = sha256 hex. **평문 토큰 없음** |
> | 쿠키 `HttpOnly` | `document.cookie`에 세션 쿠키가 보이지 않음 |
> | 만료 | 발급 +30일 |
> | 검수 API (admin) | 404 — 인증·권한을 통과하고 제안이 없어서 난 404다 |
> | 관리자 API (admin) | 201 |
> | 세션바 | `TaeHun Bak · admin` |
>
> **남은 것 하나:** `ADMIN_EMAIL`과 다른 계정이 `member`로 생성되는 경로는
> 계정이 하나뿐이라 실물로 못 봤다. `roleFor`·`promote`의 단위 테스트가 덮고 있고,
> `ADMIN_EMAIL`을 잠시 비우고 재로그인하면 "역할을 매 로그인마다 재판정한다"까지
> 함께 확인된다.

## 7. 다음

- **배포 (Railway) → `docs/slices/completed/deploy.md`.** 이 슬라이스가 끝나야 시작할 수 있었다
- 편집·검수 CSR 화면 (이제 만들 수 있다) — 아직 명세 없음
- 역할 관리 UI, 도서 목록·검색, 관리자 확인 큐 화면 — 아직 명세 없음

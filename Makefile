# reclassic — 모든 작업의 단일 진입점
#
# 사람과 AI 에이전트 모두 이 Makefile을 통해서만 명령을 실행한다.
# `go test ./...` 같은 명령을 직접 추론해서 쓰지 말 것 (AGENTS.md 참조).

SHELL := /bin/bash
.DEFAULT_GOAL := help

GO      ?= go
PNPM    ?= pnpm
WEB     ?= web
CACHE   ?= .cache/gutenberg
CORPUS  ?= internal/parse/testdata/corpus.json

# web 의존성 설치 완료 스탬프 (web-install 참조). node_modules 안이라 .gitignore가 이미 덮는다.
WEB_STAMP := $(WEB)/node_modules/.install-stamp

# 같은 네트워크의 다른 기기에서 접속할 때 쓸 주소.
# 인터페이스 이름을 넘겨짚지 않는다 — 기본 라우트가 쓰는 것을 찾는다 (en0일 수도 en1일 수도 있다).
# 못 찾으면 LAN_IP=192.168.x.x 로 직접 지정한다.
LAN_IP ?= $(shell \
	iface=$$(route -n get default 2>/dev/null | awk '/interface:/{print $$2}'); \
	[ -n "$$iface" ] && ipconfig getifaddr $$iface 2>/dev/null \
	|| hostname -I 2>/dev/null | awk '{print $$1}')

# 웹 프로세스에 넘길 환경변수. Go는 .env를 직접 읽지만(internal/config) Node는 읽지 않는다.
# .env를 통째로 source하지 않는 이유는 User-Agent 값에 셸 메타문자가 있기 때문이다.
WEB_ENV = $(shell test -f .env && grep -E '^(API_INTERNAL_HOST|API_PORT|VITE_API_URL|VITE_LOGIN_URL)=' .env | tr '\n' ' ')

# 빌드 타임 도구는 go.mod의 tool 디렉티브로 고정한다 (ADR-020).
# 별도 설치가 필요 없고 버전이 저장소에 박힌다.
SQLC    := $(GO) tool sqlc
OAPI    := $(GO) tool oapi-codegen

# GitHub 작업(PR·이슈)은 계정을 고정한다. git push는 SSH 키를 쓰지만
# gh는 머신 전역 활성 계정의 HTTPS 토큰을 쓰기 때문이다. 이유 전문은 scripts/gh 주석.
GH      := ./scripts/gh

# 웹 이미지는 VITE_ 값을 빌드 시점에 번들에 박는다 (ADR-032).
# 로컬 확인 빌드에서는 .env 값을 쓴다. Railway에서는 서비스 변수가 같은 이름의 ARG로 들어간다.
VITE_API_URL   ?= $(shell test -f .env && grep -E '^VITE_API_URL=' .env | cut -d= -f2-)
VITE_LOGIN_URL ?= $(shell test -f .env && grep -E '^VITE_LOGIN_URL=' .env | cut -d= -f2-)

# 필수 도구가 없으면 설치 방법을 알려주고 멈춘다
define need
	@command -v $(1) >/dev/null 2>&1 || { \
		echo "✗ '$(1)' 가 필요합니다. 설치: $(2)"; exit 1; }
endef

.PHONY: help
help: ## 사용 가능한 명령 보기
	@echo "reclassic — 사용 가능한 명령"
	@echo
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: doctor
doctor: ## 개발 환경 점검
	@echo "── 개발 환경 점검 ──"
	@command -v $(GO)     >/dev/null 2>&1 && echo "✓ go      $$($(GO) version | awk '{print $$3}')" || echo "✗ go      — brew install go"
	@command -v docker    >/dev/null 2>&1 && echo "✓ docker" || echo "✗ docker  — https://docs.docker.com/desktop/"
	@command -v node      >/dev/null 2>&1 && echo "✓ node    $$(node --version)" || echo "✗ node    — brew install node"
	@command -v $(PNPM)   >/dev/null 2>&1 && echo "✓ pnpm    $$(cd $(WEB) && $(PNPM) --version) (web/package.json 고정, ADR-019)" || echo "✗ pnpm    — corepack enable"
	@$(SQLC) version >/dev/null 2>&1 && echo "✓ sqlc    $$($(SQLC) version) (go tool)" || echo "✗ sqlc    — go.mod tool 디렉티브 확인 (ADR-020)"
	@$(OAPI) --version >/dev/null 2>&1 && echo "✓ oapi-codegen $$($(OAPI) --version | tail -1) (go tool)" || echo "✗ oapi-codegen — go.mod tool 디렉티브 확인 (ADR-020)"
	@command -v golangci-lint >/dev/null 2>&1 && echo "✓ golangci-lint" || echo "· golangci-lint — 미설치 시 go vet 으로 대체. brew install golangci-lint"
	@gh auth token --user dinnertime-who >/dev/null 2>&1 && echo "✓ gh      dinnertime-who (scripts/gh 로 고정)" || echo "✗ gh      — brew install gh && gh auth login (dinnertime-who)"

.PHONY: build
build: ## 바이너리 빌드 (bin/)
	$(call need,$(GO),brew install go)
	@test -f go.mod || { echo "✗ go.mod 없음. 아직 Go 모듈이 초기화되지 않았습니다."; exit 1; }
	$(GO) build -o bin/ ./cmd/...

.PHONY: test
test: web-install ## 테스트 실행 (Go + web)
	$(call need,$(GO),brew install go)
	@test -f go.mod || { echo "✗ go.mod 없음."; exit 1; }
	@echo "── go test ──"
	$(GO) test ./...
	@# web 테스트는 DATABASE_URL 없이, API 없이 돈다 (ADR-035 · PR 체크리스트 2).
	@echo "── vitest ──"
	cd $(WEB) && $(PNPM) run test

.PHONY: lint
lint: web-install ## 린트 (Go + tsc + ESLint)
	$(call need,$(GO),brew install go)
	@test -f go.mod || { echo "✗ go.mod 없음."; exit 1; }
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run \
		|| { echo "· golangci-lint 미설치 — go vet 으로 대체"; $(GO) vet ./...; }
	@echo "── tsc ──"
	cd $(WEB) && $(PNPM) run typecheck
	@echo "── eslint ──"
	cd $(WEB) && $(PNPM) run lint

.PHONY: docs-check
docs-check: ## 문서 구조 검사 (ADR 번호·색인·끊어진 링크·AGENTS.md 길이)
	@./scripts/docs-check

.PHONY: fmt
fmt: web-install ## 포맷 (Go + web)
	$(call need,$(GO),brew install go)
	@echo "── go fmt ──"
	$(GO) fmt ./...
	@# 웹은 포매터를 따로 두지 않는다 (ADR-035에 없다). ESLint의 자동 수정으로 갈음한다.
	@echo "── eslint --fix ──"
	cd $(WEB) && $(PNPM) run fmt

.PHONY: generate
generate: web-install ## 코드 생성 (sqlc / oapi-codegen / orval)
	@echo "── sqlc ──"
	$(SQLC) generate
	@echo "── oapi-codegen ──"
	$(OAPI) -config oapi-codegen.yaml openapi.yaml
	@echo "── orval ──"
	cd $(WEB) && $(PNPM) run generate-api

.PHONY: migrate
migrate: ## 마이그레이션 적용 (goose → River 순, ADR-017/022)
	$(call need,$(GO),brew install go)
	$(GO) run ./tools/migrate

.PHONY: dev
dev: ## 로컬 의존 서비스 기동 (Postgres + MinIO)
	$(call need,docker,https://docs.docker.com/desktop/)
	docker compose up -d --wait
	@echo
	@echo "Postgres(:5432) · MinIO(:9000, 콘솔 :9001) 기동 완료. 다음 순서로 띄운다:"
	@echo "  make migrate     # goose → River"
	@echo "  make run-api     # :8080"
	@echo "  make run-worker  # 잡 소비"
	@echo "  make run-web     # :3100  (SSR)"

.PHONY: dev-down
dev-down: ## 로컬 의존 서비스 정지
	docker compose down

.PHONY: run-api
run-api: ## API 서버 실행 (:8080)
	$(GO) run ./cmd/api

.PHONY: run-worker
run-worker: ## 워커 실행 (아직 잡을 소비하지 않는다)
	$(GO) run ./cmd/worker

.PHONY: run-web
run-web: web-install ## 웹 개발 서버 실행 (:3100, SSR). SSH 포워딩으로도 붙는다
	@# 127.0.0.1에 붙인다. Vite 기본값은 localhost인데 macOS에서 [::1]로만 열려서
	@# `ssh -L 3100:localhost:3100`이 IPv4로 붙을 때 연결이 거부된다.
	@# 특정 주소이므로 v6only=0이어도 IPv4가 매핑되지 않는다 — Go 서버의 [::]와 다르다.
	cd $(WEB) && env $(WEB_ENV) $(PNPM) run dev --host 127.0.0.1

.PHONY: run-web-lan
run-web-lan: web-install ## 웹 개발 서버를 LAN에 노출 (휴대폰 등에서 접속). LAN_IP=192.168.x.x 로 지정 가능
	@test -n "$(LAN_IP)" || { echo "✗ LAN IP를 찾지 못했습니다. 'make run-web-lan LAN_IP=192.168.x.x' 로 지정하세요."; exit 1; }
	@echo "── 같은 네트워크의 다른 기기에서 ──"
	@echo "  웹  : http://$(LAN_IP):3100"
	@echo "  API : http://$(LAN_IP):8080  (Go 서버는 [::] 바인딩이라 이미 열려 있습니다)"
	@echo
	@echo "⚠ 개발 서버입니다. ADMIN_TOKEN과 X-User-Handle 신원이 그대로 노출됩니다."
	@echo "  신뢰하는 네트워크에서만 쓰세요."
	@echo
	cd $(WEB) && env $(WEB_ENV) VITE_API_URL=http://$(LAN_IP):8080 $(PNPM) run dev --host

.PHONY: web-install
web-install: $(WEB_STAMP) ## web/ 의존성 설치 (lockfile 이 설치보다 새로울 때만)
	$(call need,$(PNPM),corepack enable)

# 설치 완료 스탬프. `test -d node_modules`로 판정하면 **디렉터리가 있기만 하면 건너뛰어서**
# 브랜치를 옮겨 의존성이 늘어도, 캐시가 있는 CI에서도 낡은 채로 진행한다.
# make의 타깃 의존 규칙에 맡긴다 — lockfile·package.json이 스탬프보다 새로우면 다시 깐다.
#
# 스탬프를 node_modules 안에 두는 이유: node_modules를 지우면 스탬프도 같이 사라져
# 반드시 다시 깔린다. 밖에 두면 "스탬프는 있는데 node_modules는 없는" 상태가 생긴다.
$(WEB_STAMP): $(WEB)/pnpm-lock.yaml $(WEB)/package.json
	$(call need,$(PNPM),corepack enable)
	@# --frozen-lockfile 을 뗄 수 없다 (ADR-019). lockfile 이 유일한 진실이다.
	cd $(WEB) && $(PNPM) install --frozen-lockfile
	@touch $@

.PHONY: web-add
web-add: ## web/ 의존성 추가 (예: make web-add PKG=@fontsource/noto-serif-kr, DEV=1 이면 devDependencies)
	$(call need,$(PNPM),corepack enable)
	@test -n "$(PKG)" || { echo "✗ 패키지 이름이 필요합니다. 예: make web-add PKG=@fontsource/noto-serif-kr"; exit 1; }
	@# 여기만 lockfile을 고친다. `web-install`은 --frozen-lockfile 전용으로 남는다 (ADR-019) —
	@# lockfile이 유일한 진실이라는 성질은 "아무나 못 고친다"가 아니라 "고치는 자리가 하나다"로 지킨다.
	cd $(WEB) && $(PNPM) add $(if $(DEV),--save-dev,) $(PKG)
	@# 방금 깔았으므로 스탬프를 앞세운다. 안 하면 다음 web-install이 통째로 다시 깐다.
	@touch $(WEB_STAMP)
	@echo
	@echo "⚠ $(WEB)/package.json 과 $(WEB)/pnpm-lock.yaml 이 바뀌었습니다. **둘 다 커밋하세요.**"
	@echo "  의존성을 늘리는 판단은 ADR 감입니다 (AGENTS.md 작업 규칙 2)."

.PHONY: ui-add
ui-add: web-install ## shadcn 컴포넌트 추가 (예: make ui-add C=button). 먼저 shadcn에 있는지 찾을 것 (ADR-035)
	@test -n "$(C)" || { echo "✗ 컴포넌트 이름이 필요합니다. 예: make ui-add C=button"; exit 1; }
	@# 이미 있는 컴포넌트를 다시 add하면 손으로 고친 내용이 덮어써집니다 (ADR-035).
	@for c in $(C); do \
		test ! -f $(WEB)/src/components/ui/$$c.tsx || { \
			echo "✗ $$c 는 이미 있습니다. 다시 add하면 수정한 내용이 덮어써집니다 (ADR-035)."; \
			echo "  정말 갈아엎으려면 파일을 먼저 지우세요."; exit 1; }; \
	done
	cd $(WEB) && $(PNPM) dlx shadcn@latest add $(C)

.PHONY: ingest
ingest: ## 캐시된 원문을 파싱해 DB에 적재 (멱등). ONLY=1342 로 한 권만
	$(call need,$(GO),brew install go)
	$(GO) run ./cmd/ingest run -corpus=$(CORPUS) -cache=$(CACHE) $(if $(ONLY),-only=$(ONLY))

.PHONY: succession
succession: ## stable_id 승계 매칭률 측정 (읽기 전용). ONLY=1342 로 한 권만
	$(call need,$(GO),brew install go)
	$(GO) run ./cmd/ingest succession -corpus=$(CORPUS) -cache=$(CACHE) $(if $(ONLY),-only=$(ONLY))

.PHONY: fetch-corpus
fetch-corpus: ## 검증용 Gutenberg 도서를 .cache/ 로 내려받기 (커밋 안 함)
	$(call need,$(GO),brew install go)
	@test -f $(CORPUS) || { echo "✗ $(CORPUS) 없음."; exit 1; }
	@mkdir -p $(CACHE)
	$(GO) run ./cmd/parsecheck fetch -corpus=$(CORPUS) -cache=$(CACHE)

.PHONY: parsecheck
parsecheck: ## 파서 검증 리포트 생성
	$(call need,$(GO),brew install go)
	$(GO) run ./cmd/parsecheck report -corpus=$(CORPUS) -cache=$(CACHE) -out=.cache/report.html

.PHONY: golden
golden: ## golden 스냅샷 비교 (GOLDEN_UPDATE=1 이면 갱신)
	$(call need,$(GO),brew install go)
	@test -f $(CORPUS) || { echo "✗ $(CORPUS) 없음."; exit 1; }
	@if [ "$(GOLDEN_UPDATE)" = "1" ]; then \
		$(GO) run ./cmd/parsecheck golden -corpus=$(CORPUS) -cache=$(CACHE) -update; \
	else \
		$(GO) run ./cmd/parsecheck golden -corpus=$(CORPUS) -cache=$(CACHE); \
	fi

.PHONY: docker-build
docker-build: ## Go 이미지 빌드 (api·worker·migrate 셋이 한 이미지에, ADR-031)
	$(call need,docker,https://docs.docker.com/desktop/)
	docker build -t reclassic-go .

.PHONY: docker-build-web
docker-build-web: ## 웹 이미지 빌드 (컨텍스트는 저장소 루트, ADR-029)
	$(call need,docker,https://docs.docker.com/desktop/)
	@test -n "$(VITE_API_URL)"   || { echo "✗ VITE_API_URL 이 비어 있습니다 (.env 확인)"; exit 1; }
	@test -n "$(VITE_LOGIN_URL)" || { echo "✗ VITE_LOGIN_URL 이 비어 있습니다 (.env 확인)"; exit 1; }
	docker build -f $(WEB)/Dockerfile \
		--build-arg VITE_API_URL=$(VITE_API_URL) \
		--build-arg VITE_LOGIN_URL=$(VITE_LOGIN_URL) \
		-t reclassic-web .

.PHONY: pr
pr: ## PR 생성 (계정 고정, .github/pull_request_template.md 사용)
	$(call need,gh,brew install gh)
	$(GH) pr create --base main

.PHONY: clean
clean: ## 빌드 산출물 삭제 (.cache는 남김)
	rm -rf bin/ $(WEB)/.output $(WEB)/.nitro $(WEB)/dist

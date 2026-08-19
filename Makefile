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

# 웹 프로세스에 넘길 환경변수. Go는 .env를 직접 읽지만(internal/config) Node는 읽지 않는다.
# .env를 통째로 source하지 않는 이유는 User-Agent 값에 셸 메타문자가 있기 때문이다.
WEB_ENV = $(shell test -f .env && grep -E '^(API_INTERNAL_HOST|API_PORT|VITE_API_URL)=' .env | tr '\n' ' ')

# 빌드 타임 도구는 go.mod의 tool 디렉티브로 고정한다 (ADR-020).
# 별도 설치가 필요 없고 버전이 저장소에 박힌다.
SQLC    := $(GO) tool sqlc
OAPI    := $(GO) tool oapi-codegen

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

.PHONY: build
build: ## 바이너리 빌드 (bin/)
	$(call need,$(GO),brew install go)
	@test -f go.mod || { echo "✗ go.mod 없음. 아직 Go 모듈이 초기화되지 않았습니다."; exit 1; }
	$(GO) build -o bin/ ./cmd/...

.PHONY: test
test: ## 테스트 실행
	$(call need,$(GO),brew install go)
	@test -f go.mod || { echo "✗ go.mod 없음."; exit 1; }
	$(GO) test ./...

.PHONY: lint
lint: web-install ## 린트 (Go + TS 타입 검사)
	$(call need,$(GO),brew install go)
	@test -f go.mod || { echo "✗ go.mod 없음."; exit 1; }
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run \
		|| { echo "· golangci-lint 미설치 — go vet 으로 대체"; $(GO) vet ./...; }
	@echo "── tsc ──"
	cd $(WEB) && $(PNPM) run typecheck

.PHONY: fmt
fmt: ## 포맷
	$(call need,$(GO),brew install go)
	$(GO) fmt ./...

.PHONY: generate
generate: web-install ## 코드 생성 (sqlc / oapi-codegen / orval)
	@echo "── sqlc ──"
	$(SQLC) generate
	@echo "── oapi-codegen ──"
	$(OAPI) -config oapi-codegen.yaml openapi.yaml
	@echo "── orval ──"
	cd $(WEB) && $(PNPM) run generate-api

.PHONY: migrate
migrate: ## 마이그레이션 적용 (goose, ADR-017)
	$(call need,$(GO),brew install go)
	$(GO) run ./tools/migrate

.PHONY: dev
dev: ## 로컬 의존 서비스 기동 (Postgres)
	$(call need,docker,https://docs.docker.com/desktop/)
	docker compose up -d --wait
	@echo
	@echo "Postgres 기동 완료. 다음 순서로 띄운다:"
	@echo "  make migrate    # 스키마 적용"
	@echo "  make run-api    # :8080"
	@echo "  make run-web    # :3000  (SSR)"

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
run-web: web-install ## 웹 개발 서버 실행 (:3000, SSR)
	cd $(WEB) && env $(WEB_ENV) $(PNPM) run dev

.PHONY: web-install
web-install: ## web/ 의존성 설치 (없을 때만)
	$(call need,$(PNPM),corepack enable)
	@test -d $(WEB)/node_modules || { cd $(WEB) && $(PNPM) install --frozen-lockfile; }

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

.PHONY: clean
clean: ## 빌드 산출물 삭제 (.cache는 남김)
	rm -rf bin/ $(WEB)/.output $(WEB)/.nitro $(WEB)/dist

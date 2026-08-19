# reclassic — 모든 작업의 단일 진입점
#
# 사람과 AI 에이전트 모두 이 Makefile을 통해서만 명령을 실행한다.
# `go test ./...` 같은 명령을 직접 추론해서 쓰지 말 것 (AGENTS.md 참조).

SHELL := /bin/bash
.DEFAULT_GOAL := help

GO      ?= go
CACHE   ?= .cache/gutenberg
CORPUS  ?= internal/parse/testdata/corpus.json

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
	@command -v sqlc      >/dev/null 2>&1 && echo "✓ sqlc" || echo "· sqlc    — (아직 불필요) brew install sqlc"
	@command -v golangci-lint >/dev/null 2>&1 && echo "✓ golangci-lint" || echo "· golangci-lint — (아직 불필요) brew install golangci-lint"

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
lint: ## 린트
	$(call need,$(GO),brew install go)
	@test -f go.mod || { echo "✗ go.mod 없음."; exit 1; }
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run \
		|| { echo "· golangci-lint 미설치 — go vet 으로 대체"; $(GO) vet ./...; }

.PHONY: fmt
fmt: ## 포맷
	$(call need,$(GO),brew install go)
	$(GO) fmt ./...

.PHONY: generate
generate: ## 코드 생성 (sqlc / oapi-codegen / orval)
	@echo "TODO: 스키마와 openapi.yaml이 생기면 연결한다"
	@echo "  sqlc generate"
	@echo "  oapi-codegen -config oapi.yaml openapi.yaml"
	@echo "  cd web && npx orval"

.PHONY: migrate
migrate: ## 마이그레이션 적용
	@echo "TODO: 마이그레이션 도구 결정 후 연결 (ADR 추가 필요)"

.PHONY: dev
dev: ## 로컬 실행 (docker compose)
	@test -f docker-compose.yml || { echo "✗ docker-compose.yml 없음. 스켈레톤 생성 후 사용 가능."; exit 1; }
	docker compose up

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
	rm -rf bin/

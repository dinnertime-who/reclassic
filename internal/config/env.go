// Package config는 환경변수 로딩을 담당한다.
//
// `.env` 값에는 셸이 해석하지 못하는 문자가 들어간다(예: User-Agent의 괄호).
// 그래서 Makefile에서 source하지 않고 프로세스가 직접 읽는다.
package config

import (
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv는 파일의 KEY=VALUE를 프로세스 환경에 채운다.
// 이미 설정된 값은 덮어쓰지 않는다 — 실제 환경이 파일을 이긴다.
// 파일이 없으면 조용히 넘어간다. 배포 환경에는 .env가 없다.
func LoadDotEnv(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

// Require는 값이 없으면 에러를 낸다.
// 기본값을 조용히 채우지 않는다 (CONVENTIONS) — 없으면 기동 시점에 실패해야 한다.
func Require(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("환경변수 %s가 비어 있다. .env를 확인할 것", key)
	}
	return v, nil
}

// RequireList는 쉼표로 구분된 값을 읽는다. 빈 항목은 버린다.
func RequireList(key string) ([]string, error) {
	raw, err := Require(key)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("환경변수 %s에 값이 없다", key)
	}
	return out, nil
}

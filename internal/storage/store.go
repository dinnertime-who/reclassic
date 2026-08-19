// Package storage는 원본 HTML 스냅샷 보관을 담당한다.
//
// 프로덕션은 Cloudflare R2다 (ADR-008). S3 호환이라 aws-sdk-go-v2를 그대로 쓰고,
// 로컬 개발에서는 같은 프로토콜의 MinIO를 붙인다.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// ObjectStore는 호출부가 스토리지에 대해 알아야 하는 전부다.
// 외부 I/O는 인터페이스 뒤에 둔다 (CONVENTIONS) — 잡 테스트에서 갈아끼운다.
type ObjectStore interface {
	Put(ctx context.Context, key string, body []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// SourceKey는 원문 스냅샷의 키다.
//
// 내용 해시를 키에 넣는다. 같은 원문을 두 번 올려도 같은 자리에 덮어써지므로
// 잡이 재시도돼도 쓰레기가 쌓이지 않는다. 잡은 멱등이어야 한다.
func SourceKey(gutenbergID int, contentHash string) string {
	return fmt.Sprintf("sources/%d/%s.html", gutenbergID, contentHash)
}

// HashContent는 SourceKey에 넣을 해시를 만든다.
// 적재 쪽 content_hash(parse.Evaluation.SourceSHA256)와 같은 값이어야 한다.
func HashContent(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

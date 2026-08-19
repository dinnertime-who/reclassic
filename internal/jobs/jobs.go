// Package jobs는 수집 파이프라인의 River 잡을 담는다.
//
// 잡을 잘게 나눈다 (ARCHITECTURE "수집 파이프라인"). 하나의 거대한 잡보다 재시도가 쉽다.
//
//	FetchSource : 원문 요청 → R2 저장 → book_sources 기록 → ParseBook enqueue
//	ParseBook   : R2에서 읽어 Ingester 호출
//
// 모든 핸들러는 멱등이어야 한다. River가 재시도하기 때문이다.
package jobs

// 큐를 나눈다. 수집은 직렬이어야 하고 파싱은 그럴 이유가 없다.
const (
	// QueueFetch의 동시성은 반드시 1이다. Gutenberg에 병렬 요청을 보내면
	// IP가 차단되고 복구가 어렵다.
	QueueFetch = "fetch"
	QueueParse = "parse"
)

// FetchSourceArgs는 원문 하나를 받아오라는 지시다.
type FetchSourceArgs struct {
	BookID      int64  `json:"book_id"`
	GutenbergID int    `json:"gutenberg_id"`
	Title       string `json:"title"`
}

func (FetchSourceArgs) Kind() string { return "fetch_source" }

// ParseBookArgs는 이미 R2에 있는 원문을 파싱해 적재하라는 지시다.
// 원문을 인자로 넘기지 않는다 — 잡 인자는 Postgres 행에 들어가고,
// 셰익스피어 전집은 7MB다.
type ParseBookArgs struct {
	BookID      int64  `json:"book_id"`
	GutenbergID int    `json:"gutenberg_id"`
	Title       string `json:"title"`
	S3Key       string `json:"s3_key"`
}

func (ParseBookArgs) Kind() string { return "parse_book" }

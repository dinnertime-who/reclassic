package translate

// DefaultIndexThreshold는 챕터 색인 기준이다 (ADR-023).
//
// 승인 문단 / 전체 문단이 이 값 이상이면 index, 미만이면 noindex.
// 코드 상수가 아니라 설정값으로 두는 이유는 색인 결과를 보고 조정하기 위함이다.
// 바꾸면 ADR-023을 갱신할 것.
const DefaultIndexThreshold = 0.80

// Coverage는 챕터 하나의 번역 진행률이다.
type Coverage struct {
	Total    int
	Approved int
}

// Ratio는 승인 비율이다. 문단이 없으면 0이다 —
// 빈 챕터를 1.0으로 치면 내용 없는 페이지가 색인된다.
func (c Coverage) Ratio() float64 {
	if c.Total == 0 {
		return 0
	}
	return float64(c.Approved) / float64(c.Total)
}

// Indexable은 이 챕터를 색인해도 되는지다 (ADR-007 + ADR-023).
//
// 원문 전용 페이지는 이 판정과 무관하게 언제나 noindex다.
// 여기서 다루는 것은 번역 페이지뿐이다.
func (c Coverage) Indexable(threshold float64) bool {
	if c.Total == 0 {
		return false
	}
	return c.Ratio() >= threshold
}

// RobotsMeta는 읽기 화면이 내보낼 robots 값이다.
func (c Coverage) RobotsMeta(threshold float64) string {
	if c.Indexable(threshold) {
		return "index, follow"
	}
	// follow는 유지한다. 색인은 안 해도 링크는 따라가게 둔다 (ADR-007).
	return "noindex, follow"
}

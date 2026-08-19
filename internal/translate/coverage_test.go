package translate

import "testing"

func TestCoverageIndexable(t *testing.T) {
	tests := []struct {
		name          string
		total         int
		approved      int
		wantIndexable bool
	}{
		{"완역", 100, 100, true},
		{"임계값 정확히", 100, 80, true},
		{"임계값 바로 아래", 100, 79, false},
		{"절반", 100, 50, false},
		{"미번역", 100, 0, false},
		{"빈 챕터는 색인하지 않는다", 0, 0, false},
		{"작은 챕터 — 4/5", 5, 4, true},
		{"작은 챕터 — 3/5", 5, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Coverage{Total: tt.total, Approved: tt.approved}
			if got := c.Indexable(DefaultIndexThreshold); got != tt.wantIndexable {
				t.Errorf("Indexable = %v, want %v (ratio %.3f)", got, tt.wantIndexable, c.Ratio())
			}

			wantMeta := "noindex, follow"
			if tt.wantIndexable {
				wantMeta = "index, follow"
			}
			if got := c.RobotsMeta(DefaultIndexThreshold); got != wantMeta {
				t.Errorf("RobotsMeta = %q, want %q", got, wantMeta)
			}
		})
	}
}

// 빈 챕터를 1.0으로 치면 내용 없는 페이지가 색인된다.
func TestEmptyChapterRatioIsZeroNotOne(t *testing.T) {
	if got := (Coverage{}).Ratio(); got != 0 {
		t.Errorf("빈 챕터 Ratio = %v, want 0", got)
	}
}

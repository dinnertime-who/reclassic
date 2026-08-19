package book

import "testing"

func TestDiffStableIDs(t *testing.T) {
	tests := []struct {
		name                 string
		stored, current      []string
		matched, added, lost int
		rate                 float64
	}{
		{
			name:    "파서 미변경 — 100%",
			stored:  []string{"a", "b", "c"},
			current: []string{"a", "b", "c"},
			matched: 3, added: 0, lost: 0, rate: 1,
		},
		{
			name:    "문단 하나가 바뀜",
			stored:  []string{"a", "b", "c"},
			current: []string{"a", "b", "z"},
			matched: 2, added: 1, lost: 1, rate: 2.0 / 3.0,
		},
		{
			name:    "순서만 바뀜 — 본문 해시라 영향 없음",
			stored:  []string{"a", "b", "c"},
			current: []string{"c", "a", "b"},
			matched: 3, added: 0, lost: 0, rate: 1,
		},
		{
			name:    "문단이 추가되기만 함",
			stored:  []string{"a", "b"},
			current: []string{"a", "b", "c", "d"},
			matched: 2, added: 2, lost: 0, rate: 1,
		},
		{
			name:    "저장본이 비었음",
			stored:  nil,
			current: []string{"a"},
			matched: 0, added: 1, lost: 0, rate: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diffStableIDs(1, "t", tt.stored, tt.current)
			if got.Matched != tt.matched || got.Added != tt.added || got.Lost != tt.lost {
				t.Errorf("matched/added/lost = %d/%d/%d, want %d/%d/%d",
					got.Matched, got.Added, got.Lost, tt.matched, tt.added, tt.lost)
			}
			if diff := got.Rate() - tt.rate; diff > 1e-9 || diff < -1e-9 {
				t.Errorf("Rate() = %v, want %v", got.Rate(), tt.rate)
			}
		})
	}
}

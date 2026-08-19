package book

import "testing"

func TestGate(t *testing.T) {
	tests := []struct {
		name       string
		chapters   int
		paragraphs int
		confidence float64
		want       bool
	}{
		{"평범한 장편", 61, 2139, 0.93, true},
		{"경계값 — 딱 임계값이면 통과", MaxChapters, MaxParagraphs, MinConfidence, true},
		{"셰익스피어 전집 (ADR-014)", 842, 39355, 0.95, false},
		{"챕터만 초과", 201, 100, 0.95, false},
		{"문단만 초과", 10, 15001, 0.95, false},
		{"신뢰도 미달", 20, 500, 0.84, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Gate(tt.chapters, tt.paragraphs, tt.confidence)
			if got.Passed != tt.want {
				t.Errorf("Passed = %v, want %v (reasons: %v)", got.Passed, tt.want, got.Reasons)
			}
			if !got.Passed && len(got.Reasons) == 0 {
				t.Error("통과하지 못했는데 이유가 비어 있다")
			}
		})
	}
}

// 2위 도서들이 임계값에 걸리지 않는지 본다. ADR-014가 "나머지 21권은 안전하다"고
// 적은 근거가 코드에서도 유지되는지 확인하는 것이다.
func TestGateRunnersUpAreSafe(t *testing.T) {
	runnersUp := []struct {
		name       string
		chapters   int
		paragraphs int
	}{
		{"2701 Moby Dick — 챕터 2위", 145, 2487},
		{"4300 Ulysses — 문단 2위", 21, 7064},
	}
	for _, b := range runnersUp {
		if got := Gate(b.chapters, b.paragraphs, 0.9); !got.Passed {
			t.Errorf("%s가 게이트에 걸렸다: %v", b.name, got.Reasons)
		}
	}
}

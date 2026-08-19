package parse

import "testing"

func TestScoreFormula(t *testing.T) {
	t.Parallel()

	sig := Signals{
		Coverage:      1,
		TitleRatio:    1,
		ChapterSanity: true,
		ParaSanity:    true,
		ShortRatio:    0,
		NoiseTitle:    false,
	}
	got := Score(sig)
	if got != 1 {
		t.Fatalf("perfect signals: got %v, want 1", got)
	}

	sig.NoiseTitle = true
	got = Score(sig)
	want := Confidence(0.85)
	if got != want {
		t.Fatalf("noise title: got %v, want %v", got, want)
	}
}

func TestVerdict(t *testing.T) {
	t.Parallel()
	if Verdict(0.85) != "auto" {
		t.Fatal("0.85 should be auto")
	}
	if Verdict(0.60) != "review" {
		t.Fatal("0.60 should be review")
	}
	if Verdict(0.59) != "fail" {
		t.Fatal("0.59 should be fail")
	}
}

func TestComputeSignalsEmpty(t *testing.T) {
	t.Parallel()
	sig := ComputeSignals(&Result{}, 100)
	if sig.Coverage != 0 || sig.ChapterSanity || sig.ParaSanity {
		t.Fatalf("unexpected signals: %+v", sig)
	}
}

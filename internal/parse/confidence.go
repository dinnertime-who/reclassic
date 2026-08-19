package parse

import (
	"math"
	"sort"
	"strings"
)

// 신뢰도 가중치와 판정 임계값. 조정하면 PARSER_REPORT에 근거를 남긴다.
const (
	WeightCoverage      = 0.45
	WeightTitleRatio    = 0.20
	WeightChapterSanity = 0.15
	WeightParaSanity    = 0.10
	WeightShortPenalty  = 0.10
	PenaltyNoiseTitle   = 0.15

	ShortRatioScale       = 0.30
	MinParaMedian         = 40
	MaxParaMedian         = 3000
	ShortParaLen          = 20
	MinAvgParasPerChapter = 3
	MinChapters           = 1

	AutoMin   = 0.85
	ReviewMin = 0.60

	SingleChapterCap = 0.50
)

// Signals는 신뢰도 공식에 들어가는 측정값이다.
type Signals struct {
	Coverage      float64
	TitleRatio    float64
	ChapterSanity bool
	ParaSanity    bool
	ShortRatio    float64
	NoiseTitle    bool
}

func ComputeSignals(res *Result, bodyChars int) Signals {
	var sig Signals
	if res == nil {
		return sig
	}

	var paraLens []int
	var titled int
	var extracted int
	for _, ch := range res.Chapters {
		if Normalize(ch.Title) != "" {
			titled++
		}
		if noiseTitle(ch.Title) {
			sig.NoiseTitle = true
		}
		for _, p := range ch.Paragraphs {
			n := len(p.Text)
			paraLens = append(paraLens, n)
			extracted += n
		}
	}

	if bodyChars > 0 {
		sig.Coverage = float64(extracted) / float64(bodyChars)
		if sig.Coverage > 1 {
			sig.Coverage = 1
		}
	}
	if n := len(res.Chapters); n > 0 {
		sig.TitleRatio = float64(titled) / float64(n)
		avg := float64(len(paraLens)) / float64(n)
		sig.ChapterSanity = n >= MinChapters && avg >= MinAvgParasPerChapter
	}
	if n := len(paraLens); n > 0 {
		med := medianInt(paraLens)
		sig.ParaSanity = med >= MinParaMedian && med <= MaxParaMedian
		var short int
		for _, l := range paraLens {
			if l < ShortParaLen {
				short++
			}
		}
		sig.ShortRatio = float64(short) / float64(n)
	}
	return sig
}

func Score(sig Signals) Confidence {
	shortTerm := 1 - math.Min(sig.ShortRatio/ShortRatioScale, 1)
	raw := WeightCoverage*sig.Coverage +
		WeightTitleRatio*sig.TitleRatio +
		WeightChapterSanity*bool01(sig.ChapterSanity) +
		WeightParaSanity*bool01(sig.ParaSanity) +
		WeightShortPenalty*shortTerm -
		PenaltyNoiseTitle*bool01(sig.NoiseTitle)
	return Confidence(clamp(raw, 0, 1))
}

func Verdict(c Confidence) string {
	switch {
	case c >= AutoMin:
		return "auto"
	case c >= ReviewMin:
		return "review"
	default:
		return "fail"
	}
}

func VerdictKO(c Confidence) string {
	switch Verdict(c) {
	case "auto":
		return "자동"
	case "review":
		return "검토"
	default:
		return "실패"
	}
}

func noiseTitle(title string) bool {
	n := strings.ToUpper(Normalize(title))
	for _, w := range []string{"CONTENTS", "INDEX", "ILLUSTRATIONS"} {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}

func bool01(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func medianInt(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	cp := append([]int(nil), vals...)
	sort.Ints(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

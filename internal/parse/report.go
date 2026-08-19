package parse

import (
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
)

func WriteHTMLReport(path string, evals []*Evaluation) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return RenderHTMLReport(f, evals)
}

func RenderHTMLReport(w io.Writer, evals []*Evaluation) error {
	var auto, review, fail int
	for _, ev := range evals {
		if ev.Best == nil || ev.Best.Result == nil {
			fail++
			continue
		}
		switch Verdict(ev.Best.Result.Confidence) {
		case "auto":
			auto++
		case "review":
			review++
		default:
			fail++
		}
	}
	n := len(evals)

	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="ko">
<head>
<meta charset="utf-8">
<title>reclassic parser report</title>
<style>
body { font-family: Georgia, serif; margin: 2rem auto; max-width: 1100px; color: #222; }
h1, h2, h3 { font-family: system-ui, sans-serif; }
.summary { display: flex; gap: 1rem; margin: 1rem 0 2rem; }
.card { border: 1px solid #ddd; padding: 1rem 1.2rem; border-radius: 6px; }
.auto { background: #e8f6e8; }
.review { background: #fff6d6; }
.fail { background: #fde8e8; }
table { border-collapse: collapse; width: 100%%; margin: 0.6rem 0 1.2rem; font-size: 0.95rem; }
th, td { border: 1px solid #ddd; padding: 0.35rem 0.5rem; text-align: left; vertical-align: top; }
th { background: #f5f5f5; }
.preview { color: #444; font-size: 0.92rem; }
.warn { color: #a33; }
.meta { color: #666; font-size: 0.9rem; }
</style>
</head>
<body>
<h1>파서 검증 리포트</h1>
<div class="summary">
<div class="card">검증 %d권</div>
<div class="card auto">자동 %d (%.0f%%)</div>
<div class="card review">검토 %d (%.0f%%)</div>
<div class="card fail">실패 %d (%.0f%%)</div>
</div>
`, n, auto, pct(auto, n), review, pct(review, n), fail, pct(fail, n))

	for _, ev := range evals {
		if err := renderBook(w, ev); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "</body></html>\n")
	return err
}

func renderBook(w io.Writer, ev *Evaluation) error {
	verdict := "실패"
	class := "fail"
	strategy := "-"
	conf := 0.0
	cov := 0.0
	if ev.Best != nil && ev.Best.Result != nil {
		verdict = VerdictKO(ev.Best.Result.Confidence)
		class = Verdict(ev.Best.Result.Confidence)
		strategy = ev.Best.Name
		conf = float64(ev.Best.Result.Confidence)
		cov = ev.Best.Signals.Coverage
	}

	_, _ = fmt.Fprintf(w, `<article class="card %s" id="book-%d">
<h2>%d · %s</h2>
<p class="meta">분류 %s · 판정 <strong>%s</strong> · 전략 %s · 신뢰도 %.3f · coverage %.3f · 챕터 %d · 문단 %d</p>
`, class, ev.GutenbergID, ev.GutenbergID, html.EscapeString(ev.Title), html.EscapeString(ev.Category),
		html.EscapeString(verdict), html.EscapeString(strategy), conf, cov, ev.ChapterCount(), ev.ParagraphCount())

	_, _ = io.WriteString(w, "<h3>전략 비교</h3><table><tr><th>전략</th><th>신뢰도</th><th>coverage</th><th>titleRatio</th><th>chapterSanity</th><th>paraSanity</th><th>shortRatio</th><th>noiseTitle</th><th>챕터</th><th>문단</th></tr>")
	for _, sc := range ev.All {
		if sc.Err != nil {
			_, _ = fmt.Fprintf(w, "<tr><td>%s</td><td colspan=\"9\" class=\"warn\">%s</td></tr>", html.EscapeString(sc.Name), html.EscapeString(sc.Err.Error()))
			continue
		}
		ch, para := 0, 0
		if sc.Result != nil {
			ch = len(sc.Result.Chapters)
			for _, c := range sc.Result.Chapters {
				para += len(c.Paragraphs)
			}
		}
		_, _ = fmt.Fprintf(w, "<tr><td>%s</td><td>%.3f</td><td>%.3f</td><td>%.3f</td><td>%v</td><td>%v</td><td>%.3f</td><td>%v</td><td>%d</td><td>%d</td></tr>",
			html.EscapeString(sc.Name),
			sc.Result.Confidence, sc.Signals.Coverage, sc.Signals.TitleRatio,
			sc.Signals.ChapterSanity, sc.Signals.ParaSanity, sc.Signals.ShortRatio, sc.Signals.NoiseTitle,
			ch, para)
	}
	_, _ = io.WriteString(w, "</table>")

	if ev.Best != nil && ev.Best.Result != nil {
		_, _ = io.WriteString(w, "<h3>챕터</h3><table><tr><th>#</th><th>제목</th><th>문단</th><th>첫 문단 앞 200자</th></tr>")
		for _, ch := range ev.Best.Result.Chapters {
			preview := ""
			if len(ch.Paragraphs) > 0 {
				preview = Preview(ch.Paragraphs[0].Text, 200)
			}
			_, _ = fmt.Fprintf(w, "<tr><td>%d</td><td>%s</td><td>%d</td><td class=\"preview\">%s</td></tr>",
				ch.Idx, html.EscapeString(ch.Title), len(ch.Paragraphs), html.EscapeString(preview))
		}
		_, _ = io.WriteString(w, "</table>")
	}

	warns := ev.Warnings
	if ev.Best != nil && ev.Best.Result != nil {
		warns = ev.Best.Result.Warnings
	}
	if len(warns) > 0 {
		_, _ = io.WriteString(w, "<h3>경고</h3><ul>")
		for _, wmsg := range warns {
			_, _ = fmt.Fprintf(w, `<li class="warn">%s</li>`, html.EscapeString(wmsg))
		}
		_, _ = io.WriteString(w, "</ul>")
	}
	_, _ = io.WriteString(w, "</article>\n")
	return nil
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(part) / float64(total)
}

func PrintSummary(w io.Writer, evals []*Evaluation) {
	var auto, review, fail int
	for _, ev := range evals {
		conf := Confidence(0)
		strategy := "-"
		if ev.Best != nil && ev.Best.Result != nil {
			conf = ev.Best.Result.Confidence
			strategy = ev.Best.Name
		}
		v := Verdict(conf)
		switch v {
		case "auto":
			auto++
		case "review":
			review++
		default:
			fail++
		}
		cov := 0.0
		if ev.Best != nil {
			cov = ev.Best.Signals.Coverage
		}
		fmt.Fprintf(w, "%4d  %-42s  %-6s  %-16s  conf=%.3f  cov=%.3f  ch=%4d  p=%5d\n",
			ev.GutenbergID, truncate(ev.Title, 42), VerdictKO(conf), strategy, conf, cov, ev.ChapterCount(), ev.ParagraphCount())
	}
	n := len(evals)
	fmt.Fprintf(w, "\n자동 %d (%.0f%%)  검토 %d (%.0f%%)  실패 %d (%.0f%%)  / %d권\n",
		auto, pct(auto, n), review, pct(review, n), fail, pct(fail, n), n)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

package parse

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestStableIDN(t *testing.T) {
	t.Parallel()
	text := Normalize("Yes.")
	if StableIDN(text, 1) != StableID(text) {
		t.Fatal("n=1 must match current StableID")
	}
	if StableIDN(text, 2) == StableID(text) {
		t.Fatal("n=2 must differ from first occurrence")
	}
	if StableIDN(text, 2) != StableID(text+"#2") {
		t.Fatal("n=2 must be sha256(text + \"#2\")")
	}
}

func TestAssignOccurrenceIDs(t *testing.T) {
	t.Parallel()
	res := &Result{Chapters: []Chapter{
		{Paragraphs: []Paragraph{{Text: "Yes."}, {Text: "No."}}},
		{Paragraphs: []Paragraph{{Text: "Yes."}}},
	}}
	assignOccurrenceIDs(res)
	first := res.Chapters[0].Paragraphs[0].StableID
	second := res.Chapters[1].Paragraphs[0].StableID
	if first == second {
		t.Fatal("repeated paragraph kept the same stable_id")
	}
	if first != StableID("Yes.") {
		t.Fatal("first occurrence changed")
	}
}

func TestDropEmptyChapters(t *testing.T) {
	t.Parallel()
	res := &Result{Chapters: []Chapter{
		{Title: "TOC", Paragraphs: nil},
		{Title: "Chapter I", Paragraphs: []Paragraph{{Text: "hello there this is a real paragraph."}}},
		{Title: "Empty", Paragraphs: []Paragraph{}},
	}}
	dropEmptyChapters(res)
	if len(res.Chapters) != 1 || res.Chapters[0].Title != "Chapter I" {
		t.Fatalf("chapters = %+v", res.Chapters)
	}
}

func TestRestoreImageInitial(t *testing.T) {
	t.Parallel()
	html := `<p class="nind"><span class="letra"><img alt="M" src="x.png"></span>R. BENNET was among the earliest.</p>`
	got, warn := restoreInitialFromHTML(html, "R. BENNET was among the earliest.")
	if warn != "" {
		t.Fatal(warn)
	}
	if got != "MR. BENNET was among the earliest." {
		t.Fatalf("got %q", got)
	}
}

func TestRestoreImageInitialDoesNotTouchTextDropcap(t *testing.T) {
	t.Parallel()
	html := `<p class="pfirst"><span class="dropcap">I</span>t was the Dover road.</p>`
	text := "It was the Dover road."
	got, warn := restoreInitialFromHTML(html, text)
	if warn != "" || got != text {
		t.Fatalf("text dropcap changed: %q %q", got, warn)
	}
}

func TestPageMarkerTitle(t *testing.T) {
	t.Parallel()
	res := &Result{Chapters: []Chapter{
		{Title: "{ix}", Paragraphs: []Paragraph{{Text: "preface paragraph that is long enough."}}},
		{Title: "Chapter I", Paragraphs: []Paragraph{{Text: "it is a truth universally acknowledged today."}}},
	}}
	cleanChapterTitles(nil, res)
	if res.Chapters[0].Title != "" {
		t.Fatalf("page marker title not cleared: %q", res.Chapters[0].Title)
	}
	if res.Chapters[1].Title != "Chapter I" {
		t.Fatalf("real title changed: %q", res.Chapters[1].Title)
	}
}

func TestPostprocessFrontMatterAndEmptyAndCaption(t *testing.T) {
	t.Parallel()
	raw := `<html><body>
<p>PREFACE. This preface should not vanish into thin air.</p>
<nav class="toc">
<a href="#c1">Chapter I</a>
<a href="#c1empty">Chapter I (toc)</a>
</nav>
<a id="c1empty"></a>
<h2 id="c1"><span class="caption">A picture caption.</span> Chapter I</h2>
<p>It is a truth universally acknowledged, that a single man in possession of a good fortune must be in want of a wife.</p>
<p>However little known the feelings or views of such a man may be on his first entering a neighbourhood.</p>
<p>This truth is so well fixed in the minds of the surrounding families.</p>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := AnchorTOC{}.Extract(doc)
	if err != nil {
		t.Fatal(err)
	}
	got := Postprocess(doc, extracted)
	if len(got.Chapters) < 2 {
		t.Fatalf("want front-matter + chapter, got %d chapters: %+v", len(got.Chapters), titles(got))
	}
	if len(got.Chapters[0].Paragraphs) == 0 || !strings.Contains(got.Chapters[0].Paragraphs[0].Text, "PREFACE") {
		t.Fatalf("front-matter missing: %+v", got.Chapters[0])
	}
	var empty int
	for _, ch := range got.Chapters {
		if len(ch.Paragraphs) == 0 {
			empty++
		}
	}
	if empty != 0 {
		t.Fatalf("empty chapters remain: %d", empty)
	}
	foundClean := false
	for _, ch := range got.Chapters {
		if ch.Title == "Chapter I" {
			foundClean = true
		}
		if strings.Contains(ch.Title, "caption") || strings.Contains(ch.Title, "picture") {
			t.Fatalf("caption leaked into title: %q", ch.Title)
		}
	}
	if !foundClean {
		t.Fatalf("cleaned chapter title missing: %v", titles(got))
	}
}

func TestCleanTitlesKeepsTOCText(t *testing.T) {
	t.Parallel()
	raw := `<html><body>
<nav class="toc">
<a href="#s1">Scene I. Elsinore. A platform before the Castle</a>
<a href="#s2">Scene II. A room of state</a>
</nav>
<h2>ACT I</h2>
<a id="s1"></a>
<p>Enter Francisco and Barnardo, two sentinels, who keep the watch upon the platform.</p>
<p>Another speech of decent length so the scene is not empty after extraction.</p>
<p>A third paragraph of similar length to keep chapter sanity signals healthy.</p>
<h2>SCENE II</h2>
<a id="s2"></a>
<p>Enter Claudius King of Denmark, Gertrude the Queen, Hamlet, and the courtiers.</p>
<p>Another reasonably long paragraph in the second scene of the play.</p>
<p>A third paragraph so this chapter also has enough text to look like a scene.</p>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := AnchorTOC{}.Extract(doc)
	if err != nil {
		t.Fatal(err)
	}
	got := Postprocess(doc, extracted)
	var titles []string
	for _, ch := range got.Chapters {
		if len(ch.Paragraphs) > 0 {
			titles = append(titles, ch.Title)
		}
	}
	joined := strings.Join(titles, " | ")
	if strings.Contains(joined, "ACT I") && !strings.Contains(joined, "Scene I") {
		t.Fatalf("TOC scene title overwritten by ACT heading: %v", titles)
	}
	found := false
	for _, ttitle := range titles {
		if strings.Contains(ttitle, "Scene I") && strings.Contains(ttitle, "Elsinore") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want TOC scene title kept, got %v", titles)
	}
}

func TestCleanTitlesRewritesHeadingCaption(t *testing.T) {
	t.Parallel()
	raw := `<html><body>
<h2 id="ch2"><span class="caption">I hope Mr. Bingley will like it.</span> CHAPTER II.</h2>
<p>Mr. Bennet was among the earliest of those who waited on Mr. Bingley at Netherfield.</p>
<p>He had always intended to visit him, though to the last always assuring his wife that he should not go.</p>
<p>Another paragraph of decent length so the chapter stays after postprocess.</p>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := HeadingSplit{}.Extract(doc)
	if err != nil {
		t.Fatal(err)
	}
	got := Postprocess(doc, extracted)
	if len(got.Chapters) == 0 {
		t.Fatal("no chapters")
	}
	title := got.Chapters[0].Title
	if strings.Contains(title, "Bingley") || strings.Contains(title, "caption") {
		t.Fatalf("caption remained in heading title: %q", title)
	}
	if !strings.Contains(strings.ToUpper(title), "CHAPTER II") {
		t.Fatalf("want CHAPTER II, got %q", title)
	}
}

func TestEvaluateAppliesPostprocessToBestOnly(t *testing.T) {
	t.Parallel()
	raw := []byte(`<html><body>
<p>PREFACE. Keep this preface paragraph for the administrator to see.</p>
<section class="chapter" id="c1">
<h2>Chapter I</h2>
<p>It is a truth universally acknowledged, that a single man in possession of a good fortune must be in want of a wife.</p>
<p>However little known the feelings or views of such a man may be on his first entering a neighbourhood.</p>
<p>This truth is so well fixed in the minds of the surrounding families.</p>
</section>
</body></html>`)
	ev, err := EvaluateHTML(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Best == nil {
		t.Fatal("no best")
	}
	foundFront := false
	for _, ch := range ev.Best.Result.Chapters {
		for _, p := range ch.Paragraphs {
			if strings.Contains(p.Text, "PREFACE") {
				foundFront = true
			}
		}
	}
	if !foundFront {
		t.Fatal("best result did not keep front-matter after postprocess")
	}
}

func titles(res *Result) []string {
	var out []string
	for _, ch := range res.Chapters {
		out = append(out, ch.Title)
	}
	return out
}

package parse

import (
	"strings"
	"testing"
)

const fixtureHTML = `<!DOCTYPE html>
<html><body>
<div id="pg-header" class="pg-boilerplate">
<p>*** START OF THE PROJECT GUTENBERG EBOOK TEST BOOK ***</p>
</div>
<nav class="toc">
<a href="#c1">Chapter I</a>
<a href="#c2">Chapter II</a>
</nav>
<section class="chapter" id="c1">
<h2>Chapter I</h2>
<p>It is a truth universally acknowledged, that a single man in possession of a good fortune must be in want of a wife.</p>
<p>However little known the feelings or views of such a man may be on his first entering a neighbourhood.</p>
<p>This truth is so well fixed in the minds of the surrounding families.</p>
</section>
<section class="chapter" id="c2">
<h2>Chapter II</h2>
<p>Mr. Bennet was among the earliest of those who waited on Mr. Bingley.</p>
<p>He had always intended to visit him, though to the last always assuring his wife that he should not go.</p>
<p>Another paragraph of decent length so the median stays healthy enough.</p>
</section>
<div id="pg-footer" class="pg-boilerplate">
<p>*** END OF THE PROJECT GUTENBERG EBOOK TEST BOOK ***</p>
</div>
</body></html>`

func TestEvaluateFixture(t *testing.T) {
	t.Parallel()
	ev, err := EvaluateHTML([]byte(fixtureHTML), nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Best == nil || ev.Best.Result == nil {
		t.Fatal("no best result")
	}
	if ev.Best.Name != "section-chapter" && ev.Best.Name != "heading-split" && ev.Best.Name != "anchor-toc" {
		t.Fatalf("unexpected winning strategy %s", ev.Best.Name)
	}
	if ev.ChapterCount() != 2 {
		t.Fatalf("chapters = %d, want 2", ev.ChapterCount())
	}
	if ev.ParagraphCount() != 6 {
		t.Fatalf("paragraphs = %d, want 6", ev.ParagraphCount())
	}
	if ev.Best.Signals.Coverage <= 0 {
		t.Fatal("coverage should be > 0")
	}

	names := map[string]bool{}
	for _, sc := range ev.All {
		names[sc.Name] = true
		if sc.Err != nil {
			t.Fatalf("%s: %v", sc.Name, sc.Err)
		}
	}
	for _, want := range []string{"section-chapter", "heading-split", "anchor-toc", "single-chapter"} {
		if !names[want] {
			t.Fatalf("missing strategy %s", want)
		}
	}
	if ev.Best.Result.Chapters[0].Paragraphs[0].StableID == "" {
		t.Fatal("stable_id empty")
	}
}

func TestExcludeTOCParagraphs(t *testing.T) {
	t.Parallel()
	html := `<html><body>
<div class="toc"><p>CONTENTS should not be a paragraph</p></div>
<h2>Chapter I</h2>
<p>A real paragraph that is long enough to keep around after normalization.</p>
</body></html>`
	ev, err := EvaluateHTML([]byte(html), []Extractor{HeadingSplit{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range ev.Best.Result.Chapters {
		for _, p := range ch.Paragraphs {
			if strings.Contains(p.Text, "CONTENTS") {
				t.Fatalf("toc paragraph leaked: %q", p.Text)
			}
		}
	}
}

func TestSingleChapterCap(t *testing.T) {
	t.Parallel()
	html := `<html><body>
<h1>A Modest Proposal</h1>
<p>One reasonably long paragraph of an essay without chapter structure at all.</p>
<p>Another reasonably long paragraph so coverage and para sanity both look healthy.</p>
<p>A third paragraph of similar length to satisfy chapter sanity if mis-applied.</p>
</body></html>`
	ev, err := EvaluateHTML([]byte(html), []Extractor{SingleChapter{}})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Best.Result.Confidence > SingleChapterCap {
		t.Fatalf("single-chapter cap not applied: %v", ev.Best.Result.Confidence)
	}
}

func TestTitleMatch(t *testing.T) {
	t.Parallel()
	if !TitleMatch("Pride and Prejudice", "The Project Gutenberg eBook of Pride and Prejudice, by Jane Austen") {
		t.Fatal("expected match")
	}
	if TitleMatch("Pride and Prejudice", "Moby Dick; Or, The Whale") {
		t.Fatal("should not match")
	}
}

package parse

import "github.com/PuerkitoBio/goquery"

type Confidence float64 // 0.0 ~ 1.0

type Paragraph struct {
	Idx      int
	Text     string // 정규화된 평문
	HTML     string // 원본 조각
	StableID string
}

type Chapter struct {
	Idx        int
	Title      string
	Anchor     string
	Paragraphs []Paragraph
}

type Result struct {
	Chapters   []Chapter
	Strategy   string
	Confidence Confidence
	Warnings   []string
}

type Extractor interface {
	Name() string
	Extract(doc *goquery.Document) (*Result, error)
}

func DefaultExtractors() []Extractor {
	return []Extractor{
		SectionChapter{},
		HeadingSplit{},
		AnchorTOC{},
		SingleChapter{},
	}
}

func reindex(res *Result) {
	if res == nil {
		return
	}
	for i := range res.Chapters {
		res.Chapters[i].Idx = i
		for j := range res.Chapters[i].Paragraphs {
			res.Chapters[i].Paragraphs[j].Idx = j
		}
	}
}

func emptyResult(name string, warnings ...string) *Result {
	return &Result{Strategy: name, Warnings: warnings}
}

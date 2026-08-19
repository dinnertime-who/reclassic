package parse

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// 문단 수집 시 제외할 영역. 리포트에서 조정 효과를 보기 위해 상수로 모은다.
const ExcludeSelector = "nav, header, footer, .toc, .contents, .pg-boilerplate, .tnote, figcaption, table, #pg-header, #pg-footer, #pg-machine-header, #pg-machine-footer"

func isExcluded(s *goquery.Selection) bool {
	return s.Closest(ExcludeSelector).Length() > 0
}

func paragraphFrom(s *goquery.Selection) (Paragraph, bool) {
	text := Normalize(s.Text())
	if text == "" {
		return Paragraph{}, false
	}
	htmlFrag, err := goquery.OuterHtml(s)
	if err != nil {
		htmlFrag = ""
	}
	return Paragraph{
		Text:     text,
		HTML:     htmlFrag,
		StableID: StableID(text),
	}, true
}

func collectParagraphs(scope *goquery.Selection) []Paragraph {
	var out []Paragraph
	scope.Find("p").Each(func(_ int, s *goquery.Selection) {
		if isExcluded(s) {
			return
		}
		p, ok := paragraphFrom(s)
		if !ok {
			return
		}
		p.Idx = len(out)
		out = append(out, p)
	})
	return out
}

func headingText(s *goquery.Selection) string {
	return Normalize(s.Text())
}

func nodeName(s *goquery.Selection) string {
	return strings.ToLower(goquery.NodeName(s))
}

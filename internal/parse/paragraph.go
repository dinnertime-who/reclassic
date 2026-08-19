package parse

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// 문단 수집 시 제외할 영역. 리포트에서 조정 효과를 보기 위해 상수로 모은다.
const ExcludeSelector = "nav, header, footer, .toc, .contents, .pg-boilerplate, .tnote, figcaption, table, #pg-header, #pg-footer, #pg-machine-header, #pg-machine-footer"

// 문단 **안에** 섞여 있어 본문 글자를 오염시키는 조각. 조상이 아니라 자손이라
// ExcludeSelector로는 걸러지지 않는다.
//
// 종이책 쪽번호 앵커가 대상이다:
//
//	<span class="pagenum"><a id="page_xxiv">{xxiv}</a></span>
//
// 그대로 두면 문장 한가운데에 "whether,{xi} this combination"처럼 남고,
// 그 글자가 stable_id 해시에 들어간다 (ADR-025).
//
// 근거가 있는 것만 넣는다. 코퍼스 22권에서 이 표기를 쓰는 책은 1342 하나이고
// .pageno 같은 변형은 나타나지 않았다. 없는 규칙보다 틀린 규칙이 나쁘다.
const InlineNoiseSelector = ".pagenum"

func isExcluded(s *goquery.Selection) bool {
	return s.Closest(ExcludeSelector).Length() > 0
}

func paragraphFrom(s *goquery.Selection) (Paragraph, bool) {
	text := cleanedParagraphText(s)
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

// cleanedParagraphText는 인라인 잡음을 뺀 본문이다.
// HTML 조각은 손대지 않는다 — 이미지 이니셜 복원(ADR-013 §4)이 원본을 봐야 한다.
func cleanedParagraphText(s *goquery.Selection) string {
	if s.Find(InlineNoiseSelector).Length() == 0 {
		return Normalize(s.Text())
	}
	clone := s.Clone()
	clone.Find(InlineNoiseSelector).Remove()
	return Normalize(clone.Text())
}

func headingText(s *goquery.Selection) string {
	return Normalize(s.Text())
}

func nodeName(s *goquery.Selection) string {
	return strings.ToLower(goquery.NodeName(s))
}

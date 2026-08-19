package parse

import "github.com/PuerkitoBio/goquery"

type HeadingSplit struct{}

func (HeadingSplit) Name() string { return "heading-split" }

func (HeadingSplit) Extract(doc *goquery.Document) (*Result, error) {
	res := &Result{Strategy: "heading-split"}
	cur := -1

	doc.Find("h1, h2, h3, p").Each(func(_ int, s *goquery.Selection) {
		if isExcluded(s) {
			return
		}
		name := nodeName(s)
		switch name {
		case "h1", "h2", "h3":
			title := headingText(s)
			if title == "" {
				return
			}
			anchor, _ := s.Attr("id")
			if anchor == "" {
				if id, ok := s.Find("[id]").First().Attr("id"); ok {
					anchor = id
				}
			}
			res.Chapters = append(res.Chapters, Chapter{Title: title, Anchor: anchor})
			cur = len(res.Chapters) - 1
		case "p":
			p, ok := paragraphFrom(s)
			if !ok {
				return
			}
			if cur < 0 {
				res.Chapters = append(res.Chapters, Chapter{Title: ""})
				cur = 0
			}
			p.Idx = len(res.Chapters[cur].Paragraphs)
			res.Chapters[cur].Paragraphs = append(res.Chapters[cur].Paragraphs, p)
		}
	})

	// 제목만 있고 문단이 없는 챕터는 남긴다 — 목차/장 제목 오탐을 신호로 쓰기 위함.
	if len(res.Chapters) == 0 {
		res.Warnings = append(res.Warnings, "heading-split: 챕터 없음")
	}
	reindex(res)
	return res, nil
}

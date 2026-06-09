package service

import (
	"sort"
	"strings"
	"unicode"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

type WordCloudTerm struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
}

func BuildWordCloud(messages []model.Message, limit int) []WordCloudTerm {
	if limit <= 0 {
		limit = 30
	}
	counts := map[string]int{}
	for _, message := range messages {
		senderTerms := senderStopTerms(message)
		for _, term := range tokenizeTerms(stripSenderPrefix(message.Content, message)) {
			if senderTerms[term] {
				continue
			}
			counts[term]++
		}
	}
	terms := make([]WordCloudTerm, 0, len(counts))
	for term, count := range counts {
		if count <= 0 {
			continue
		}
		terms = append(terms, WordCloudTerm{Term: term, Count: count})
	}
	sort.Slice(terms, func(i int, j int) bool {
		if terms[i].Count == terms[j].Count {
			return terms[i].Term < terms[j].Term
		}
		return terms[i].Count > terms[j].Count
	})
	if len(terms) > limit {
		terms = terms[:limit]
	}
	return terms
}

func stripSenderPrefix(content string, message model.Message) string {
	content = strings.TrimSpace(content)
	for _, sender := range []string{message.DisplaySender(), message.OriginalSender, message.Sender} {
		sender = strings.TrimSpace(sender)
		if sender == "" {
			continue
		}
		for _, sep := range []string{":", "："} {
			prefix := sender + sep
			if strings.HasPrefix(content, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(content, prefix))
			}
		}
	}
	return content
}

func senderStopTerms(message model.Message) map[string]bool {
	terms := map[string]bool{}
	for _, sender := range []string{message.DisplaySender(), message.OriginalSender, message.Sender} {
		for _, term := range tokenizeTerms(sender) {
			terms[term] = true
		}
	}
	return terms
}

func tokenizeTerms(content string) []string {
	terms := make([]string, 0)
	var current []rune
	var currentKind string
	flush := func() {
		if len(current) == 0 {
			return
		}
		term := strings.ToLower(string(current))
		kind := currentKind
		current = current[:0]
		runes := []rune(term)
		if len(runes) < 2 {
			return
		}
		if kind == "han" {
			for i := 0; i+1 < len(runes); i++ {
				gram := string(runes[i : i+2])
				if !isStopTerm(gram) {
					terms = append(terms, gram)
				}
			}
			return
		}
		if !isStopTerm(term) {
			terms = append(terms, term)
		}
	}
	for _, r := range content {
		kind := ""
		switch {
		case unicode.Is(unicode.Han, r):
			kind = "han"
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			kind = "word"
			r = unicode.ToLower(r)
		}
		if kind == "" {
			flush()
			currentKind = ""
			continue
		}
		if currentKind != "" && currentKind != kind {
			flush()
		}
		currentKind = kind
		current = append(current, r)
	}
	flush()
	return terms
}

func isStopTerm(term string) bool {
	switch term {
	case "我们", "你们", "他们", "这个", "那个", "就是", "然后", "但是", "还是", "可以", "没有", "the", "and", "for", "that", "this", "with", "you":
		return true
	default:
		return false
	}
}

package service

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/kevinmatthe/record_analysis/internal/model"
	"github.com/kevinmatthe/record_analysis/internal/textclean"
	"github.com/yanyiwu/gojieba"
)

type WordCloudTerm struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
}

var (
	jiebaOnce sync.Once
	jieba     *gojieba.Jieba
)

func BuildWordCloud(messages []model.Message, limit int) []WordCloudTerm {
	if limit <= 0 {
		limit = 30
	}
	counts := map[string]int{}
	docFreq := map[string]int{}
	docCount := 0
	for _, message := range messages {
		senderTerms := senderStopTerms(message)
		content := textclean.CleanMessageText(stripSenderPrefix(message.Content, message))
		if !textclean.IsNaturalMessageText(content) {
			continue
		}
		seenInMessage := map[string]bool{}
		hasTerm := false
		for _, term := range tokenizeTerms(content) {
			if senderTerms[term] {
				continue
			}
			counts[term]++
			hasTerm = true
			if !seenInMessage[term] {
				docFreq[term]++
				seenInMessage[term] = true
			}
		}
		if hasTerm {
			docCount++
		}
	}
	scores := wordCloudScores(counts, docFreq, docCount)
	type scoredTerm struct {
		WordCloudTerm
		Score float64
	}
	terms := make([]scoredTerm, 0, len(counts))
	for term, count := range counts {
		if count <= 0 {
			continue
		}
		terms = append(terms, scoredTerm{WordCloudTerm: WordCloudTerm{Term: term, Count: count}, Score: scores[term]})
	}
	sort.Slice(terms, func(i int, j int) bool {
		if terms[i].Score == terms[j].Score {
			if terms[i].Count == terms[j].Count {
				return terms[i].Term < terms[j].Term
			}
			return terms[i].Count > terms[j].Count
		}
		return terms[i].Score > terms[j].Score
	})
	if len(terms) > limit {
		terms = terms[:limit]
	}
	result := make([]WordCloudTerm, 0, len(terms))
	for _, term := range terms {
		result = append(result, term.WordCloudTerm)
	}
	return result
}

func wordCloudScores(counts map[string]int, docFreq map[string]int, docCount int) map[string]float64 {
	scores := map[string]float64{}
	if docCount == 0 {
		return scores
	}
	for term, count := range counts {
		idf := math.Log(float64(docCount+1)/float64(docFreq[term])) + 1
		scores[term] = math.Log1p(float64(count)) * idf
	}
	return scores
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
			terms = append(terms, jiebaTerms(term)...)
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

func jiebaTerms(content string) []string {
	jiebaOnce.Do(func() {
		jieba = gojieba.NewJieba()
	})
	if jieba == nil {
		return nil
	}
	words := jieba.Cut(content, true)
	terms := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.TrimSpace(strings.ToLower(word))
		if !isUsefulTerm(word) {
			continue
		}
		terms = append(terms, word)
	}
	return terms
}

func isUsefulTerm(term string) bool {
	runes := []rune(term)
	if len(runes) < 2 {
		return false
	}
	if isStopTerm(term) {
		return false
	}
	hasLetterOrHan := false
	for _, r := range runes {
		switch {
		case unicode.Is(unicode.Han, r), unicode.IsLetter(r):
			hasLetterOrHan = true
		case unicode.IsDigit(r):
			continue
		default:
			return false
		}
	}
	return hasLetterOrHan
}

func isStopTerm(term string) bool {
	switch term {
	case "我们", "你们", "他们", "这个", "那个", "就是", "然后", "但是", "还是", "可以", "没有", "一个", "一下", "什么", "怎么", "不是", "因为", "所以", "觉得", "the", "and", "for", "that", "this", "with", "you":
		return true
	default:
		return false
	}
}

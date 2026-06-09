package service

import (
	"testing"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

func TestBuildWordCloudCountsTerms(t *testing.T) {
	messages := []model.Message{
		{ID: "MSG_000001", MsgTime: time.Date(2026, 6, 1, 10, 0, 0, 0, time.Local), Content: "今天吃饭，吃饭了吗"},
		{ID: "MSG_000002", MsgTime: time.Date(2026, 6, 1, 10, 1, 0, 0, time.Local), Content: "movie plan movie"},
		{ID: "MSG_000003", MsgTime: time.Date(2026, 6, 1, 10, 2, 0, 0, time.Local), Content: "今天 movie"},
	}

	terms := BuildWordCloud(messages, 3)
	if len(terms) != 3 {
		t.Fatalf("terms = %+v", terms)
	}
	if terms[0].Term != "movie" || terms[0].Count != 3 {
		t.Fatalf("top term = %+v", terms[0])
	}
	if terms[1].Term != "今天" || terms[1].Count != 2 {
		t.Fatalf("second term = %+v", terms[1])
	}
	if terms[2].Term != "吃饭" || terms[2].Count != 2 {
		t.Fatalf("third term = %+v", terms[2])
	}
}

func TestBuildWordCloudIgnoresSenderPrefixAndSenderName(t *testing.T) {
	messages := []model.Message{
		{ID: "MSG_000001", Sender: "PERSON_B", OriginalSender: "小青", Content: "小青: 今天吃饭"},
		{ID: "MSG_000002", Sender: "PERSON_B", OriginalSender: "小青", Content: "小青 今天看电影"},
	}

	terms := BuildWordCloud(messages, 10)
	for _, term := range terms {
		if term.Term == "小青" {
			t.Fatalf("sender name leaked into word cloud: %+v", terms)
		}
	}
}

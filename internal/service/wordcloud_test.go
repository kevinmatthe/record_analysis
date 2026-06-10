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
	if termCount(terms, "movie") != 3 {
		t.Fatalf("expected movie count 3: %+v", terms)
	}
	if termCount(terms, "今天") != 2 {
		t.Fatalf("expected 今天 count 2: %+v", terms)
	}
	if termCount(terms, "吃饭") != 2 {
		t.Fatalf("expected 吃饭 count 2: %+v", terms)
	}
}

func TestBuildWordCloudRanksDistinctSegmentTermsWithTFIDF(t *testing.T) {
	messages := []model.Message{
		{ID: "MSG_000001", Content: "今天 吃饭 movie"},
		{ID: "MSG_000002", Content: "今天 吃饭 movie"},
		{ID: "MSG_000003", Content: "今天 吃饭 旅行签证 旅行签证"},
	}

	terms := BuildWordCloud(messages, 5)
	if len(terms) == 0 {
		t.Fatal("expected terms")
	}
	if termCount(terms, "旅行") == 0 && termCount(terms, "签证") == 0 && termCount(terms, "旅行签证") == 0 {
		t.Fatalf("expected distinctive TF-IDF term first, got %+v", terms)
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

func TestBuildWordCloudIgnoresWeChatStructuralNoise(t *testing.T) {
	messages := []model.Message{
		{ID: "MSG_000001", Content: `<msg><appmsg appid="wx"><nickname>null</nickname><type>4</type><msgid>1</msgid><![CDATA[noise]]></appmsg></msg>`},
		{ID: "MSG_000002", Content: "今晚吃火锅"},
	}

	terms := BuildWordCloud(messages, 10)
	for _, term := range terms {
		switch term.Term {
		case "nickname", "appid", "msgid", "cdata", "null", "type":
			t.Fatalf("wechat structure leaked into word cloud: %+v", terms)
		}
	}
	if termCount(terms, "火锅") == 0 && termCount(terms, "吃火锅") == 0 {
		t.Fatalf("expected natural term: %+v", terms)
	}
}

func termCount(terms []WordCloudTerm, term string) int {
	for _, item := range terms {
		if item.Term == term {
			return item.Count
		}
	}
	return 0
}

package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

func TestOpenAICompatibleExtractorExtractActionsUsesStructuredOutputs(t *testing.T) {
	var sawJSONSchema bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		responseFormat, ok := req["response_format"].(map[string]interface{})
		if !ok || responseFormat["type"] != "json_schema" {
			t.Fatalf("missing json_schema response_format: %#v", req["response_format"])
		}
		if req["max_tokens"] != float64(4096) {
			t.Fatalf("max_tokens = %v", req["max_tokens"])
		}
		thinking, ok := req["thinking"].(map[string]interface{})
		if !ok || thinking["type"] != "disabled" {
			t.Fatalf("thinking = %#v", req["thinking"])
		}
		sawJSONSchema = true
		writeChatCompletion(w, `{
			"msg_id":"MSG_000001",
			"sender":"PERSON_A",
			"action_type":"express_dissatisfaction",
			"emotion":"不安",
			"intent":"希望得到回应",
			"target":"PERSON_B",
			"evidence_text":"你今天怎么又不回我",
			"evidence_msg_ids":["MSG_000001"],
			"confidence":0.82,
			"uncertainty":"只基于文字"
		}`)
	}))
	defer server.Close()

	extractor := NewOpenAICompatibleExtractor(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	messages := []model.Message{taskMsg(1, "PERSON_A", 0, "你今天怎么又不回我")}
	segments := []model.Segment{{ID: "SEG_000001", RelationshipID: "rel_test", MessageIDs: []string{"MSG_000001"}}}

	actions, err := extractor.ExtractActions(context.Background(), messages, segments)
	if err != nil {
		t.Fatal(err)
	}
	if !sawJSONSchema {
		t.Fatal("server did not observe json schema request")
	}
	if len(actions) != 1 || actions[0].ActionType != "express_dissatisfaction" {
		t.Fatalf("actions = %+v", actions)
	}
	if actions[0].SegmentID != "SEG_000001" {
		t.Fatalf("segment id = %s", actions[0].SegmentID)
	}
}

func TestOpenAICompatibleExtractorExtractActionsBySegmentUsesBatchSchema(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		format := req["response_format"].(map[string]interface{})
		jsonSchema := format["json_schema"].(map[string]interface{})
		if jsonSchema["name"] != "action_batch_extraction" {
			t.Fatalf("schema name = %v", jsonSchema["name"])
		}
		writeChatCompletion(w, `{
			"actions":[{
				"msg_id":"MSG_000001",
				"sender":"PERSON_A",
				"action_type":"express_dissatisfaction",
				"emotion":"不安",
				"intent":"希望得到回应",
				"target":"PERSON_B",
				"evidence_text":"你今天怎么又不回我",
				"evidence_msg_ids":["MSG_000001"],
				"confidence":0.82,
				"uncertainty":"只基于文字"
			}]
		}`)
	}))
	defer server.Close()

	extractor := NewOpenAICompatibleExtractor(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	messages := []model.Message{taskMsg(1, "PERSON_A", 0, "你今天怎么又不回我")}
	segments := []model.Segment{{ID: "SEG_000001", RelationshipID: "rel_test", MessageIDs: []string{"MSG_000001"}}}

	actions, err := extractor.ExtractActionsBySegment(context.Background(), messages, segments)
	if err != nil {
		t.Fatal(err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	if len(actions) != 1 || actions[0].SegmentID != "SEG_000001" {
		t.Fatalf("actions = %+v", actions)
	}
}

func TestOpenAICompatibleExtractorGeneratesReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeChatCompletion(w, `{
			"report_markdown":"# 报告\n\nevidence_msg_ids: [MSG_000001]",
			"evidence_event_ids":["EVT_000001"],
			"evidence_msg_ids":["MSG_000001"],
			"confidence":0.8,
			"uncertainty":"样本少"
		}`)
	}))
	defer server.Close()

	extractor := NewOpenAICompatibleExtractor(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)
	report, err := extractor.GenerateReport(context.Background(), ReportInput{
		RelationshipID: "rel_test",
		Metrics: model.BehaviorMetrics{
			RelationshipID: "rel_test",
			PeriodStart:    start,
			PeriodEnd:      start.Add(time.Hour),
			Values:         map[string]interface{}{"message_volume": 1},
		},
		Events: []model.RelationshipEvent{{ID: "EVT_000001", EvidenceMsgIDs: []string{"MSG_000001"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.Markdown, "evidence_msg_ids") {
		t.Fatalf("report markdown = %s", report.Markdown)
	}
	if report.ModelName != "test-model" {
		t.Fatalf("model = %s", report.ModelName)
	}
	if len(report.EvidenceEventIDs) != 1 || report.EvidenceEventIDs[0] != "EVT_000001" {
		t.Fatalf("evidence events = %+v", report.EvidenceEventIDs)
	}
}

func TestOpenAICompatibleExtractorReportsNonStopFinishReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message":       map[string]interface{}{"role": "assistant", "content": `{"actions":[`},
					"finish_reason": "length",
				},
			},
		})
	}))
	defer server.Close()

	extractor := NewOpenAICompatibleExtractor(OpenAICompatibleConfig{
		BaseURL: server.URL,
		Model:   "test-model",
	})
	messages := []model.Message{taskMsg(1, "PERSON_A", 0, "你今天怎么又不回我")}
	segments := []model.Segment{{ID: "SEG_000001", RelationshipID: "rel_test", MessageIDs: []string{"MSG_000001"}}}

	_, err := extractor.ExtractActionsBySegment(context.Background(), messages, segments)
	if err == nil || !strings.Contains(err.Error(), "finished with length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAICompatibleExtractorUsesVersionedBasePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		writeChatCompletion(w, `{
			"actions":[{
				"msg_id":"MSG_000001",
				"sender":"PERSON_A",
				"action_type":"express_dissatisfaction",
				"emotion":"不安",
				"intent":"希望得到回应",
				"target":"PERSON_B",
				"evidence_text":"你今天怎么又不回我",
				"evidence_msg_ids":["MSG_000001"],
				"confidence":0.82,
				"uncertainty":"只基于文字"
			}]
		}`)
	}))
	defer server.Close()

	extractor := NewOpenAICompatibleExtractor(OpenAICompatibleConfig{BaseURL: server.URL + "/api/v3", Model: "m"})
	messages := []model.Message{taskMsg(1, "PERSON_A", 0, "你今天怎么又不回我")}
	segments := []model.Segment{{ID: "SEG_000001", RelationshipID: "rel_test", MessageIDs: []string{"MSG_000001"}}}

	if _, err := extractor.ExtractActionsBySegment(context.Background(), messages, segments); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAICompatibleExtractorReportsTokenUsage(t *testing.T) {
	var usageEvent UsageEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "chatcmpl_test",
			"object": "chat.completion",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": `{"actions":[]}`,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     120,
				"completion_tokens": 30,
				"total_tokens":      150,
			},
		})
	}))
	defer server.Close()

	extractor := NewOpenAICompatibleExtractor(OpenAICompatibleConfig{
		BaseURL: server.URL,
		Model:   "test-model",
	})
	ctx := WithUsageReporter(context.Background(), func(event UsageEvent) {
		usageEvent = event
	})
	messages := []model.Message{taskMsg(1, "PERSON_A", 0, "你今天怎么又不回我")}
	segments := []model.Segment{{ID: "SEG_000001", RelationshipID: "rel_test", MessageIDs: []string{"MSG_000001"}}}

	if _, err := extractor.ExtractActionsBySegment(ctx, messages, segments); err != nil {
		t.Fatal(err)
	}
	if usageEvent.SchemaName != "action_batch_extraction" || usageEvent.TotalTokens != 150 {
		t.Fatalf("usage event = %+v", usageEvent)
	}
}

func TestOpenAICompatibleExtractorRetriesEnglishTopicSummary(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			writeChatCompletion(w, `{
				"title":"Dinner plan",
				"summary":"They discussed where to eat and who would arrive first.",
				"topics":["dinner","arrival time"],
				"key_events":["PERSON_A asked about dinner."],
				"evidence_msg_ids":["MSG_000001"],
				"confidence":0.8,
				"uncertainty":"Short sample."
			}`)
			return
		}
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		messages := req["messages"].([]interface{})
		system := messages[0].(map[string]interface{})["content"].(string)
		if !strings.Contains(system, "上一次输出没有满足语言要求") {
			t.Fatalf("retry prompt missing language suffix: %s", system)
		}
		writeChatCompletion(w, `{
			"title":"晚饭安排",
			"summary":"双方在讨论去哪里吃饭以及到达顺序。",
			"topics":["晚饭地点","到达时间"],
			"key_events":["PERSON_A 询问晚饭安排。"],
			"evidence_msg_ids":["MSG_000001"],
			"confidence":0.8,
			"uncertainty":"样本较短。"
		}`)
	}))
	defer server.Close()

	extractor := NewOpenAICompatibleExtractor(OpenAICompatibleConfig{
		BaseURL: server.URL,
		Model:   "test-model",
	})
	summary, err := extractor.SummarizeTopic(context.Background(), TopicSummaryInput{
		RelationshipID: "rel_test",
		ScopeID:        "bucket_1",
		Granularity:    "hour",
		StartTime:      "2026-06-09 12:00:00",
		EndTime:        "2026-06-09 13:00:00",
		Messages:       []model.Message{taskMsg(1, "PERSON_A", 0, "晚上吃什么？")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
	if summary.Title != "晚饭安排" || !strings.Contains(summary.Summary, "讨论") {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestTopicSummaryNeedsChineseRetryAllowsProductNames(t *testing.T) {
	summary := TopicSummary{
		Title:       "OpenAI SDK 接入",
		Summary:     "讨论 OpenAI SDK 的配置方式和请求参数。",
		Topics:      []string{"OpenAI SDK", "BaseURL 配置"},
		KeyEvents:   []string{"确认使用兼容 OpenAI 的 SDK。"},
		Uncertainty: "需要继续联调接口返回。",
	}
	if topicSummaryNeedsChineseRetry(summary) {
		t.Fatal("expected Chinese summary with product names to pass")
	}
}

func writeChatCompletion(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     "chatcmpl_test",
		"object": "chat.completion",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
	})
}

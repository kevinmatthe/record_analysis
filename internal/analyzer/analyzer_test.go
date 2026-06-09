package analyzer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/llm"
	"github.com/kevinmatthe/record_analysis/internal/model"
)

func msg(id int, sender string, minute int, content string) model.Message {
	return model.Message{
		ID:             model.StableMessageID(id),
		RelationshipID: "rel_test",
		Sender:         sender,
		MsgTime:        time.Date(2026, 6, 1, 20, minute, 0, 0, time.Local),
		MsgType:        "文本",
		Content:        content,
		RawContent:     content,
		Source:         "test",
	}
}

func TestAnalyzeMessagesWithoutExtractorOnlyImportsSegmentsAndMetrics(t *testing.T) {
	messages := []model.Message{
		msg(1, "PERSON_A", 0, "你今天怎么又不回我"),
		msg(2, "PERSON_B", 2, "我在忙，不知道怎么说"),
		msg(3, "PERSON_A", 4, "我只是希望你能回应一下我的感受"),
	}

	result, err := AnalyzeMessages(messages, "rel_test")
	if err != nil {
		t.Fatal(err)
	}
	if result.Metrics.Values["message_volume"] != 3 {
		t.Fatalf("message_volume = %v, want 3", result.Metrics.Values["message_volume"])
	}
	if len(result.Segments) == 0 {
		t.Fatal("expected segments")
	}
	if len(result.Actions) != 0 || len(result.Events) != 0 {
		t.Fatalf("default path must not synthesize actions/events without LLM, got %d actions %d events", len(result.Actions), len(result.Events))
	}
	if !strings.Contains(result.Report.Markdown, "未配置结构化 LLM extractor") {
		t.Fatal("report should clearly state extractor is not configured")
	}
	if strings.Contains(result.Report.Markdown, "冲突事件数") || strings.Contains(result.Report.Markdown, "修复尝试数") {
		t.Fatal("disabled report should not present rule-derived relationship conclusions")
	}
}

func TestAnalyzeMessagesWithExtractorUsesStructuredLLMOutputs(t *testing.T) {
	messages := []model.Message{
		msg(1, "PERSON_A", 0, "你今天怎么又不回我"),
		msg(2, "PERSON_B", 2, "对不起，我刚才在忙"),
	}
	extractor := fakeExtractor{}

	result, err := AnalyzeMessagesWithOptions(context.Background(), messages, "rel_test", AnalyzeOptions{Extractor: extractor})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Actions) != 2 {
		t.Fatalf("actions = %d, want 2", len(result.Actions))
	}
	if len(result.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(result.Events))
	}
	if !strings.Contains(result.Report.Markdown, "evidence_msg_ids") {
		t.Fatal("LLM report should carry evidence ids")
	}
	if !strings.Contains(result.Report.Markdown, "你今天怎么又不回我") {
		t.Fatal("LLM report should include evidence text")
	}
}

func TestAnalyzeMessagesPrefersSegmentBatchActionExtraction(t *testing.T) {
	messages := []model.Message{
		msg(1, "PERSON_A", 0, "你今天怎么又不回我"),
		msg(2, "PERSON_B", 2, "对不起，我刚才在忙"),
	}
	extractor := &batchFakeExtractor{fakeExtractor: fakeExtractor{}}

	result, err := AnalyzeMessagesWithOptions(context.Background(), messages, "rel_test", AnalyzeOptions{Extractor: extractor})
	if err != nil {
		t.Fatal(err)
	}
	if !extractor.batchCalled {
		t.Fatal("expected analyzer to use segment batch action extraction")
	}
	if extractor.singleCalled {
		t.Fatal("analyzer should not call single-message action extraction when batch is available")
	}
	if len(result.Actions) != 2 {
		t.Fatalf("actions = %d, want 2", len(result.Actions))
	}
}

func TestAnalyzeMessagesWithExtractorCapsLLMMessages(t *testing.T) {
	messages := []model.Message{
		msg(1, "PERSON_A", 0, "第一句"),
		msg(2, "PERSON_B", 1, "第二句"),
		msg(3, "PERSON_A", 2, "第三句"),
	}

	result, err := AnalyzeMessagesWithOptions(context.Background(), messages, "rel_test", AnalyzeOptions{
		Extractor:      fakeExtractor{},
		MaxLLMMessages: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 3 {
		t.Fatalf("imported messages = %d, want 3", len(result.Messages))
	}
	if len(result.Actions) != 2 {
		t.Fatalf("LLM actions = %d, want 2", len(result.Actions))
	}
	if result.Actions[0].MsgID != model.StableMessageID(2) || result.Actions[1].MsgID != model.StableMessageID(3) {
		t.Fatalf("expected latest messages to be sent to LLM, got actions on %s and %s", result.Actions[0].MsgID, result.Actions[1].MsgID)
	}
	if got := result.Metrics.Values["message_volume"]; got != 3 {
		t.Fatalf("message_volume = %v, want 3", got)
	}
}

func TestAnalyzeMessagesQuickModeUsesSingleReportCall(t *testing.T) {
	messages := []model.Message{
		msg(1, "PERSON_A", 0, "第一句"),
		msg(2, "PERSON_B", 1, "第二句"),
	}
	extractor := &countingExtractor{fakeExtractor: fakeExtractor{}}

	result, err := AnalyzeMessagesWithOptions(context.Background(), messages, "rel_test", AnalyzeOptions{
		Extractor:      extractor,
		MaxLLMMessages: 1,
		Mode:           "quick",
	})
	if err != nil {
		t.Fatal(err)
	}
	if extractor.reportCalls != 1 {
		t.Fatalf("report calls = %d, want 1", extractor.reportCalls)
	}
	if extractor.actionCalls != 0 || extractor.eventCalls != 0 || extractor.dimensionCalls != 0 {
		t.Fatalf("unexpected extractor calls: actions=%d events=%d dimensions=%d", extractor.actionCalls, extractor.eventCalls, extractor.dimensionCalls)
	}
	if len(result.Actions) != 0 || len(result.Events) != 0 {
		t.Fatalf("quick mode should skip actions/events, got %d/%d", len(result.Actions), len(result.Events))
	}
	if result.Report.ModelName != "fake_extractor" {
		t.Fatalf("model name = %s", result.Report.ModelName)
	}
}

type fakeExtractor struct{}

func (fakeExtractor) ExtractActions(_ context.Context, messages []model.Message, segments []model.Segment) ([]model.MessageAction, error) {
	segmentID := ""
	if len(segments) > 0 {
		segmentID = segments[0].ID
	}
	return []model.MessageAction{
		{
			ID:             "ACT_000001",
			RelationshipID: "rel_test",
			MsgID:          messages[0].ID,
			SegmentID:      segmentID,
			Sender:         messages[0].Sender,
			ActionType:     "express_dissatisfaction",
			EvidenceText:   messages[0].Content,
			Confidence:     0.82,
		},
		{
			ID:             "ACT_000002",
			RelationshipID: "rel_test",
			MsgID:          messages[1].ID,
			SegmentID:      segmentID,
			Sender:         messages[1].Sender,
			ActionType:     "repair_attempt",
			EvidenceText:   messages[1].Content,
			Confidence:     0.78,
		},
	}, nil
}

type batchFakeExtractor struct {
	fakeExtractor
	batchCalled  bool
	singleCalled bool
}

func (b *batchFakeExtractor) ExtractActions(ctx context.Context, messages []model.Message, segments []model.Segment) ([]model.MessageAction, error) {
	b.singleCalled = true
	return b.fakeExtractor.ExtractActions(ctx, messages, segments)
}

type countingExtractor struct {
	fakeExtractor
	actionCalls    int
	eventCalls     int
	dimensionCalls int
	reportCalls    int
}

func (c *countingExtractor) ExtractActions(ctx context.Context, messages []model.Message, segments []model.Segment) ([]model.MessageAction, error) {
	c.actionCalls++
	return c.fakeExtractor.ExtractActions(ctx, messages, segments)
}

func (c *countingExtractor) ExtractEvents(ctx context.Context, messages []model.Message, segments []model.Segment, actions []model.MessageAction) ([]model.RelationshipEvent, error) {
	c.eventCalls++
	return c.fakeExtractor.ExtractEvents(ctx, messages, segments, actions)
}

func (c *countingExtractor) GenerateDimensions(ctx context.Context, metrics model.BehaviorMetrics, events []model.RelationshipEvent) (model.PsychologicalDimensions, error) {
	c.dimensionCalls++
	return c.fakeExtractor.GenerateDimensions(ctx, metrics, events)
}

func (c *countingExtractor) GenerateReport(ctx context.Context, input llm.ReportInput) (model.PeriodReport, error) {
	c.reportCalls++
	return c.fakeExtractor.GenerateReport(ctx, input)
}

func (b *batchFakeExtractor) ExtractActionsBySegment(ctx context.Context, messages []model.Message, segments []model.Segment) ([]model.MessageAction, error) {
	b.batchCalled = true
	return b.fakeExtractor.ExtractActions(ctx, messages, segments)
}

type failingExtractor struct{}

func (failingExtractor) ExtractActions(context.Context, []model.Message, []model.Segment) ([]model.MessageAction, error) {
	return nil, errors.New("unexpected extractor call")
}
func (failingExtractor) ExtractEvents(context.Context, []model.Message, []model.Segment, []model.MessageAction) ([]model.RelationshipEvent, error) {
	return nil, errors.New("unexpected extractor call")
}
func (failingExtractor) GenerateDimensions(context.Context, model.BehaviorMetrics, []model.RelationshipEvent) (model.PsychologicalDimensions, error) {
	return model.PsychologicalDimensions{}, errors.New("unexpected extractor call")
}
func (failingExtractor) GenerateReport(context.Context, llm.ReportInput) (model.PeriodReport, error) {
	return model.PeriodReport{}, errors.New("unexpected extractor call")
}

func (fakeExtractor) ExtractEvents(_ context.Context, _ []model.Message, segments []model.Segment, actions []model.MessageAction) ([]model.RelationshipEvent, error) {
	return []model.RelationshipEvent{
		{
			ID:                "EVT_000001",
			RelationshipID:    "rel_test",
			SegmentID:         segments[0].ID,
			EventType:         "conflict",
			Topic:             "回复延迟",
			Result:            "出现不满表达和修复尝试",
			RepairStatus:      "partial",
			EvidenceMsgIDs:    []string{actions[0].MsgID, actions[1].MsgID},
			EvidenceActionIDs: []string{actions[0].ID, actions[1].ID},
			Confidence:        0.8,
		},
	}, nil
}

func (fakeExtractor) GenerateDimensions(_ context.Context, metrics model.BehaviorMetrics, events []model.RelationshipEvent) (model.PsychologicalDimensions, error) {
	return model.PsychologicalDimensions{
		RelationshipID: metrics.RelationshipID,
		PeriodStart:    metrics.PeriodStart,
		PeriodEnd:      metrics.PeriodEnd,
		Values: map[string]interface{}{
			"repair_capacity": map[string]interface{}{
				"trend":              "stable",
				"evidence_event_ids": []string{events[0].ID},
				"confidence":         0.75,
			},
		},
	}, nil
}

func (fakeExtractor) GenerateReport(_ context.Context, input llm.ReportInput) (model.PeriodReport, error) {
	return model.PeriodReport{
		RelationshipID: input.RelationshipID,
		PeriodType:     "weekly",
		PeriodStart:    input.Metrics.PeriodStart,
		PeriodEnd:      input.Metrics.PeriodEnd,
		Markdown:       "claim: 出现回复延迟相关互动\nevidence_msg_ids: [MSG_000001 MSG_000002]\n- PERSON_A：你今天怎么又不回我\n",
		ModelName:      "fake_extractor",
	}, nil
}

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
	openai "github.com/sashabaranov/go-openai"
)

type OpenAICompatibleConfig struct {
	BaseURL       string
	APIKey        string
	Model         string
	ContextWindow int
	AssetRoot     string
	HTTPClient    *http.Client
	Logf          func(format string, args ...interface{})
	MaxTokens     int
}

type OpenAICompatibleExtractor struct {
	model         string
	contextWindow int
	assetRoot     string
	client        *openai.Client
	logf          func(format string, args ...interface{})
	maxTokens     int
}

func NewOpenAICompatibleExtractor(config OpenAICompatibleConfig) *OpenAICompatibleExtractor {
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	contextWindow := config.ContextWindow
	if contextWindow <= 0 {
		contextWindow = 2
	}
	maxTokens := config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = baseURL
	clientConfig.HTTPClient = withThinkingTransport(client, shouldDisableThinking(baseURL))
	return &OpenAICompatibleExtractor{
		model:         config.Model,
		contextWindow: contextWindow,
		assetRoot:     config.AssetRoot,
		client:        openai.NewClientWithConfig(clientConfig),
		logf:          config.Logf,
		maxTokens:     maxTokens,
	}
}

func (e *OpenAICompatibleExtractor) ExtractActions(ctx context.Context, messages []model.Message, segments []model.Segment) ([]model.MessageAction, error) {
	tasks := BuildActionTasks(messages, messagesRelationshipID(messages), e.contextWindow)
	segmentByMsg := map[string]string{}
	for _, segment := range segments {
		for _, msgID := range segment.MessageIDs {
			segmentByMsg[msgID] = segment.ID
		}
	}
	actions := make([]model.MessageAction, 0, len(tasks))
	for index, task := range tasks {
		var action model.MessageAction
		if err := e.completeJSON(ctx, "llm/prompts/action_extraction.md", task, "action_extraction", "llm/schemas/action.schema.json", &action); err != nil {
			return nil, err
		}
		if action.ID == "" {
			action.ID = fmt.Sprintf("ACT_%06d", index+1)
		}
		if action.RelationshipID == "" {
			action.RelationshipID = task.RelationshipID
		}
		if action.MsgID == "" {
			action.MsgID = task.MsgID
		}
		if action.SegmentID == "" {
			action.SegmentID = segmentByMsg[action.MsgID]
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func (e *OpenAICompatibleExtractor) ExtractActionsBySegment(ctx context.Context, messages []model.Message, segments []model.Segment) ([]model.MessageAction, error) {
	tasks := BuildActionBatchTasks(messages, segments)
	actions := make([]model.MessageAction, 0, len(messages))
	for _, task := range tasks {
		var response struct {
			Actions []model.MessageAction `json:"actions"`
		}
		if err := e.completeJSON(ctx, "llm/prompts/action_batch_extraction.md", task, "action_batch_extraction", "llm/schemas/action_batch.schema.json", &response); err != nil {
			return nil, err
		}
		for _, action := range response.Actions {
			if action.ID == "" {
				action.ID = fmt.Sprintf("ACT_%06d", len(actions)+1)
			}
			if action.RelationshipID == "" {
				action.RelationshipID = task.RelationshipID
			}
			if action.SegmentID == "" {
				action.SegmentID = task.Segment.ID
			}
			actions = append(actions, action)
		}
	}
	return actions, nil
}

func (e *OpenAICompatibleExtractor) ExtractEvents(ctx context.Context, messages []model.Message, segments []model.Segment, actions []model.MessageAction) ([]model.RelationshipEvent, error) {
	events := make([]model.RelationshipEvent, 0, len(segments))
	for _, segment := range segments {
		task := BuildEventTask(segment, messages, actions)
		var event model.RelationshipEvent
		if err := e.completeJSON(ctx, "llm/prompts/event_extraction.md", task, "event_extraction", "llm/schemas/event.schema.json", &event); err != nil {
			return nil, err
		}
		if event.ID == "" {
			event.ID = fmt.Sprintf("EVT_%06d", len(events)+1)
		}
		if event.RelationshipID == "" {
			event.RelationshipID = segment.RelationshipID
		}
		if event.SegmentID == "" {
			event.SegmentID = segment.ID
		}
		events = append(events, event)
	}
	return events, nil
}

func (e *OpenAICompatibleExtractor) GenerateDimensions(ctx context.Context, metrics model.BehaviorMetrics, events []model.RelationshipEvent) (model.PsychologicalDimensions, error) {
	task := map[string]interface{}{
		"behavior_metrics": metrics,
		"events":           events,
	}
	var response struct {
		Dimensions map[string]interface{} `json:"dimensions"`
	}
	if err := e.completeJSON(ctx, "llm/prompts/dimension_generation.md", task, "dimension_generation", "llm/schemas/dimensions.schema.json", &response); err != nil {
		return model.PsychologicalDimensions{}, err
	}
	return model.PsychologicalDimensions{
		RelationshipID: metrics.RelationshipID,
		PeriodStart:    metrics.PeriodStart,
		PeriodEnd:      metrics.PeriodEnd,
		Values:         response.Dimensions,
	}, nil
}

func (e *OpenAICompatibleExtractor) GenerateReport(ctx context.Context, input ReportInput) (model.PeriodReport, error) {
	task := BuildReportTask(input.RelationshipID, input.Messages, input.Metrics, input.Dimensions, input.Events, nil)
	var response struct {
		ReportMarkdown   string   `json:"report_markdown"`
		EvidenceEventIDs []string `json:"evidence_event_ids"`
		EvidenceMsgIDs   []string `json:"evidence_msg_ids"`
		Confidence       float64  `json:"confidence"`
		Uncertainty      string   `json:"uncertainty"`
	}
	if err := e.completeJSON(ctx, "llm/prompts/report_generation.md", task, "report_generation", "llm/schemas/report.schema.json", &response); err != nil {
		return model.PeriodReport{}, err
	}
	return model.PeriodReport{
		RelationshipID:   input.RelationshipID,
		PeriodType:       "weekly",
		PeriodStart:      input.Metrics.PeriodStart,
		PeriodEnd:        input.Metrics.PeriodEnd,
		Markdown:         response.ReportMarkdown,
		EvidenceEventIDs: response.EvidenceEventIDs,
		ModelName:        e.model,
	}, nil
}

func (e *OpenAICompatibleExtractor) MergeReports(ctx context.Context, input ReportMergeInput) (model.PeriodReport, error) {
	task := BuildReportMergeTask(input)
	var response struct {
		ReportMarkdown   string   `json:"report_markdown"`
		EvidenceEventIDs []string `json:"evidence_event_ids"`
		EvidenceMsgIDs   []string `json:"evidence_msg_ids"`
		Confidence       float64  `json:"confidence"`
		Uncertainty      string   `json:"uncertainty"`
	}
	if err := e.completeJSON(ctx, "llm/prompts/report_merge.md", task, "report_merge", "llm/schemas/report.schema.json", &response); err != nil {
		return model.PeriodReport{}, err
	}
	return model.PeriodReport{
		RelationshipID:   input.RelationshipID,
		PeriodType:       "branch_merge",
		PeriodStart:      parseReportMergeTime(input.PeriodStart),
		PeriodEnd:        parseReportMergeTime(input.PeriodEnd),
		Markdown:         response.ReportMarkdown,
		EvidenceEventIDs: response.EvidenceEventIDs,
		ModelName:        e.model,
	}, nil
}

func (e *OpenAICompatibleExtractor) SummarizeTopic(ctx context.Context, input TopicSummaryInput) (TopicSummary, error) {
	task := BuildTopicSummaryTask(input)
	var response TopicSummary
	if err := e.completeJSON(ctx, "llm/prompts/topic_summary.md", task, "topic_summary", "llm/schemas/topic_summary.schema.json", &response); err != nil {
		return TopicSummary{}, err
	}
	if topicSummaryNeedsChineseRetry(response) {
		response = TopicSummary{}
		if err := e.completeJSONWithPromptSuffix(ctx, "llm/prompts/topic_summary.md", chineseOnlyPromptSuffix(), task, "topic_summary", "llm/schemas/topic_summary.schema.json", &response); err != nil {
			return TopicSummary{}, err
		}
	}
	response.ModelName = e.model
	return response, nil
}

func parseReportMergeTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	return time.Time{}
}

func (e *OpenAICompatibleExtractor) MergeTopicSummaries(ctx context.Context, input TopicSummaryMergeInput) (TopicSummary, error) {
	task := BuildTopicSummaryMergeTask(input)
	var response TopicSummary
	if err := e.completeJSON(ctx, "llm/prompts/topic_summary_merge.md", task, "topic_summary_merge", "llm/schemas/topic_summary.schema.json", &response); err != nil {
		return TopicSummary{}, err
	}
	if topicSummaryNeedsChineseRetry(response) {
		response = TopicSummary{}
		if err := e.completeJSONWithPromptSuffix(ctx, "llm/prompts/topic_summary_merge.md", chineseOnlyPromptSuffix(), task, "topic_summary_merge", "llm/schemas/topic_summary.schema.json", &response); err != nil {
			return TopicSummary{}, err
		}
	}
	response.ModelName = e.model
	return response, nil
}

func (e *OpenAICompatibleExtractor) completeJSON(ctx context.Context, promptPath string, input interface{}, schemaName string, schemaPath string, output interface{}) error {
	return e.completeJSONWithPromptSuffix(ctx, promptPath, "", input, schemaName, schemaPath, output)
}

func (e *OpenAICompatibleExtractor) completeJSONWithPromptSuffix(ctx context.Context, promptPath string, promptSuffix string, input interface{}, schemaName string, schemaPath string, output interface{}) error {
	if e.model == "" {
		return errors.New("llm model is required")
	}
	start := time.Now()
	e.log("llm request start schema=%s model=%s", schemaName, e.model)
	systemPrompt, err := e.readAsset(promptPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(promptSuffix) != "" {
		systemPrompt = strings.TrimSpace(systemPrompt) + "\n\n" + strings.TrimSpace(promptSuffix)
	}
	schemaText, err := e.readAsset(schemaPath)
	if err != nil {
		return err
	}
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaText), &schema); err != nil {
		return err
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return err
	}
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	resp, err := e.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: e.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: string(inputJSON)},
		},
		MaxTokens: e.maxTokens,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:   schemaName,
				Schema: json.RawMessage(schemaJSON),
				Strict: true,
			},
		},
	})
	if err != nil {
		e.log("llm request failed schema=%s duration=%s error=%v", schemaName, time.Since(start).Round(time.Millisecond), err)
		return err
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return errors.New("llm response contained no content")
	}
	reportUsage(ctx, UsageEvent{
		SchemaName:       schemaName,
		Model:            e.model,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	})
	if resp.Choices[0].FinishReason != "" && resp.Choices[0].FinishReason != openai.FinishReasonStop {
		return fmt.Errorf("llm response finished with %s before valid JSON was confirmed", resp.Choices[0].FinishReason)
	}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), output); err != nil {
		e.log("llm response parse failed schema=%s duration=%s error=%v", schemaName, time.Since(start).Round(time.Millisecond), err)
		return err
	}
	e.log("llm request done schema=%s duration=%s", schemaName, time.Since(start).Round(time.Millisecond))
	return nil
}

func chineseOnlyPromptSuffix() string {
	return "重要：上一次输出没有满足语言要求。请重新生成，所有可读字段必须使用简体中文；除专有名词、产品名、URL、代码标识外，不要输出英文句子。"
}

func topicSummaryNeedsChineseRetry(summary TopicSummary) bool {
	text := strings.Join(topicSummaryReadableFields(summary), "\n")
	cjk, latin := countCJKAndLatin(text)
	if latin <= 12 {
		return false
	}
	if cjk < 4 {
		return true
	}
	return latin > cjk*2
}

func topicSummaryReadableFields(summary TopicSummary) []string {
	fields := []string{summary.Title, summary.Summary, summary.Uncertainty}
	fields = append(fields, summary.Topics...)
	fields = append(fields, summary.KeyEvents...)
	return fields
}

func countCJKAndLatin(text string) (int, int) {
	var cjk int
	var latin int
	for _, r := range text {
		switch {
		case r >= '\u4e00' && r <= '\u9fff':
			cjk++
		case r >= 'A' && r <= 'Z':
			latin++
		case r >= 'a' && r <= 'z':
			latin++
		}
	}
	return cjk, latin
}

func (e *OpenAICompatibleExtractor) log(format string, args ...interface{}) {
	if e.logf != nil {
		e.logf(format, args...)
	}
}

func (e *OpenAICompatibleExtractor) readAsset(path string) (string, error) {
	candidates := []string{path, filepath.Join("../..", path)}
	if e.assetRoot != "" {
		candidates = append([]string{filepath.Join(e.assetRoot, path)}, candidates...)
	}
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("asset not found: %s", path)
}

func messagesRelationshipID(messages []model.Message) string {
	if len(messages) == 0 {
		return ""
	}
	return messages[0].RelationshipID
}

func shouldDisableThinking(baseURL string) bool {
	baseURL = strings.ToLower(baseURL)
	return !strings.Contains(baseURL, "api.openai.com")
}

func withThinkingTransport(client *http.Client, disableThinking bool) *http.Client {
	clone := *client
	if !disableThinking {
		return &clone
	}
	base := clone.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone.Transport = thinkingTransport{base: base}
	return &clone
}

type thinkingTransport struct {
	base http.RoundTripper
}

func (t thinkingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/chat/completions") || req.Body == nil {
		return t.base.RoundTrip(req)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		_ = req.Body.Close()
		return nil, err
	}
	_ = req.Body.Close()
	payload["thinking"] = map[string]string{"type": "disabled"}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	cloned := req.Clone(req.Context())
	cloned.Body = io.NopCloser(bytes.NewReader(data))
	cloned.ContentLength = int64(len(data))
	cloned.Header.Set("Content-Length", strconv.Itoa(len(data)))
	return t.base.RoundTrip(cloned)
}

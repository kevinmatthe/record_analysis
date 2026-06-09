package llm

import (
	"context"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

type Extractor interface {
	ExtractActions(ctx context.Context, messages []model.Message, segments []model.Segment) ([]model.MessageAction, error)
	ExtractEvents(ctx context.Context, messages []model.Message, segments []model.Segment, actions []model.MessageAction) ([]model.RelationshipEvent, error)
	GenerateDimensions(ctx context.Context, metrics model.BehaviorMetrics, events []model.RelationshipEvent) (model.PsychologicalDimensions, error)
	GenerateReport(ctx context.Context, input ReportInput) (model.PeriodReport, error)
}

type BatchActionExtractor interface {
	ExtractActionsBySegment(ctx context.Context, messages []model.Message, segments []model.Segment) ([]model.MessageAction, error)
}

type TopicSummarizer interface {
	SummarizeTopic(ctx context.Context, input TopicSummaryInput) (TopicSummary, error)
}

type TopicSummaryMerger interface {
	MergeTopicSummaries(ctx context.Context, input TopicSummaryMergeInput) (TopicSummary, error)
}

type TopicSummaryInput struct {
	RelationshipID string
	ScopeID        string
	Granularity    string
	StartTime      string
	EndTime        string
	Messages       []model.Message
}

type TopicSummary struct {
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	Topics         []string `json:"topics"`
	KeyEvents      []string `json:"key_events"`
	EvidenceMsgIDs []string `json:"evidence_msg_ids"`
	Confidence     float64  `json:"confidence"`
	Uncertainty    string   `json:"uncertainty"`
	ModelName      string   `json:"model_name,omitempty"`
}

type TopicSummaryMergeInput struct {
	RelationshipID string
	ScopeID        string
	Granularity    string
	StartTime      string
	EndTime        string
	Summaries      []TopicSummary
}

type ReportInput struct {
	RelationshipID string
	Messages       []model.Message
	Segments       []model.Segment
	Actions        []model.MessageAction
	Events         []model.RelationshipEvent
	Metrics        model.BehaviorMetrics
	Dimensions     model.PsychologicalDimensions
}

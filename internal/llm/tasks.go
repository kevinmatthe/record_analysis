package llm

import (
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
	"github.com/kevinmatthe/record_analysis/internal/textclean"
)

type MessageForLLM struct {
	ID      string `json:"id"`
	Sender  string `json:"sender"`
	Time    string `json:"time"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type ActionExtractionTask struct {
	RelationshipID string          `json:"relationship_id"`
	MsgID          string          `json:"msg_id"`
	ContextBefore  []MessageForLLM `json:"context_before"`
	Message        MessageForLLM   `json:"message"`
	ContextAfter   []MessageForLLM `json:"context_after"`
}

type ActionBatchExtractionTask struct {
	RelationshipID string          `json:"relationship_id"`
	Segment        model.Segment   `json:"segment"`
	Messages       []MessageForLLM `json:"messages"`
}

type EventExtractionTask struct {
	RelationshipID string                `json:"relationship_id"`
	Segment        model.Segment         `json:"segment"`
	Messages       []MessageForLLM       `json:"messages"`
	Actions        []model.MessageAction `json:"actions"`
}

type ReportGenerationTask struct {
	RelationshipID  string                        `json:"relationship_id"`
	PeriodStart     string                        `json:"period_start"`
	PeriodEnd       string                        `json:"period_end"`
	Messages        []MessageForLLM               `json:"messages"`
	Metrics         model.BehaviorMetrics         `json:"behavior_metrics"`
	Dimensions      model.PsychologicalDimensions `json:"psychological_dimensions"`
	Events          []model.RelationshipEvent     `json:"events"`
	CounterEvidence []string                      `json:"counter_evidence"`
}

type ReportMergeTask struct {
	RelationshipID string               `json:"relationship_id"`
	PeriodStart    string               `json:"period_start"`
	PeriodEnd      string               `json:"period_end"`
	Reports        []model.PeriodReport `json:"reports"`
}

type TopicSummaryTask struct {
	RelationshipID string          `json:"relationship_id"`
	ScopeID        string          `json:"scope_id"`
	Granularity    string          `json:"granularity"`
	StartTime      string          `json:"start_time"`
	EndTime        string          `json:"end_time"`
	Messages       []MessageForLLM `json:"messages"`
}

type TopicSummaryMergeTask struct {
	RelationshipID string         `json:"relationship_id"`
	ScopeID        string         `json:"scope_id"`
	Granularity    string         `json:"granularity"`
	StartTime      string         `json:"start_time"`
	EndTime        string         `json:"end_time"`
	Summaries      []TopicSummary `json:"summaries"`
}

func BuildActionTasks(messages []model.Message, relationshipID string, contextWindow int) []ActionExtractionTask {
	if contextWindow < 0 {
		contextWindow = 0
	}
	tasks := make([]ActionExtractionTask, 0, len(messages))
	for index, message := range messages {
		beforeStart := index - contextWindow
		if beforeStart < 0 {
			beforeStart = 0
		}
		afterEnd := index + contextWindow + 1
		if afterEnd > len(messages) {
			afterEnd = len(messages)
		}
		tasks = append(tasks, ActionExtractionTask{
			RelationshipID: relationshipID,
			MsgID:          message.ID,
			ContextBefore:  llmMessages(messages[beforeStart:index]),
			Message:        llmMessage(message),
			ContextAfter:   llmMessages(messages[index+1 : afterEnd]),
		})
	}
	return tasks
}

func BuildActionBatchTasks(messages []model.Message, segments []model.Segment) []ActionBatchExtractionTask {
	messageByID := map[string]model.Message{}
	for _, message := range messages {
		messageByID[message.ID] = message
	}
	tasks := make([]ActionBatchExtractionTask, 0, len(segments))
	for _, segment := range segments {
		segmentMessages := make([]model.Message, 0, len(segment.MessageIDs))
		for _, msgID := range segment.MessageIDs {
			if message, ok := messageByID[msgID]; ok {
				segmentMessages = append(segmentMessages, message)
			}
		}
		tasks = append(tasks, ActionBatchExtractionTask{
			RelationshipID: segment.RelationshipID,
			Segment:        segment,
			Messages:       llmMessages(segmentMessages),
		})
	}
	return tasks
}

func BuildEventTask(segment model.Segment, messages []model.Message, actions []model.MessageAction) EventExtractionTask {
	messageByID := map[string]model.Message{}
	for _, message := range messages {
		messageByID[message.ID] = message
	}
	segmentMessages := make([]model.Message, 0, len(segment.MessageIDs))
	for _, msgID := range segment.MessageIDs {
		if message, ok := messageByID[msgID]; ok {
			segmentMessages = append(segmentMessages, message)
		}
	}
	segmentActions := make([]model.MessageAction, 0, len(actions))
	for _, action := range actions {
		if action.SegmentID == segment.ID {
			segmentActions = append(segmentActions, action)
		}
	}
	return EventExtractionTask{
		RelationshipID: segment.RelationshipID,
		Segment:        segment,
		Messages:       llmMessages(segmentMessages),
		Actions:        segmentActions,
	}
}

func BuildReportTask(
	relationshipID string,
	messages []model.Message,
	metrics model.BehaviorMetrics,
	dimensions model.PsychologicalDimensions,
	events []model.RelationshipEvent,
	counterEvidence []string,
) ReportGenerationTask {
	return ReportGenerationTask{
		RelationshipID:  relationshipID,
		PeriodStart:     formatTime(metrics.PeriodStart),
		PeriodEnd:       formatTime(metrics.PeriodEnd),
		Messages:        llmMessages(messages),
		Metrics:         metrics,
		Dimensions:      dimensions,
		Events:          events,
		CounterEvidence: counterEvidence,
	}
}

func BuildReportMergeTask(input ReportMergeInput) ReportMergeTask {
	return ReportMergeTask{
		RelationshipID: input.RelationshipID,
		PeriodStart:    input.PeriodStart,
		PeriodEnd:      input.PeriodEnd,
		Reports:        input.Reports,
	}
}

func BuildTopicSummaryTask(input TopicSummaryInput) TopicSummaryTask {
	return TopicSummaryTask{
		RelationshipID: input.RelationshipID,
		ScopeID:        input.ScopeID,
		Granularity:    input.Granularity,
		StartTime:      input.StartTime,
		EndTime:        input.EndTime,
		Messages:       llmMessages(input.Messages),
	}
}

func BuildTopicSummaryMergeTask(input TopicSummaryMergeInput) TopicSummaryMergeTask {
	return TopicSummaryMergeTask{
		RelationshipID: input.RelationshipID,
		ScopeID:        input.ScopeID,
		Granularity:    input.Granularity,
		StartTime:      input.StartTime,
		EndTime:        input.EndTime,
		Summaries:      input.Summaries,
	}
}

func llmMessages(messages []model.Message) []MessageForLLM {
	result := make([]MessageForLLM, 0, len(messages))
	for _, message := range messages {
		if !textclean.IsNaturalMessageText(message.Content) {
			continue
		}
		result = append(result, llmMessage(message))
	}
	return result
}

func llmMessage(message model.Message) MessageForLLM {
	return MessageForLLM{
		ID:      message.ID,
		Sender:  message.Sender,
		Time:    formatTime(message.MsgTime),
		Type:    message.MsgType,
		Content: textclean.CleanMessageText(message.Content),
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

package model

import (
	"fmt"
	"time"
)

type Message struct {
	ID             string    `json:"id"`
	RelationshipID string    `json:"relationship_id"`
	Sender         string    `json:"sender"`
	OriginalSender string    `json:"original_sender,omitempty"`
	Receiver       string    `json:"receiver,omitempty"`
	MsgTime        time.Time `json:"msg_time"`
	MsgType        string    `json:"msg_type"`
	Content        string    `json:"content"`
	RawContent     string    `json:"raw_content"`
	Source         string    `json:"source"`
	ContentHash    string    `json:"content_hash"`
}

type Segment struct {
	ID             string    `json:"id"`
	RelationshipID string    `json:"relationship_id"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	MessageIDs     []string  `json:"message_ids"`
	SegmentType    string    `json:"segment_type"`
	Summary        string    `json:"summary,omitempty"`
}

type MessageAction struct {
	ID             string  `json:"id"`
	RelationshipID string  `json:"relationship_id"`
	MsgID          string  `json:"msg_id"`
	SegmentID      string  `json:"segment_id,omitempty"`
	Sender         string  `json:"sender"`
	ActionType     string  `json:"action_type"`
	Emotion        string  `json:"emotion"`
	Intent         string  `json:"intent"`
	Target         string  `json:"target,omitempty"`
	EvidenceText   string  `json:"evidence_text"`
	Confidence     float64 `json:"confidence"`
}

type RelationshipEvent struct {
	ID                string   `json:"id"`
	RelationshipID    string   `json:"relationship_id"`
	SegmentID         string   `json:"segment_id"`
	EventType         string   `json:"event_type"`
	Topic             string   `json:"topic"`
	Trigger           string   `json:"trigger"`
	Process           []string `json:"process"`
	Result            string   `json:"result"`
	RepairStatus      string   `json:"repair_status"`
	RepairInitiator   string   `json:"repair_initiator,omitempty"`
	EvidenceMsgIDs    []string `json:"evidence_msg_ids"`
	EvidenceActionIDs []string `json:"evidence_action_ids"`
	Confidence        float64  `json:"confidence"`
}

type BehaviorMetrics struct {
	RelationshipID string                 `json:"relationship_id"`
	PeriodStart    time.Time              `json:"period_start"`
	PeriodEnd      time.Time              `json:"period_end"`
	Values         map[string]interface{} `json:"metrics"`
}

type PsychologicalDimensions struct {
	RelationshipID string                 `json:"relationship_id"`
	PeriodStart    time.Time              `json:"period_start"`
	PeriodEnd      time.Time              `json:"period_end"`
	Values         map[string]interface{} `json:"dimensions"`
}

type PeriodReport struct {
	RelationshipID   string    `json:"relationship_id"`
	PeriodType       string    `json:"period_type"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
	Markdown         string    `json:"report_markdown"`
	EvidenceEventIDs []string  `json:"evidence_event_ids"`
	ModelName        string    `json:"model_name"`
}

type AnalysisResult struct {
	RelationshipID string                  `json:"relationship_id"`
	Messages       []Message               `json:"messages"`
	Segments       []Segment               `json:"segments"`
	Actions        []MessageAction         `json:"actions"`
	Events         []RelationshipEvent     `json:"events"`
	Metrics        BehaviorMetrics         `json:"behavior_metrics"`
	Dimensions     PsychologicalDimensions `json:"psychological_dimensions"`
	Report         PeriodReport            `json:"period_report"`
}

func StableMessageID(index int) string {
	return fmt.Sprintf("MSG_%06d", index)
}

func (m Message) DisplaySender() string {
	if m.OriginalSender != "" {
		return m.OriginalSender
	}
	if m.Sender == "PERSON_A" {
		return "我"
	}
	return m.Sender
}

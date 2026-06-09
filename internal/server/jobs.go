package server

import (
	"encoding/json"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
	"github.com/kevinmatthe/record_analysis/internal/service"
)

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type AnalysisJob struct {
	ID               string                  `json:"id"`
	Status           JobStatus               `json:"status"`
	Stage            string                  `json:"stage"`
	RelationshipID   string                  `json:"relationship_id"`
	FileName         string                  `json:"file_name"`
	MessageCount     int                     `json:"message_count"`
	LLMMessageLimit  int                     `json:"llm_message_limit"`
	LLMMessageCount  int                     `json:"llm_message_count"`
	AnalysisMode     string                  `json:"analysis_mode"`
	ProcessedCount   int                     `json:"processed_count"`
	Progress         float64                 `json:"progress"`
	PreviewTotal     int                     `json:"preview_total"`
	PromptTokens     int                     `json:"prompt_tokens"`
	CompletionTokens int                     `json:"completion_tokens"`
	TotalTokens      int                     `json:"total_tokens"`
	ResultRecordID   string                  `json:"result_record_id,omitempty"`
	Error            string                  `json:"error,omitempty"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
	Events           []JobEvent              `json:"events"`
	Result           *service.AnalysisRecord `json:"result,omitempty"`
	messages         []model.Message
}

type JobEvent struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

type AnalysisBranch struct {
	ID               string    `json:"id"`
	JobID            string    `json:"job_id"`
	RelationshipID   string    `json:"relationship_id"`
	Title            string    `json:"title"`
	Granularity      string    `json:"granularity"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	MessageCount     int       `json:"message_count"`
	BucketIDs        []string  `json:"bucket_ids"`
	ClusterID        string    `json:"cluster_id"`
	TopicHint        string    `json:"topic_hint"`
	Status           string    `json:"status"`
	Stage            string    `json:"stage"`
	Progress         float64   `json:"progress"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	ReportMarkdown   string    `json:"report_markdown,omitempty"`
	ModelName        string    `json:"model_name,omitempty"`
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type AnalysisWorkItem struct {
	ID               string          `json:"id"`
	JobID            string          `json:"job_id"`
	Kind             string          `json:"kind"`
	ScopeType        string          `json:"scope_type"`
	ScopeID          string          `json:"scope_id"`
	Granularity      string          `json:"granularity"`
	StartTime        time.Time       `json:"start_time"`
	EndTime          time.Time       `json:"end_time"`
	Status           string          `json:"status"`
	Priority         int             `json:"priority"`
	Progress         float64         `json:"progress"`
	MessageCount     int             `json:"message_count"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	TotalTokens      int             `json:"total_tokens"`
	Result           json.RawMessage `json:"result,omitempty"`
	Error            string          `json:"error,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	ClaimedAt        *time.Time      `json:"claimed_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
}

type MessagePreview struct {
	ID      string `json:"id"`
	Sender  string `json:"sender"`
	Time    string `json:"time"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type PreviewPage struct {
	Items      []MessagePreview `json:"items"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}

type MessageSearchPage struct {
	Items      []MessagePreview `json:"items"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
	Source     string           `json:"source"`
}

func cloneJob(job *AnalysisJob) AnalysisJob {
	copy := *job
	copy.Events = append([]JobEvent(nil), job.Events...)
	copy.messages = nil
	if job.Result != nil {
		record := *job.Result
		copy.Result = &record
	}
	return copy
}

func messageSearchPage(messages []model.Message, page int, pageSize int, source string) MessageSearchPage {
	preview := previewPage(messages, page, pageSize)
	return MessageSearchPage{
		Items:      preview.Items,
		Total:      preview.Total,
		Page:       preview.Page,
		PageSize:   preview.PageSize,
		TotalPages: preview.TotalPages,
		Source:     source,
	}
}

func previewPage(messages []model.Message, page int, pageSize int) PreviewPage {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	total := len(messages)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := make([]MessagePreview, 0, end-start)
	for _, message := range messages[start:end] {
		content := message.Content
		if len([]rune(content)) > 160 {
			runes := []rune(content)
			content = string(runes[:160]) + "..."
		}
		items = append(items, MessagePreview{
			ID:      message.ID,
			Sender:  message.DisplaySender(),
			Time:    message.MsgTime.Format("2006-01-02 15:04:05"),
			Type:    message.MsgType,
			Content: content,
		})
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	return PreviewPage{Items: items, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}
}

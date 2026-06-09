package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

type Indexer interface {
	IndexMessages(ctx context.Context, docs []MessageDocument) error
	IndexSummary(ctx context.Context, doc SummaryDocument) error
	IndexBranch(ctx context.Context, doc BranchDocument) error
	SearchMessages(ctx context.Context, request MessageSearchRequest) (MessageSearchResult, error)
	Status(ctx context.Context) Status
}

type NoopIndexer struct{}

func (NoopIndexer) IndexMessages(context.Context, []MessageDocument) error { return nil }
func (NoopIndexer) IndexSummary(context.Context, SummaryDocument) error    { return nil }
func (NoopIndexer) IndexBranch(context.Context, BranchDocument) error      { return nil }
func (NoopIndexer) SearchMessages(context.Context, MessageSearchRequest) (MessageSearchResult, error) {
	return MessageSearchResult{}, fmt.Errorf("opensearch disabled")
}
func (NoopIndexer) Status(context.Context) Status {
	return Status{Enabled: false, Degraded: true, Reason: "opensearch config missing or disabled"}
}

type Status struct {
	Enabled  bool   `json:"enabled"`
	Healthy  bool   `json:"healthy"`
	Degraded bool   `json:"degraded"`
	Reason   string `json:"reason,omitempty"`
	URL      string `json:"url,omitempty"`
}

type Config struct {
	Domain             string
	User               string
	Password           string
	RecordMessageIndex string
	RecordSummaryIndex string
	RecordBranchIndex  string
	Scheme             string
}

type OpenSearchIndexer struct {
	baseURL            string
	user               string
	password           string
	recordMessageIndex string
	recordSummaryIndex string
	recordBranchIndex  string
	client             *http.Client
}

type MessageDocument struct {
	JobID          string    `json:"job_id"`
	RelationshipID string    `json:"relationship_id"`
	MessageID      string    `json:"message_id"`
	Sender         string    `json:"sender"`
	DisplaySender  string    `json:"display_sender"`
	MessageTime    time.Time `json:"message_time"`
	MessageType    string    `json:"message_type"`
	Content        string    `json:"content"`
	Source         string    `json:"source,omitempty"`
	ContentHash    string    `json:"content_hash,omitempty"`
	IndexedAt      time.Time `json:"indexed_at"`
}

type MessageSearchRequest struct {
	JobID    string
	Query    string
	Start    *time.Time
	End      *time.Time
	Page     int
	PageSize int
}

type MessageSearchResult struct {
	Items      []MessageDocument `json:"items"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
	Source     string            `json:"source"`
}

type SummaryDocument struct {
	ID               string          `json:"id"`
	JobID            string          `json:"job_id"`
	RelationshipID   string          `json:"relationship_id"`
	Kind             string          `json:"kind"`
	ScopeType        string          `json:"scope_type"`
	ScopeID          string          `json:"scope_id"`
	Granularity      string          `json:"granularity"`
	StartTime        time.Time       `json:"start_time"`
	EndTime          time.Time       `json:"end_time"`
	Status           string          `json:"status"`
	Result           json.RawMessage `json:"result"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	TotalTokens      int             `json:"total_tokens"`
	IndexedAt        time.Time       `json:"indexed_at"`
}

type BranchDocument struct {
	ID               string    `json:"id"`
	JobID            string    `json:"job_id"`
	RelationshipID   string    `json:"relationship_id"`
	Title            string    `json:"title"`
	Granularity      string    `json:"granularity"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	MessageCount     int       `json:"message_count"`
	TopicHint        string    `json:"topic_hint"`
	Status           string    `json:"status"`
	Stage            string    `json:"stage"`
	ReportMarkdown   string    `json:"report_markdown,omitempty"`
	ModelName        string    `json:"model_name,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	IndexedAt        time.Time `json:"indexed_at"`
}

func NewOpenSearchIndexer(cfg Config) (*OpenSearchIndexer, error) {
	baseURL, err := normalizeBaseURL(cfg.Domain, cfg.Scheme)
	if err != nil {
		return nil, err
	}
	if cfg.RecordMessageIndex == "" {
		cfg.RecordMessageIndex = "record_analysis_messages"
	}
	if cfg.RecordSummaryIndex == "" {
		cfg.RecordSummaryIndex = "record_analysis_summaries"
	}
	if cfg.RecordBranchIndex == "" {
		cfg.RecordBranchIndex = "record_analysis_branches"
	}
	return &OpenSearchIndexer{
		baseURL:            baseURL,
		user:               cfg.User,
		password:           cfg.Password,
		recordMessageIndex: cfg.RecordMessageIndex,
		recordSummaryIndex: cfg.RecordSummaryIndex,
		recordBranchIndex:  cfg.RecordBranchIndex,
		client:             &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func MessageDocuments(jobID string, relationshipID string, messages []model.Message) []MessageDocument {
	now := time.Now()
	docs := make([]MessageDocument, 0, len(messages))
	for _, message := range messages {
		docs = append(docs, MessageDocument{
			JobID:          jobID,
			RelationshipID: relationshipID,
			MessageID:      message.ID,
			Sender:         message.Sender,
			DisplaySender:  message.DisplaySender(),
			MessageTime:    message.MsgTime,
			MessageType:    message.MsgType,
			Content:        message.Content,
			Source:         message.Source,
			ContentHash:    message.ContentHash,
			IndexedAt:      now,
		})
	}
	return docs
}

func (i *OpenSearchIndexer) IndexMessages(ctx context.Context, docs []MessageDocument) error {
	if len(docs) == 0 {
		return nil
	}
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, doc := range docs {
		id := doc.JobID + ":" + doc.MessageID
		if err := encoder.Encode(map[string]map[string]string{"index": {"_index": i.recordMessageIndex, "_id": id}}); err != nil {
			return err
		}
		if err := encoder.Encode(doc); err != nil {
			return err
		}
	}
	return i.do(ctx, http.MethodPost, "/_bulk", &body)
}

func (i *OpenSearchIndexer) IndexSummary(ctx context.Context, doc SummaryDocument) error {
	doc.IndexedAt = time.Now()
	if doc.ID == "" {
		doc.ID = doc.JobID + ":" + doc.Kind + ":" + doc.ScopeID
	}
	return i.putDocument(ctx, i.recordSummaryIndex, doc.ID, doc)
}

func (i *OpenSearchIndexer) IndexBranch(ctx context.Context, doc BranchDocument) error {
	doc.IndexedAt = time.Now()
	return i.putDocument(ctx, i.recordBranchIndex, doc.ID, doc)
}

func (i *OpenSearchIndexer) SearchMessages(ctx context.Context, request MessageSearchRequest) (MessageSearchResult, error) {
	page, pageSize := normalizePage(request.Page, request.PageSize)
	must := []map[string]interface{}{
		{"term": map[string]interface{}{"job_id": request.JobID}},
	}
	if strings.TrimSpace(request.Query) != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{"content": request.Query},
		})
	}
	filters := make([]map[string]interface{}, 0, 1)
	if request.Start != nil || request.End != nil {
		rangeQuery := map[string]interface{}{}
		if request.Start != nil {
			rangeQuery["gte"] = request.Start.Format(time.RFC3339)
		}
		if request.End != nil {
			rangeQuery["lt"] = request.End.Format(time.RFC3339)
		}
		filters = append(filters, map[string]interface{}{"range": map[string]interface{}{"message_time": rangeQuery}})
	}
	body := map[string]interface{}{
		"from": (page - 1) * pageSize,
		"size": pageSize,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must":   must,
				"filter": filters,
			},
		},
		"sort": []map[string]interface{}{{"message_time": map[string]string{"order": "asc"}}},
	}
	var raw struct {
		Hits struct {
			Total interface{} `json:"total"`
			Hits  []struct {
				Source MessageDocument `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := i.postJSON(ctx, "/"+url.PathEscape(i.recordMessageIndex)+"/_search", body, &raw); err != nil {
		return MessageSearchResult{}, err
	}
	items := make([]MessageDocument, 0, len(raw.Hits.Hits))
	for _, hit := range raw.Hits.Hits {
		items = append(items, hit.Source)
	}
	total := parseSearchTotal(raw.Hits.Total)
	return MessageSearchResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages(total, pageSize),
		Source:     "opensearch",
	}, nil
}

func (i *OpenSearchIndexer) Status(ctx context.Context) Status {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.baseURL, nil)
	if err != nil {
		return Status{Enabled: true, Healthy: false, Degraded: true, Reason: err.Error(), URL: i.baseURL}
	}
	if i.user != "" || i.password != "" {
		req.SetBasicAuth(i.user, i.password)
	}
	resp, err := i.client.Do(req)
	if err != nil {
		return Status{Enabled: true, Healthy: false, Degraded: true, Reason: err.Error(), URL: i.baseURL}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Status{Enabled: true, Healthy: false, Degraded: true, Reason: resp.Status, URL: i.baseURL}
	}
	return Status{Enabled: true, Healthy: true, URL: i.baseURL}
}

func (i *OpenSearchIndexer) putDocument(ctx context.Context, index string, id string, doc interface{}) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(doc); err != nil {
		return err
	}
	return i.do(ctx, http.MethodPut, "/"+url.PathEscape(index)+"/_doc/"+url.PathEscape(id), &body)
}

func (i *OpenSearchIndexer) postJSON(ctx context.Context, path string, payload interface{}, out interface{}) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, i.baseURL+path, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if i.user != "" || i.password != "" {
		req.SetBasicAuth(i.user, i.password)
	}
	resp, err := i.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch POST %s returned %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (i *OpenSearchIndexer) do(ctx context.Context, method string, path string, body *bytes.Buffer) error {
	req, err := http.NewRequestWithContext(ctx, method, i.baseURL+path, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if path == "/_bulk" {
		req.Header.Set("Content-Type", "application/x-ndjson")
	}
	if i.user != "" || i.password != "" {
		req.SetBasicAuth(i.user, i.password)
	}
	resp, err := i.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch %s %s returned %s", method, path, resp.Status)
	}
	return nil
}

func normalizeBaseURL(domain string, scheme string) (string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("opensearch domain is required")
	}
	if strings.Contains(domain, "://") {
		parsed, err := url.Parse(domain)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(parsed.String(), "/"), nil
	}
	if scheme == "" {
		scheme = "http"
	}
	host := domain
	if _, _, err := net.SplitHostPort(domain); err != nil {
		if !strings.Contains(err.Error(), "missing port in address") {
			return "", err
		}
		host = net.JoinHostPort(domain, "9200")
	}
	return scheme + "://" + host, nil
}

func normalizePage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func totalPages(total int, pageSize int) int {
	if total <= 0 {
		return 0
	}
	return (total + pageSize - 1) / pageSize
}

func parseSearchTotal(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case map[string]interface{}:
		if total, ok := typed["value"].(float64); ok {
			return int(total)
		}
	}
	return 0
}

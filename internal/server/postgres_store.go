package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/llm"
	"github.com/kevinmatthe/record_analysis/internal/model"
	"github.com/kevinmatthe/record_analysis/internal/service"
	"github.com/kevinmatthe/record_analysis/internal/textclean"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresStore struct {
	db *gorm.DB
}

type RecordAnalysisUser struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	Username     string    `gorm:"column:username;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"column:password_hash;not null" json:"-"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (*RecordAnalysisUser) TableName() string { return "record_analysis_users" }

type RecordAnalysisSession struct {
	Token     string    `gorm:"column:token;primaryKey" json:"token"`
	Username  string    `gorm:"column:username;not null;index" json:"username"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null;index" json:"expires_at"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

func (*RecordAnalysisSession) TableName() string { return "record_analysis_sessions" }

type RecordAnalysisJob struct {
	ID               string    `gorm:"column:id;primaryKey" json:"id"`
	Status           string    `gorm:"column:status;not null;default:'queued';index" json:"status"`
	Stage            string    `gorm:"column:stage;not null;default:''" json:"stage"`
	RelationshipID   string    `gorm:"column:relationship_id;not null;index" json:"relationship_id"`
	FileName         string    `gorm:"column:file_name;not null" json:"file_name"`
	MessageCount     int       `gorm:"column:message_count;not null" json:"message_count"`
	LLMMessageLimit  int       `gorm:"column:llm_message_limit;not null" json:"llm_message_limit"`
	LLMMessageCount  int       `gorm:"column:llm_message_count;not null" json:"llm_message_count"`
	AnalysisMode     string    `gorm:"column:analysis_mode;not null;default:'full'" json:"analysis_mode"`
	ProcessedCount   int       `gorm:"column:processed_count;not null;default:0" json:"processed_count"`
	Progress         float64   `gorm:"column:progress;not null;default:0" json:"progress"`
	PreviewTotal     int       `gorm:"column:preview_total;not null;default:0" json:"preview_total"`
	PromptTokens     int       `gorm:"column:prompt_tokens;not null;default:0" json:"prompt_tokens"`
	CompletionTokens int       `gorm:"column:completion_tokens;not null;default:0" json:"completion_tokens"`
	TotalTokens      int       `gorm:"column:total_tokens;not null;default:0" json:"total_tokens"`
	ResultRecordID   string    `gorm:"column:result_record_id;not null;default:''" json:"result_record_id"`
	Error            string    `gorm:"column:error;not null;default:''" json:"error"`
	CreatedAt        time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (*RecordAnalysisJob) TableName() string { return "record_analysis_jobs" }

type RecordAnalysisJobEvent struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	JobID     string    `gorm:"column:job_id;not null;index" json:"job_id"`
	Message   string    `gorm:"column:message;not null" json:"message"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

func (*RecordAnalysisJobEvent) TableName() string { return "record_analysis_job_events" }

type RecordAnalysisJobMessage struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	JobID     string    `gorm:"column:job_id;not null;index:idx_record_analysis_job_messages_job_seq" json:"job_id"`
	Seq       int       `gorm:"column:seq;not null;index:idx_record_analysis_job_messages_job_seq" json:"seq"`
	MsgID     string    `gorm:"column:msg_id;not null" json:"msg_id"`
	Sender    string    `gorm:"column:sender;not null" json:"sender"`
	MsgTime   time.Time `gorm:"column:msg_time;not null" json:"msg_time"`
	MsgType   string    `gorm:"column:msg_type;not null" json:"msg_type"`
	Content   string    `gorm:"column:content;type:text;not null" json:"content"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

func (*RecordAnalysisJobMessage) TableName() string { return "record_analysis_job_messages" }

type RecordAnalysisRecord struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	RelationshipID string    `gorm:"column:relationship_id;not null;index" json:"relationship_id"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	PeriodStart    time.Time `gorm:"column:period_start" json:"period_start"`
	PeriodEnd      time.Time `gorm:"column:period_end" json:"period_end"`
	MessageCount   int       `gorm:"column:message_count;not null" json:"message_count"`
	ActionCount    int       `gorm:"column:action_count;not null" json:"action_count"`
	EventCount     int       `gorm:"column:event_count;not null" json:"event_count"`
	ModelName      string    `gorm:"column:model_name;not null" json:"model_name"`
	ObjectKey      string    `gorm:"column:object_key;not null" json:"object_key"`
	ObjectURI      string    `gorm:"column:object_uri;not null" json:"object_uri"`
	ReportPath     string    `gorm:"column:report_path;not null" json:"report_path"`
	Status         string    `gorm:"column:status;not null;index" json:"status"`
}

func (*RecordAnalysisRecord) TableName() string { return "record_analysis_records" }

type RecordAnalysisBranch struct {
	ID               string    `gorm:"column:id;primaryKey" json:"id"`
	JobID            string    `gorm:"column:job_id;not null;index" json:"job_id"`
	RelationshipID   string    `gorm:"column:relationship_id;not null;index" json:"relationship_id"`
	Title            string    `gorm:"column:title;not null;default:''" json:"title"`
	Granularity      string    `gorm:"column:granularity;not null;default:'hour'" json:"granularity"`
	StartTime        time.Time `gorm:"column:start_time;not null;index" json:"start_time"`
	EndTime          time.Time `gorm:"column:end_time;not null;index" json:"end_time"`
	MessageCount     int       `gorm:"column:message_count;not null;default:0" json:"message_count"`
	BucketIDs        string    `gorm:"column:bucket_ids;type:text;not null;default:'[]'" json:"bucket_ids"`
	ClusterID        string    `gorm:"column:cluster_id;not null;default:''" json:"cluster_id"`
	TopicHint        string    `gorm:"column:topic_hint;type:text;not null;default:''" json:"topic_hint"`
	Status           string    `gorm:"column:status;not null;default:'ready';index" json:"status"`
	Stage            string    `gorm:"column:stage;not null;default:'等待分析'" json:"stage"`
	Progress         float64   `gorm:"column:progress;not null;default:0" json:"progress"`
	PromptTokens     int       `gorm:"column:prompt_tokens;not null;default:0" json:"prompt_tokens"`
	CompletionTokens int       `gorm:"column:completion_tokens;not null;default:0" json:"completion_tokens"`
	TotalTokens      int       `gorm:"column:total_tokens;not null;default:0" json:"total_tokens"`
	ReportMarkdown   string    `gorm:"column:report_markdown;type:text;not null;default:''" json:"report_markdown"`
	ModelName        string    `gorm:"column:model_name;not null;default:''" json:"model_name"`
	Error            string    `gorm:"column:error;not null;default:''" json:"error"`
	CreatedAt        time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (*RecordAnalysisBranch) TableName() string { return "record_analysis_branches" }

type RecordAnalysisWorkItem struct {
	ID               string     `gorm:"column:id;primaryKey" json:"id"`
	JobID            string     `gorm:"column:job_id;not null;index:idx_record_analysis_work_items_job_status;index:idx_record_analysis_work_items_kind_scope,unique" json:"job_id"`
	Kind             string     `gorm:"column:kind;not null;index:idx_record_analysis_work_items_kind_scope,unique" json:"kind"`
	ScopeType        string     `gorm:"column:scope_type;not null;default:'bucket'" json:"scope_type"`
	ScopeID          string     `gorm:"column:scope_id;not null;index:idx_record_analysis_work_items_kind_scope,unique" json:"scope_id"`
	Granularity      string     `gorm:"column:granularity;not null;default:'day';index:idx_record_analysis_work_items_kind_scope,unique" json:"granularity"`
	StartTime        time.Time  `gorm:"column:start_time;not null;index" json:"start_time"`
	EndTime          time.Time  `gorm:"column:end_time;not null;index" json:"end_time"`
	Status           string     `gorm:"column:status;not null;default:'queued';index:idx_record_analysis_work_items_job_status" json:"status"`
	Priority         int        `gorm:"column:priority;not null;default:0;index" json:"priority"`
	Progress         float64    `gorm:"column:progress;not null;default:0" json:"progress"`
	MessageCount     int        `gorm:"column:message_count;not null;default:0" json:"message_count"`
	PromptTokens     int        `gorm:"column:prompt_tokens;not null;default:0" json:"prompt_tokens"`
	CompletionTokens int        `gorm:"column:completion_tokens;not null;default:0" json:"completion_tokens"`
	TotalTokens      int        `gorm:"column:total_tokens;not null;default:0" json:"total_tokens"`
	ResultJSON       string     `gorm:"column:result_json;type:text;not null;default:'[]'" json:"result_json"`
	Error            string     `gorm:"column:error;not null;default:''" json:"error"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
	ClaimedAt        *time.Time `gorm:"column:claimed_at" json:"claimed_at"`
	CompletedAt      *time.Time `gorm:"column:completed_at" json:"completed_at"`
}

func (*RecordAnalysisWorkItem) TableName() string { return "record_analysis_work_items" }

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	if err := ensurePostgresDatabase(dsn); err != nil {
		return nil, err
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	store := &PostgresStore{db: db}
	if err := store.AutoMigrate(); err != nil {
		return nil, err
	}
	return store, nil
}

func ensurePostgresDatabase(dsn string) error {
	dbName := dsnValue(dsn, "dbname")
	if dbName == "" || dbName == "postgres" {
		return nil
	}
	maintenanceDSN := replaceDSNValue(dsn, "dbname", "postgres")
	db, err := gorm.Open(postgres.Open(maintenanceDSN), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var exists bool
	if err := db.Raw("select exists(select 1 from pg_database where datname = ?)", dbName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check postgres database %q: %w", dbName, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec("create database " + quotePostgresIdent(dbName)).Error; err != nil {
		return fmt.Errorf("create postgres database %q: %w", dbName, err)
	}
	return nil
}

func dsnValue(dsn string, key string) string {
	prefix := key + "="
	for _, part := range strings.Fields(dsn) {
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}

func replaceDSNValue(dsn string, key string, value string) string {
	prefix := key + "="
	parts := strings.Fields(dsn)
	for i, part := range parts {
		if strings.HasPrefix(part, prefix) {
			parts[i] = prefix + value
			return strings.Join(parts, " ")
		}
	}
	parts = append(parts, prefix+value)
	return strings.Join(parts, " ")
}

func quotePostgresIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func (s *PostgresStore) AutoMigrate() error {
	return s.db.AutoMigrate(
		&RecordAnalysisUser{},
		&RecordAnalysisSession{},
		&RecordAnalysisJob{},
		&RecordAnalysisJobEvent{},
		&RecordAnalysisJobMessage{},
		&RecordAnalysisRecord{},
		&RecordAnalysisBranch{},
		&RecordAnalysisWorkItem{},
	)
}

func (s *PostgresStore) Save(record service.AnalysisRecord) error {
	return s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(toDBRecord(record)).Error
}

func (s *PostgresStore) List(filter service.AnalysisRecordFilter) ([]service.AnalysisRecord, error) {
	var rows []RecordAnalysisRecord
	query := s.db.Order("created_at desc")
	if filter.RelationshipID != "" {
		query = query.Where("relationship_id = ?", filter.RelationshipID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	records := make([]service.AnalysisRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, fromDBRecord(row))
	}
	return records, nil
}

func (s *PostgresStore) Get(id string) (service.AnalysisRecord, error) {
	var row RecordAnalysisRecord
	if err := s.db.Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return service.AnalysisRecord{}, err
		}
		return service.AnalysisRecord{}, err
	}
	return fromDBRecord(row), nil
}

func (s *PostgresStore) EnsureUser(username string, password string) error {
	var count int64
	if err := s.db.Model(&RecordAnalysisUser{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Create(&RecordAnalysisUser{ID: "usr_" + newSessionToken()[:16], Username: username, PasswordHash: string(hash)}).Error
}

func (s *PostgresStore) CreateUser(username string, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Create(&RecordAnalysisUser{ID: "usr_" + newSessionToken()[:16], Username: username, PasswordHash: string(hash)}).Error
}

func (s *PostgresStore) UpdatePassword(username string, oldPassword string, newPassword string) error {
	if err := s.VerifyUser(username, oldPassword); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Model(&RecordAnalysisUser{}).Where("username = ?", username).Update("password_hash", string(hash)).Error
}

func (s *PostgresStore) VerifyUser(username string, password string) error {
	var user RecordAnalysisUser
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

func (s *PostgresStore) CreateSession(token string, username string, expiresAt time.Time) error {
	return s.db.Create(&RecordAnalysisSession{Token: token, Username: username, ExpiresAt: expiresAt}).Error
}

func (s *PostgresStore) ValidSession(token string) (string, bool) {
	var session RecordAnalysisSession
	if err := s.db.Where("token = ? AND expires_at > ?", token, time.Now()).First(&session).Error; err != nil {
		return "", false
	}
	return session.Username, true
}

func (s *PostgresStore) DeleteSession(token string) error {
	return s.db.Delete(&RecordAnalysisSession{}, "token = ?", token).Error
}

func (s *PostgresStore) CreateJob(ctx context.Context, job *AnalysisJob) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toDBJob(job)).Error; err != nil {
			return err
		}
		for i, message := range job.messages {
			if err := tx.Create(&RecordAnalysisJobMessage{
				JobID:   job.ID,
				Seq:     i + 1,
				MsgID:   message.ID,
				Sender:  message.DisplaySender(),
				MsgTime: message.MsgTime,
				MsgType: message.MsgType,
				Content: message.Content,
			}).Error; err != nil {
				return err
			}
		}
		for _, event := range job.Events {
			if err := tx.Create(&RecordAnalysisJobEvent{JobID: job.ID, Message: event.Message, CreatedAt: event.Time}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *PostgresStore) UpdateJob(ctx context.Context, job *AnalysisJob) error {
	return s.db.WithContext(ctx).Save(toDBJob(job)).Error
}

func (s *PostgresStore) AddJobEvent(ctx context.Context, jobID string, message string) error {
	return s.db.WithContext(ctx).Create(&RecordAnalysisJobEvent{JobID: jobID, Message: message}).Error
}

func (s *PostgresStore) GetJob(ctx context.Context, id string) (AnalysisJob, error) {
	var row RecordAnalysisJob
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return AnalysisJob{}, err
	}
	job := fromDBJob(row)
	job.Events = s.jobEvents(ctx, id)
	if job.ResultRecordID != "" {
		if record, err := s.Get(job.ResultRecordID); err == nil {
			job.Result = &record
		}
	}
	return job, nil
}

func (s *PostgresStore) ListJobs(ctx context.Context, relationshipID string) ([]AnalysisJob, error) {
	var rows []RecordAnalysisJob
	query := s.db.WithContext(ctx).Order("created_at desc").Limit(100)
	if relationshipID != "" {
		query = query.Where("relationship_id = ?", relationshipID)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	jobs := make([]AnalysisJob, 0, len(rows))
	for _, row := range rows {
		job := fromDBJob(row)
		job.Events = s.jobEvents(ctx, job.ID)
		if job.ResultRecordID != "" {
			if record, err := s.Get(job.ResultRecordID); err == nil {
				job.Result = &record
			}
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *PostgresStore) JobPreview(ctx context.Context, jobID string, page int, pageSize int) (PreviewPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&RecordAnalysisJobMessage{}).Where("job_id = ?", jobID).Count(&total).Error; err != nil {
		return PreviewPage{}, err
	}
	var rows []RecordAnalysisJobMessage
	if err := s.db.WithContext(ctx).Where("job_id = ?", jobID).Order("seq asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return PreviewPage{}, err
	}
	messages := make([]model.Message, 0, len(rows))
	for _, row := range rows {
		content := textclean.CleanMessageText(row.Content)
		if !textclean.IsNaturalMessageText(content) {
			continue
		}
		messages = append(messages, model.Message{ID: row.MsgID, Sender: row.Sender, MsgTime: row.MsgTime, MsgType: row.MsgType, Content: content, RawContent: row.Content})
	}
	pageData := previewPage(messages, 1, pageSize)
	pageData.Total = int(total)
	pageData.Page = page
	if total > 0 {
		pageData.TotalPages = (int(total) + pageSize - 1) / pageSize
	}
	return pageData, nil
}

func (s *PostgresStore) JobTimeline(ctx context.Context, jobID string, granularity string, start *time.Time, end *time.Time) ([]service.TimelineBucket, error) {
	messages, err := s.jobMessagesInRange(ctx, jobID, start, end)
	if err != nil {
		return nil, err
	}
	buckets, err := service.BuildTimelineBuckets(messages, granularity)
	if err != nil {
		return nil, err
	}
	return s.decorateTimelineBucketsWithWorkItems(ctx, jobID, granularity, buckets, start, end)
}

func (s *PostgresStore) decorateTimelineBucketsWithWorkItems(ctx context.Context, jobID string, granularity string, buckets []service.TimelineBucket, start *time.Time, end *time.Time) ([]service.TimelineBucket, error) {
	if len(buckets) == 0 {
		return buckets, nil
	}
	var rows []RecordAnalysisWorkItem
	query := s.db.WithContext(ctx).
		Where("job_id = ? AND granularity = ? AND kind IN ?", jobID, granularity, []string{"word_cloud", "topic_summary"}).
		Order("updated_at desc")
	if start != nil {
		query = query.Where("end_time > ?", *start)
	}
	if end != nil {
		query = query.Where("start_time < ?", *end)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return decorateTimelineBuckets(buckets, rows), nil
}

func decorateTimelineBuckets(buckets []service.TimelineBucket, rows []RecordAnalysisWorkItem) []service.TimelineBucket {
	indexByID := make(map[string]int, len(buckets))
	for index := range buckets {
		indexByID[buckets[index].ID] = index
		if buckets[index].AnalysisStatus == "" {
			buckets[index].AnalysisStatus = "unseen"
		}
	}
	for _, row := range rows {
		index, ok := indexByID[row.ScopeID]
		if !ok {
			continue
		}
		bucket := &buckets[index]
		bucket.TotalTokens += row.TotalTokens
		switch row.Kind {
		case "word_cloud":
			bucket.WordCloudStatus = mergeWorkItemStatus(bucket.WordCloudStatus, row.Status)
		case "topic_summary":
			bucket.SummaryStatus = mergeWorkItemStatus(bucket.SummaryStatus, row.Status)
			if row.Status == "completed" {
				var summary llm.TopicSummary
				if err := json.Unmarshal([]byte(row.ResultJSON), &summary); err == nil {
					if summary.Title != "" {
						bucket.SummaryTitle = summary.Title
					}
					if len(summary.Topics) > 0 {
						bucket.SummaryTopics = append([]string(nil), summary.Topics...)
					}
				}
			}
		}
		bucket.AnalysisStatus = mergeWorkItemStatus(bucket.AnalysisStatus, row.Status)
	}
	return buckets
}

func mergeWorkItemStatus(current string, next string) string {
	rank := map[string]int{
		"running":   5,
		"queued":    4,
		"failed":    3,
		"completed": 2,
		"unseen":    1,
		"":          0,
	}
	if rank[next] > rank[current] {
		return next
	}
	if current == "" {
		return next
	}
	return current
}

func (s *PostgresStore) JobTimelineBucketMessages(ctx context.Context, jobID string, bucketID string, page int, pageSize int) (PreviewPage, error) {
	messages, err := s.jobMessages(ctx, jobID)
	if err != nil {
		return PreviewPage{}, err
	}
	filtered, err := service.FilterMessagesForBucket(messages, bucketID)
	if err != nil {
		return PreviewPage{}, err
	}
	return previewPage(filtered, page, pageSize), nil
}

func (s *PostgresStore) jobEvents(ctx context.Context, jobID string) []JobEvent {
	var rows []RecordAnalysisJobEvent
	_ = s.db.WithContext(ctx).Where("job_id = ?", jobID).Order("created_at asc").Limit(80).Find(&rows).Error
	events := make([]JobEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, JobEvent{Time: row.CreatedAt, Message: row.Message})
	}
	return events
}

func (s *PostgresStore) jobMessages(ctx context.Context, jobID string) ([]model.Message, error) {
	return s.jobMessagesInRange(ctx, jobID, nil, nil)
}

func (s *PostgresStore) jobMessagesByIDs(ctx context.Context, jobID string, ids []string) ([]model.Message, error) {
	ids = sanitizeMessageIDs(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []RecordAnalysisJobMessage
	if err := s.db.WithContext(ctx).Where("job_id = ? AND msg_id IN ?", jobID, ids).Order("seq asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	messages := make([]model.Message, 0, len(rows))
	senderMap := map[string]string{}
	for _, row := range rows {
		content := textclean.CleanMessageText(row.Content)
		if !textclean.IsNaturalMessageText(content) {
			continue
		}
		sender := normalizeStoredSender(row.Sender, senderMap)
		messages = append(messages, model.Message{
			ID:             row.MsgID,
			Sender:         sender,
			OriginalSender: row.Sender,
			MsgTime:        row.MsgTime,
			MsgType:        row.MsgType,
			Content:        content,
			RawContent:     row.Content,
		})
	}
	return messages, nil
}

func (s *PostgresStore) jobMessagesInRange(ctx context.Context, jobID string, start *time.Time, end *time.Time) ([]model.Message, error) {
	var rows []RecordAnalysisJobMessage
	query := s.db.WithContext(ctx).Where("job_id = ?", jobID)
	if start != nil {
		query = query.Where("msg_time >= ?", *start)
	}
	if end != nil {
		query = query.Where("msg_time < ?", *end)
	}
	if err := query.Order("seq asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	messages := make([]model.Message, 0, len(rows))
	senderMap := map[string]string{}
	for _, row := range rows {
		content := textclean.CleanMessageText(row.Content)
		if !textclean.IsNaturalMessageText(content) {
			continue
		}
		sender := normalizeStoredSender(row.Sender, senderMap)
		messages = append(messages, model.Message{
			ID:             row.MsgID,
			Sender:         sender,
			OriginalSender: row.Sender,
			MsgTime:        row.MsgTime,
			MsgType:        row.MsgType,
			Content:        content,
			RawContent:     row.Content,
		})
	}
	return messages, nil
}

func normalizeStoredSender(sender string, senderMap map[string]string) string {
	sender = strings.TrimSpace(sender)
	if sender == "" {
		return "UNKNOWN"
	}
	if sender == "PERSON_A" || sender == "PERSON_B" || strings.HasPrefix(sender, "PERSON_") {
		return sender
	}
	if sender == "我" {
		return "PERSON_A"
	}
	if sender == "系统" {
		return "系统"
	}
	if existing, ok := senderMap[sender]; ok {
		return existing
	}
	if len(senderMap) == 0 {
		senderMap[sender] = "PERSON_B"
	} else {
		senderMap[sender] = fmt.Sprintf("PERSON_%d", len(senderMap)+2)
	}
	return senderMap[sender]
}

func (s *PostgresStore) CreateBranch(ctx context.Context, branch *AnalysisBranch) error {
	var existing RecordAnalysisBranch
	err := s.db.WithContext(ctx).
		Where("job_id = ? AND granularity = ? AND start_time = ? AND end_time = ?", branch.JobID, branch.Granularity, branch.StartTime, branch.EndTime).
		First(&existing).Error
	if err == nil {
		*branch = fromDBBranch(existing)
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.WithContext(ctx).Create(toDBBranch(branch)).Error
}

func (s *PostgresStore) ListBranches(ctx context.Context, jobID string) ([]AnalysisBranch, error) {
	var rows []RecordAnalysisBranch
	if err := s.db.WithContext(ctx).Where("job_id = ?", jobID).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AnalysisBranch, 0, len(rows))
	for _, row := range rows {
		items = append(items, fromDBBranch(row))
	}
	return items, nil
}

func (s *PostgresStore) GetBranch(ctx context.Context, id string) (AnalysisBranch, error) {
	var row RecordAnalysisBranch
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return AnalysisBranch{}, err
	}
	return fromDBBranch(row), nil
}

func (s *PostgresStore) UpdateBranch(ctx context.Context, branch *AnalysisBranch) error {
	return s.db.WithContext(ctx).Save(toDBBranch(branch)).Error
}

func (s *PostgresStore) SeedWordCloudWorkItems(ctx context.Context, jobID string, granularity string) ([]AnalysisWorkItem, error) {
	return s.SeedWorkItems(ctx, jobID, "word_cloud", granularity, nil, nil)
}

func (s *PostgresStore) SeedWorkItems(ctx context.Context, jobID string, kind string, granularity string, start *time.Time, end *time.Time) ([]AnalysisWorkItem, error) {
	if kind == "" {
		kind = "word_cloud"
	}
	buckets, err := s.JobTimeline(ctx, jobID, granularity, start, end)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, bucket := range buckets {
		row := &RecordAnalysisWorkItem{
			ID:           "wi_" + newSessionToken()[:16],
			JobID:        jobID,
			Kind:         kind,
			ScopeType:    "bucket",
			ScopeID:      bucket.ID,
			Granularity:  bucket.Granularity,
			StartTime:    bucket.StartTime,
			EndTime:      bucket.EndTime,
			Status:       "queued",
			Priority:     0,
			Progress:     0,
			MessageCount: bucket.MessageCount,
			ResultJSON:   "[]",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
			return nil, err
		}
	}
	return s.ListWorkItemsInRange(ctx, jobID, kind, granularity, start, end)
}

func (s *PostgresStore) CreateMergeWorkItem(ctx context.Context, jobID string, granularity string, start time.Time, end time.Time, priority int) (AnalysisWorkItem, error) {
	scopeID := fmt.Sprintf("merge_%s_%s_%s", granularity, start.Format(time.RFC3339), end.Format(time.RFC3339))
	now := time.Now()
	row := &RecordAnalysisWorkItem{
		ID:          "wi_" + newSessionToken()[:16],
		JobID:       jobID,
		Kind:        "summary_merge",
		ScopeType:   "range",
		ScopeID:     scopeID,
		Granularity: granularity,
		StartTime:   start,
		EndTime:     end,
		Status:      "queued",
		Priority:    priority,
		Progress:    0,
		ResultJSON:  "{}",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error
	if err != nil {
		return AnalysisWorkItem{}, err
	}
	var existing RecordAnalysisWorkItem
	if err := s.db.WithContext(ctx).Where("job_id = ? AND kind = ? AND granularity = ? AND scope_id = ?", jobID, "summary_merge", granularity, scopeID).First(&existing).Error; err != nil {
		return AnalysisWorkItem{}, err
	}
	return fromDBWorkItem(existing), nil
}

func (s *PostgresStore) CompletedTopicSummariesInRange(ctx context.Context, jobID string, granularity string, start time.Time, end time.Time, limit int) ([]AnalysisWorkItem, error) {
	if limit <= 0 {
		limit = 80
	}
	var rows []RecordAnalysisWorkItem
	err := s.db.WithContext(ctx).
		Where("job_id = ? AND kind = ? AND granularity = ? AND status = ? AND start_time >= ? AND end_time <= ?", jobID, "topic_summary", granularity, "completed", start, end).
		Order("start_time asc").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]AnalysisWorkItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, fromDBWorkItem(row))
	}
	return items, nil
}

func (s *PostgresStore) ListWorkItems(ctx context.Context, jobID string, kind string, granularity string) ([]AnalysisWorkItem, error) {
	return s.ListWorkItemsInRange(ctx, jobID, kind, granularity, nil, nil)
}

func (s *PostgresStore) ListWorkItemsInRange(ctx context.Context, jobID string, kind string, granularity string, start *time.Time, end *time.Time) ([]AnalysisWorkItem, error) {
	var rows []RecordAnalysisWorkItem
	query := s.db.WithContext(ctx).Where("job_id = ?", jobID).Order("start_time asc")
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if granularity != "" {
		query = query.Where("granularity = ?", granularity)
	}
	if start != nil {
		query = query.Where("end_time > ?", *start)
	}
	if end != nil {
		query = query.Where("start_time < ?", *end)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AnalysisWorkItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, fromDBWorkItem(row))
	}
	return items, nil
}

func (s *PostgresStore) PrioritizeWorkItem(ctx context.Context, id string) (AnalysisWorkItem, error) {
	err := s.db.WithContext(ctx).Model(&RecordAnalysisWorkItem{}).
		Where("id = ? AND status IN ?", id, []string{"queued", "failed"}).
		Updates(map[string]interface{}{"priority": 100, "status": "queued", "error": "", "updated_at": time.Now()}).Error
	if err != nil {
		return AnalysisWorkItem{}, err
	}
	var row RecordAnalysisWorkItem
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return AnalysisWorkItem{}, err
	}
	return fromDBWorkItem(row), nil
}

func (s *PostgresStore) ClaimNextWorkItem(ctx context.Context) (AnalysisWorkItem, bool, error) {
	var row RecordAnalysisWorkItem
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []RecordAnalysisWorkItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", "queued").
			Order("priority desc, created_at asc").
			Limit(1).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return gorm.ErrRecordNotFound
		}
		row = rows[0]
		now := time.Now()
		return tx.Model(&RecordAnalysisWorkItem{}).Where("id = ?", row.ID).
			Updates(map[string]interface{}{"status": "running", "progress": 0.2, "claimed_at": now, "updated_at": now}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AnalysisWorkItem{}, false, nil
	}
	if err != nil {
		return AnalysisWorkItem{}, false, err
	}
	return fromDBWorkItem(row), true, nil
}

func (s *PostgresStore) CompleteWorkItem(ctx context.Context, id string, result interface{}) error {
	return s.CompleteWorkItemWithUsage(ctx, id, result, llm.UsageEvent{})
}

func (s *PostgresStore) CompleteWorkItemWithUsage(ctx context.Context, id string, result interface{}, usage llm.UsageEvent) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	now := time.Now()
	return s.db.WithContext(ctx).Model(&RecordAnalysisWorkItem{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":            "completed",
			"progress":          1,
			"result_json":       string(resultJSON),
			"prompt_tokens":     usage.PromptTokens,
			"completion_tokens": usage.CompletionTokens,
			"total_tokens":      usage.TotalTokens,
			"error":             "",
			"completed_at":      now,
			"updated_at":        now,
		}).Error
}

func (s *PostgresStore) FailWorkItem(ctx context.Context, id string, err error) error {
	return s.db.WithContext(ctx).Model(&RecordAnalysisWorkItem{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "failed", "progress": 0, "error": err.Error(), "updated_at": time.Now()}).Error
}

func toDBRecord(record service.AnalysisRecord) *RecordAnalysisRecord {
	return &RecordAnalysisRecord{
		ID:             record.ID,
		RelationshipID: record.RelationshipID,
		CreatedAt:      record.CreatedAt,
		PeriodStart:    record.PeriodStart,
		PeriodEnd:      record.PeriodEnd,
		MessageCount:   record.MessageCount,
		ActionCount:    record.ActionCount,
		EventCount:     record.EventCount,
		ModelName:      record.ModelName,
		ObjectKey:      record.ObjectKey,
		ObjectURI:      record.ObjectURI,
		ReportPath:     record.ReportPath,
		Status:         record.Status,
	}
}

func fromDBRecord(row RecordAnalysisRecord) service.AnalysisRecord {
	return service.AnalysisRecord{
		ID:             row.ID,
		RelationshipID: row.RelationshipID,
		CreatedAt:      row.CreatedAt,
		PeriodStart:    row.PeriodStart,
		PeriodEnd:      row.PeriodEnd,
		MessageCount:   row.MessageCount,
		ActionCount:    row.ActionCount,
		EventCount:     row.EventCount,
		ModelName:      row.ModelName,
		ObjectKey:      row.ObjectKey,
		ObjectURI:      row.ObjectURI,
		ReportPath:     row.ReportPath,
		Status:         row.Status,
	}
}

func toDBJob(job *AnalysisJob) *RecordAnalysisJob {
	return &RecordAnalysisJob{
		ID:               job.ID,
		Status:           string(job.Status),
		Stage:            job.Stage,
		RelationshipID:   job.RelationshipID,
		FileName:         job.FileName,
		MessageCount:     job.MessageCount,
		LLMMessageLimit:  job.LLMMessageLimit,
		LLMMessageCount:  job.LLMMessageCount,
		AnalysisMode:     job.AnalysisMode,
		ProcessedCount:   job.ProcessedCount,
		Progress:         job.Progress,
		PreviewTotal:     job.PreviewTotal,
		PromptTokens:     job.PromptTokens,
		CompletionTokens: job.CompletionTokens,
		TotalTokens:      job.TotalTokens,
		ResultRecordID:   job.ResultRecordID,
		Error:            job.Error,
		CreatedAt:        job.CreatedAt,
		UpdatedAt:        job.UpdatedAt,
	}
}

func fromDBJob(row RecordAnalysisJob) AnalysisJob {
	return AnalysisJob{
		ID:               row.ID,
		Status:           JobStatus(row.Status),
		Stage:            row.Stage,
		RelationshipID:   row.RelationshipID,
		FileName:         row.FileName,
		MessageCount:     row.MessageCount,
		LLMMessageLimit:  row.LLMMessageLimit,
		LLMMessageCount:  row.LLMMessageCount,
		AnalysisMode:     row.AnalysisMode,
		ProcessedCount:   row.ProcessedCount,
		Progress:         row.Progress,
		PreviewTotal:     row.PreviewTotal,
		PromptTokens:     row.PromptTokens,
		CompletionTokens: row.CompletionTokens,
		TotalTokens:      row.TotalTokens,
		ResultRecordID:   row.ResultRecordID,
		Error:            row.Error,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func toDBBranch(branch *AnalysisBranch) *RecordAnalysisBranch {
	data, _ := json.Marshal(branch.BucketIDs)
	return &RecordAnalysisBranch{
		ID:               branch.ID,
		JobID:            branch.JobID,
		RelationshipID:   branch.RelationshipID,
		Title:            branch.Title,
		Granularity:      branch.Granularity,
		StartTime:        branch.StartTime,
		EndTime:          branch.EndTime,
		MessageCount:     branch.MessageCount,
		BucketIDs:        string(data),
		ClusterID:        branch.ClusterID,
		TopicHint:        branch.TopicHint,
		Status:           branch.Status,
		Stage:            branch.Stage,
		Progress:         branch.Progress,
		PromptTokens:     branch.PromptTokens,
		CompletionTokens: branch.CompletionTokens,
		TotalTokens:      branch.TotalTokens,
		ReportMarkdown:   branch.ReportMarkdown,
		ModelName:        branch.ModelName,
		Error:            branch.Error,
		CreatedAt:        branch.CreatedAt,
		UpdatedAt:        branch.UpdatedAt,
	}
}

func fromDBBranch(row RecordAnalysisBranch) AnalysisBranch {
	var bucketIDs []string
	_ = json.Unmarshal([]byte(row.BucketIDs), &bucketIDs)
	return AnalysisBranch{
		ID:               row.ID,
		JobID:            row.JobID,
		RelationshipID:   row.RelationshipID,
		Title:            row.Title,
		Granularity:      row.Granularity,
		StartTime:        row.StartTime,
		EndTime:          row.EndTime,
		MessageCount:     row.MessageCount,
		BucketIDs:        bucketIDs,
		ClusterID:        row.ClusterID,
		TopicHint:        row.TopicHint,
		Status:           row.Status,
		Stage:            row.Stage,
		Progress:         row.Progress,
		PromptTokens:     row.PromptTokens,
		CompletionTokens: row.CompletionTokens,
		TotalTokens:      row.TotalTokens,
		ReportMarkdown:   row.ReportMarkdown,
		ModelName:        row.ModelName,
		Error:            row.Error,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func fromDBWorkItem(row RecordAnalysisWorkItem) AnalysisWorkItem {
	result := json.RawMessage(row.ResultJSON)
	if len(result) == 0 || !json.Valid(result) {
		result = json.RawMessage("[]")
	}
	return AnalysisWorkItem{
		ID:               row.ID,
		JobID:            row.JobID,
		Kind:             row.Kind,
		ScopeType:        row.ScopeType,
		ScopeID:          row.ScopeID,
		Granularity:      row.Granularity,
		StartTime:        row.StartTime,
		EndTime:          row.EndTime,
		Status:           row.Status,
		Priority:         row.Priority,
		Progress:         row.Progress,
		MessageCount:     row.MessageCount,
		PromptTokens:     row.PromptTokens,
		CompletionTokens: row.CompletionTokens,
		TotalTokens:      row.TotalTokens,
		Result:           result,
		Error:            row.Error,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		ClaimedAt:        row.ClaimedAt,
		CompletedAt:      row.CompletedAt,
	}
}

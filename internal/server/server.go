package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/analyzer"
	"github.com/kevinmatthe/record_analysis/internal/importer"
	"github.com/kevinmatthe/record_analysis/internal/llm"
	"github.com/kevinmatthe/record_analysis/internal/model"
	"github.com/kevinmatthe/record_analysis/internal/search"
	"github.com/kevinmatthe/record_analysis/internal/service"
)

type analyzeRequest struct {
	TempPath       string
	FileName       string
	RelationshipID string
	IncludeSystem  bool
	From           *time.Time
	To             *time.Time
	MaxLLMMessages int
	AnalysisMode   string
}

type Options struct {
	MaxUploadBytes int64
	MaxLLMMessages int
	AuthUsername   string
	AuthPassword   string
	AllowedOrigin  string
	Store          *PostgresStore
	Indexer        search.Indexer
}

type Handler struct {
	svc      *service.ChatAnalysisService
	options  Options
	sessions map[string]time.Time
	jobs     map[string]*AnalysisJob
	branches map[string]*AnalysisBranch
	store    *PostgresStore
	indexer  search.Indexer
	mu       sync.Mutex
}

func NewHandler(svc *service.ChatAnalysisService, options Options) http.Handler {
	if options.MaxUploadBytes <= 0 {
		options.MaxUploadBytes = 64 << 20
	}
	indexer := options.Indexer
	if indexer == nil {
		indexer = search.NoopIndexer{}
	}
	h := &Handler{svc: svc, options: options, sessions: map[string]time.Time{}, jobs: map[string]*AnalysisJob{}, branches: map[string]*AnalysisBranch{}, store: options.Store, indexer: indexer}
	if h.store != nil {
		go h.runWorkItemWorker()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", h.withCORS(h.loginHandler))
	mux.HandleFunc("/api/auth/register", h.withCORS(h.registerHandler))
	mux.HandleFunc("/api/auth/password", h.withCORS(h.requireAuth(h.passwordHandler)))
	mux.HandleFunc("/api/auth/logout", h.withCORS(h.requireAuth(h.logoutHandler)))
	mux.HandleFunc("/api/auth/me", h.withCORS(h.requireAuth(h.meHandler)))
	mux.HandleFunc("/api/analyze", h.withCORS(h.requireAuth(h.analyzeHandler)))
	mux.HandleFunc("/api/analyses", h.withCORS(h.requireAuth(h.analysesHandler)))
	mux.HandleFunc("/api/analyses/", h.withCORS(h.requireAuth(h.analysisDetailHandler)))
	mux.HandleFunc("/api/system/status", h.withCORS(h.requireAuth(h.systemStatusHandler)))
	mux.HandleFunc("/api/jobs", h.withCORS(h.requireAuth(h.jobsHandler)))
	mux.HandleFunc("/api/jobs/", h.withCORS(h.requireAuth(h.jobDetailHandler)))
	mux.HandleFunc("/api/branches/", h.withCORS(h.requireAuth(h.branchDetailHandler)))
	return mux
}

func (h *Handler) systemStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"postgres": map[string]interface{}{
			"enabled": h.store != nil,
		},
		"opensearch": h.indexer.Status(r.Context()),
	})
}

func (h *Handler) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.options.AllowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", h.options.AllowedOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.authEnabled() {
			next(w, r)
			return
		}
		cookie, err := r.Cookie("record_analysis_session")
		if err != nil {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
			return
		}
		username, ok := h.validSession(cookie.Value)
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, username)))
	}
}

func (h *Handler) authEnabled() bool {
	return h.store != nil || h.options.AuthUsername != "" || h.options.AuthPassword != ""
}

func (h *Handler) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid login payload"))
		return
	}
	if h.store != nil {
		if err := h.store.VerifyUser(request.Username, request.Password); err != nil {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid username or password"))
			return
		}
	} else if h.authEnabled() && (request.Username != h.options.AuthUsername || request.Password != h.options.AuthPassword) {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid username or password"))
		return
	}
	token := newSessionToken()
	expiresAt := time.Now().Add(24 * time.Hour)
	if h.store != nil {
		if err := h.store.CreateSession(token, request.Username, expiresAt); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		h.mu.Lock()
		h.sessions[token] = expiresAt
		h.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "record_analysis_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "username": request.Username})
}

func (h *Handler) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	if h.store == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("registration requires postgres store"))
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid register payload"))
		return
	}
	if strings.TrimSpace(request.Username) == "" || len(request.Password) < 6 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("username is required and password must be at least 6 characters"))
		return
	}
	if err := h.store.CreateUser(request.Username, request.Password); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"ok": true, "username": request.Username})
}

func (h *Handler) passwordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	if h.store == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("password update requires postgres store"))
		return
	}
	var request struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid password payload"))
		return
	}
	username, _ := r.Context().Value(userContextKey{}).(string)
	if len(request.NewPassword) < 6 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("new password must be at least 6 characters"))
		return
	}
	if err := h.store.UpdatePassword(username, request.OldPassword, request.NewPassword); err != nil {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("password update failed"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	if cookie, err := r.Cookie("record_analysis_session"); err == nil {
		if h.store != nil {
			_ = h.store.DeleteSession(cookie.Value)
		} else {
			h.mu.Lock()
			delete(h.sessions, cookie.Value)
			h.mu.Unlock()
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "record_analysis_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) meHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	username := h.options.AuthUsername
	if value, ok := r.Context().Value(userContextKey{}).(string); ok && value != "" {
		username = value
	}
	if username == "" {
		username = "anonymous"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"authenticated": true, "username": username})
}

func (h *Handler) analysesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := h.svc.ListAnalyses(service.AnalysisRecordFilter{
			RelationshipID: strings.TrimSpace(r.URL.Query().Get("relationship_id")),
			Status:         strings.TrimSpace(r.URL.Query().Get("status")),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": records})
	case http.MethodPost:
		h.analyzeHandler(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}
}

func (h *Handler) jobsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.listJobsHandler(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	request, err := h.parseAnalyzeRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	messages, err := importer.ParseChatFile(request.TempPath, request.RelationshipID, request.IncludeSystem)
	if err != nil {
		os.Remove(request.TempPath)
		writeError(w, http.StatusBadRequest, err)
		return
	}
	messages = filterServerPeriod(messages, request.From, request.To)
	llmCount := len(messages)
	if request.MaxLLMMessages > 0 && llmCount > request.MaxLLMMessages {
		llmCount = request.MaxLLMMessages
	}
	job := &AnalysisJob{
		ID:              "job_" + newSessionToken()[:16],
		Status:          JobStatusQueued,
		Stage:           "已解析上传文件，等待分析",
		RelationshipID:  request.RelationshipID,
		FileName:        request.FileName,
		MessageCount:    len(messages),
		LLMMessageLimit: request.MaxLLMMessages,
		LLMMessageCount: llmCount,
		AnalysisMode:    request.AnalysisMode,
		Progress:        0.1,
		PreviewTotal:    len(messages),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Events:          []JobEvent{{Time: time.Now(), Message: fmt.Sprintf("解析完成：%d 条可处理消息", len(messages))}},
		messages:        messages,
	}
	h.mu.Lock()
	h.jobs[job.ID] = job
	h.mu.Unlock()
	if h.store != nil {
		if err := h.store.CreateJob(r.Context(), job); err != nil {
			os.Remove(request.TempPath)
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	if err := h.indexer.IndexMessages(r.Context(), search.MessageDocuments(job.ID, job.RelationshipID, messages)); err != nil {
		h.updateJob(job.ID, JobStatusQueued, "OpenSearch 索引失败，已降级到 Postgres 查询："+err.Error(), 0.1, 0, nil)
	}
	go h.runAnalysisJob(job.ID, request)
	writeJSON(w, http.StatusAccepted, cloneJob(job))
}

func (h *Handler) listJobsHandler(w http.ResponseWriter, r *http.Request) {
	relationshipID := strings.TrimSpace(r.URL.Query().Get("relationship_id"))
	if h.store != nil {
		jobs, err := h.store.ListJobs(r.Context(), relationshipID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": jobs})
		return
	}
	h.mu.Lock()
	jobs := make([]AnalysisJob, 0, len(h.jobs))
	for _, job := range h.jobs {
		if relationshipID != "" && job.RelationshipID != relationshipID {
			continue
		}
		jobs = append(jobs, cloneJob(job))
	}
	h.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": jobs})
}

func (h *Handler) jobDetailHandler(w http.ResponseWriter, r *http.Request) {
	id, suffix := splitJobPath(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if h.store != nil {
		if suffix == "branches/preview" && r.Method == http.MethodPost {
			h.handleBranchPreview(w, r, id, true)
			return
		}
		if suffix == "branches" && r.Method == http.MethodPost {
			h.handleCreateBranch(w, r, id, true)
			return
		}
		if suffix == "branches" && r.Method == http.MethodGet {
			h.handleListBranches(w, r, id, true)
			return
		}
		if strings.HasPrefix(suffix, "branches/") && strings.HasSuffix(suffix, "/run") && r.Method == http.MethodPost {
			branchID := strings.TrimSuffix(strings.TrimPrefix(suffix, "branches/"), "/run")
			h.handleRunBranch(w, r, id, branchID, true)
			return
		}
		if suffix == "work-items" && r.Method == http.MethodGet {
			h.handleListWorkItems(w, r, id)
			return
		}
		if suffix == "messages/search" && r.Method == http.MethodGet {
			h.handleSearchMessages(w, r, id, true)
			return
		}
		if suffix == "work-items/seed" && r.Method == http.MethodPost {
			h.handleSeedWorkItems(w, r, id)
			return
		}
		if suffix == "work-items/merge" && r.Method == http.MethodPost {
			h.handleCreateSummaryMergeWorkItem(w, r, id)
			return
		}
		if strings.HasPrefix(suffix, "work-items/") && strings.HasSuffix(suffix, "/prioritize") && r.Method == http.MethodPost {
			itemID := strings.TrimSuffix(strings.TrimPrefix(suffix, "work-items/"), "/prioritize")
			h.handlePrioritizeWorkItem(w, r, itemID)
			return
		}
		if suffix == "preview" {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
			preview, err := h.store.JobPreview(r.Context(), id, page, pageSize)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, preview)
			return
		}
		if suffix == "timeline" {
			start, end, err := parseTimelineRange(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			items, err := h.store.JobTimeline(r.Context(), id, r.URL.Query().Get("granularity"), start, end)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"granularity": serviceGranularity(r.URL.Query().Get("granularity")),
				"items":       items,
				"clusters":    service.BuildTimelineClusters(items),
			})
			return
		}
		if strings.HasPrefix(suffix, "timeline/") && strings.HasSuffix(suffix, "/messages") {
			bucketID := strings.TrimSuffix(strings.TrimPrefix(suffix, "timeline/"), "/messages")
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
			preview, err := h.store.JobTimelineBucketMessages(r.Context(), id, bucketID, page, pageSize)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, preview)
			return
		}
		if suffix != "" {
			http.NotFound(w, r)
			return
		}
		job, err := h.store.GetJob(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, job)
		return
	}
	h.mu.Lock()
	job, ok := h.jobs[id]
	if !ok {
		h.mu.Unlock()
		writeError(w, http.StatusNotFound, fmt.Errorf("job not found"))
		return
	}
	if suffix == "preview" {
		messages := append([]model.Message(nil), job.messages...)
		h.mu.Unlock()
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		writeJSON(w, http.StatusOK, previewPage(messages, page, pageSize))
		return
	}
	if suffix == "branches" && r.Method == http.MethodPost {
		h.mu.Unlock()
		h.handleCreateBranchFromMessages(w, r, append([]model.Message(nil), job.messages...), id, job.RelationshipID)
		return
	}
	if suffix == "branches" && r.Method == http.MethodGet {
		branches := h.listBranchesMemory(id)
		h.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": branches})
		return
	}
	if strings.HasPrefix(suffix, "branches/") && strings.HasSuffix(suffix, "/run") && r.Method == http.MethodPost {
		branchID := strings.TrimSuffix(strings.TrimPrefix(suffix, "branches/"), "/run")
		h.mu.Unlock()
		h.handleRunBranchMemory(w, r, id, branchID)
		return
	}
	if suffix == "branches/preview" && r.Method == http.MethodPost {
		h.mu.Unlock()
		h.handleBranchPreviewFromMessages(w, r, append([]model.Message(nil), job.messages...), id)
		return
	}
	if suffix == "timeline" {
		messages := append([]model.Message(nil), job.messages...)
		h.mu.Unlock()
		start, end, err := parseTimelineRange(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		messages = service.FilterMessagesByTimeRange(messages, start, end)
		items, err := service.BuildTimelineBuckets(messages, r.URL.Query().Get("granularity"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"granularity": serviceGranularity(r.URL.Query().Get("granularity")),
			"items":       items,
			"clusters":    service.BuildTimelineClusters(items),
		})
		return
	}
	if strings.HasPrefix(suffix, "timeline/") && strings.HasSuffix(suffix, "/messages") {
		messages := append([]model.Message(nil), job.messages...)
		h.mu.Unlock()
		bucketID := strings.TrimSuffix(strings.TrimPrefix(suffix, "timeline/"), "/messages")
		filtered, err := service.FilterMessagesForBucket(messages, bucketID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		writeJSON(w, http.StatusOK, previewPage(filtered, page, pageSize))
		return
	}
	if suffix == "messages/search" && r.Method == http.MethodGet {
		messages := append([]model.Message(nil), job.messages...)
		h.mu.Unlock()
		h.handleSearchMessagesFromMessages(w, r, id, messages)
		return
	}
	copy := cloneJob(job)
	h.mu.Unlock()
	if suffix != "" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, copy)
}

func (h *Handler) analysisDetailHandler(w http.ResponseWriter, r *http.Request) {
	id, suffix := splitAnalysisPath(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch {
	case r.Method == http.MethodGet && suffix == "":
		record, err := h.svc.GetAnalysisRecord(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, record)
	case r.Method == http.MethodGet && suffix == "report":
		report, record, err := h.svc.ReadReport(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"record": record, "report_markdown": report})
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) branchDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/branches/"), "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if h.store != nil {
		branch, err := h.store.GetBranch(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, branch)
		return
	}
	h.mu.Lock()
	branch, ok := h.branches[id]
	if !ok {
		h.mu.Unlock()
		writeError(w, http.StatusNotFound, fmt.Errorf("branch not found"))
		return
	}
	copy := *branch
	h.mu.Unlock()
	writeJSON(w, http.StatusOK, copy)
}

func (h *Handler) analyzeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.options.MaxUploadBytes)
	if err := r.ParseMultipartForm(h.options.MaxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("parse upload: %w", err))
		return
	}
	relationshipID := strings.TrimSpace(r.FormValue("relationship_id"))
	if relationshipID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("relationship_id is required"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("file is required"))
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".txt" && ext != ".csv" && ext != ".json" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported file format: %s", ext))
		return
	}
	temp, err := os.CreateTemp("", "record-analysis-*"+ext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := io.Copy(temp, file); err != nil {
		temp.Close()
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := temp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	from, err := parseFormTime(r.FormValue("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	to, err := parseFormTime(r.FormValue("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	maxLLMMessages := h.options.MaxLLMMessages
	if value := strings.TrimSpace(r.FormValue("max_llm_messages")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("max_llm_messages must be a non-negative integer"))
			return
		}
		maxLLMMessages = parsed
	}
	analysisMode := parseAnalysisMode(r.FormValue("analysis_mode"))
	result, err := h.svc.UploadAndAnalyzeWithOptions(tempPath, relationshipID, service.UploadAnalyzeOptions{
		Context:        r.Context(),
		IncludeSystem:  parseBool(r.FormValue("include_system")),
		From:           from,
		To:             to,
		MaxLLMMessages: maxLLMMessages,
		AnalysisMode:   analysisMode,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) parseAnalyzeRequest(w http.ResponseWriter, r *http.Request) (analyzeRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, h.options.MaxUploadBytes)
	if err := r.ParseMultipartForm(h.options.MaxUploadBytes); err != nil {
		return analyzeRequest{}, fmt.Errorf("parse upload: %w", err)
	}
	relationshipID := strings.TrimSpace(r.FormValue("relationship_id"))
	if relationshipID == "" {
		return analyzeRequest{}, fmt.Errorf("relationship_id is required")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return analyzeRequest{}, fmt.Errorf("file is required")
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".txt" && ext != ".csv" && ext != ".json" {
		return analyzeRequest{}, fmt.Errorf("unsupported file format: %s", ext)
	}
	temp, err := os.CreateTemp("", "record-analysis-*"+ext)
	if err != nil {
		return analyzeRequest{}, err
	}
	tempPath := temp.Name()
	if _, err := io.Copy(temp, file); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return analyzeRequest{}, err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return analyzeRequest{}, err
	}
	from, err := parseFormTime(r.FormValue("from"))
	if err != nil {
		os.Remove(tempPath)
		return analyzeRequest{}, err
	}
	to, err := parseFormTime(r.FormValue("to"))
	if err != nil {
		os.Remove(tempPath)
		return analyzeRequest{}, err
	}
	maxLLMMessages := h.options.MaxLLMMessages
	if value := strings.TrimSpace(r.FormValue("max_llm_messages")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			os.Remove(tempPath)
			return analyzeRequest{}, fmt.Errorf("max_llm_messages must be a non-negative integer")
		}
		maxLLMMessages = parsed
	}
	analysisMode := parseAnalysisMode(r.FormValue("analysis_mode"))
	return analyzeRequest{
		TempPath:       tempPath,
		FileName:       header.Filename,
		RelationshipID: relationshipID,
		IncludeSystem:  parseBool(r.FormValue("include_system")),
		From:           from,
		To:             to,
		MaxLLMMessages: maxLLMMessages,
		AnalysisMode:   analysisMode,
	}, nil
}

func (h *Handler) runAnalysisJob(jobID string, request analyzeRequest) {
	defer os.Remove(request.TempPath)
	h.updateJob(jobID, JobStatusRunning, "开始执行分析", 0.15, 0, func(job *AnalysisJob) {})
	ctx := llm.WithUsageReporter(context.Background(), func(event llm.UsageEvent) {
		h.updateJobUsage(jobID, event)
	})
	result, err := h.svc.UploadAndAnalyzeWithOptions(request.TempPath, request.RelationshipID, service.UploadAnalyzeOptions{
		Context:        ctx,
		IncludeSystem:  request.IncludeSystem,
		From:           request.From,
		To:             request.To,
		MaxLLMMessages: request.MaxLLMMessages,
		AnalysisMode:   request.AnalysisMode,
		Progress: func(stage string, current int, total int) {
			h.updateJobProgress(jobID, stage, current, total)
		},
	})
	if err != nil {
		h.updateJob(jobID, JobStatusFailed, "执行失败", 1, 0, func(job *AnalysisJob) {
			job.Error = err.Error()
		})
		return
	}
	h.updateJob(jobID, JobStatusCompleted, "分析完成", 1, result.Record.MessageCount, func(job *AnalysisJob) {
		job.ResultRecordID = result.Record.ID
		job.Result = &result.Record
		job.ProcessedCount = job.LLMMessageCount
	})
}

func (h *Handler) updateJobProgress(jobID string, stage string, current int, total int) {
	stageText := stageLabel(stage)
	progress := stageProgress(stage, current, total)
	h.updateJob(jobID, JobStatusRunning, stageText, progress, current, func(job *AnalysisJob) {
		if stage == "message_parse" && total > 0 {
			job.MessageCount = total
			job.PreviewTotal = total
		}
	})
}

func (h *Handler) updateJob(jobID string, status JobStatus, stage string, progress float64, processed int, mutate func(job *AnalysisJob)) {
	h.mu.Lock()
	job, ok := h.jobs[jobID]
	if !ok {
		h.mu.Unlock()
		return
	}
	job.Status = status
	job.Stage = stage
	if progress > job.Progress {
		job.Progress = progress
	}
	if processed > job.ProcessedCount {
		job.ProcessedCount = processed
	}
	job.UpdatedAt = time.Now()
	if mutate != nil {
		mutate(job)
	}
	if len(job.Events) == 0 || job.Events[len(job.Events)-1].Message != stage {
		job.Events = append(job.Events, JobEvent{Time: time.Now(), Message: stage})
		if len(job.Events) > 40 {
			job.Events = job.Events[len(job.Events)-40:]
		}
	}
	copy := cloneJob(job)
	h.mu.Unlock()
	if h.store != nil {
		_ = h.store.UpdateJob(context.Background(), &copy)
		_ = h.store.AddJobEvent(context.Background(), jobID, stage)
	}
}

func (h *Handler) updateJobUsage(jobID string, event llm.UsageEvent) {
	h.updateJob(jobID, JobStatusRunning, stageLabel(event.SchemaName), 0, 0, func(job *AnalysisJob) {
		job.PromptTokens += event.PromptTokens
		job.CompletionTokens += event.CompletionTokens
		job.TotalTokens += event.TotalTokens
	})
}

func (h *Handler) runWorkItemWorker() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		h.processWorkItemsBurst(1)
	}
}

func (h *Handler) processWorkItemsBurst(limit int) {
	if h.store == nil || limit <= 0 {
		return
	}
	for i := 0; i < limit; i++ {
		if ok := h.processOneWorkItem(context.Background()); !ok {
			return
		}
	}
}

func (h *Handler) processOneWorkItem(ctx context.Context) bool {
	item, ok, err := h.store.ClaimNextWorkItem(ctx)
	if err != nil || !ok {
		return false
	}
	switch item.Kind {
	case "word_cloud":
		return h.processWordCloudWorkItem(ctx, item)
	case "topic_summary":
		return h.processTopicSummaryWorkItem(ctx, item)
	case "summary_merge":
		return h.processSummaryMergeWorkItem(ctx, item)
	default:
		_ = h.store.FailWorkItem(ctx, item.ID, fmt.Errorf("unsupported work item kind %q", item.Kind))
		return true
	}
}

func (h *Handler) processWordCloudWorkItem(ctx context.Context, item AnalysisWorkItem) bool {
	messages, err := h.store.jobMessagesInRange(ctx, item.JobID, &item.StartTime, &item.EndTime)
	if err != nil {
		_ = h.store.FailWorkItem(ctx, item.ID, err)
		return true
	}
	if err := h.store.CompleteWorkItem(ctx, item.ID, service.BuildWordCloud(messages, 30)); err != nil {
		_ = h.store.FailWorkItem(ctx, item.ID, err)
	}
	return true
}

func (h *Handler) processSummaryMergeWorkItem(ctx context.Context, item AnalysisWorkItem) bool {
	merger, ok := h.svc.Extractor().(llm.TopicSummaryMerger)
	if !ok || merger == nil {
		_ = h.store.FailWorkItem(ctx, item.ID, fmt.Errorf("summary merge requires llm merger"))
		return true
	}
	summaryItems, err := h.store.CompletedTopicSummariesInRange(ctx, item.JobID, item.Granularity, item.StartTime, item.EndTime, 80)
	if err != nil {
		_ = h.store.FailWorkItem(ctx, item.ID, err)
		return true
	}
	summaries := make([]llm.TopicSummary, 0, len(summaryItems))
	for _, summaryItem := range summaryItems {
		var summary llm.TopicSummary
		if err := json.Unmarshal(summaryItem.Result, &summary); err == nil && summary.Summary != "" {
			summaries = append(summaries, summary)
		}
	}
	if len(summaries) == 0 {
		_ = h.store.FailWorkItem(ctx, item.ID, fmt.Errorf("no completed topic summaries in range"))
		return true
	}
	relationshipID := item.JobID
	if job, err := h.store.GetJob(ctx, item.JobID); err == nil && job.RelationshipID != "" {
		relationshipID = job.RelationshipID
	}
	var usage llm.UsageEvent
	mergeCtx := llm.WithUsageReporter(ctx, func(event llm.UsageEvent) {
		usage.PromptTokens += event.PromptTokens
		usage.CompletionTokens += event.CompletionTokens
		usage.TotalTokens += event.TotalTokens
	})
	merged, err := merger.MergeTopicSummaries(mergeCtx, llm.TopicSummaryMergeInput{
		RelationshipID: relationshipID,
		ScopeID:        item.ScopeID,
		Granularity:    item.Granularity,
		StartTime:      item.StartTime.Format(time.RFC3339),
		EndTime:        item.EndTime.Format(time.RFC3339),
		Summaries:      summaries,
	})
	if err != nil {
		_ = h.store.FailWorkItem(ctx, item.ID, err)
		return true
	}
	if err := h.store.CompleteWorkItemWithUsage(ctx, item.ID, merged, usage); err != nil {
		_ = h.store.FailWorkItem(ctx, item.ID, err)
		return true
	}
	h.indexWorkItemSummary(ctx, item, relationshipID, merged, usage)
	return true
}

func (h *Handler) processTopicSummaryWorkItem(ctx context.Context, item AnalysisWorkItem) bool {
	summarizer, ok := h.svc.Extractor().(llm.TopicSummarizer)
	if !ok || summarizer == nil {
		_ = h.store.FailWorkItem(ctx, item.ID, fmt.Errorf("topic summary requires llm summarizer"))
		return true
	}
	messages, err := h.store.jobMessagesInRange(ctx, item.JobID, &item.StartTime, &item.EndTime)
	if err != nil {
		_ = h.store.FailWorkItem(ctx, item.ID, err)
		return true
	}
	if len(messages) > 200 {
		messages = messages[len(messages)-200:]
	}
	relationshipID := item.JobID
	if job, err := h.store.GetJob(ctx, item.JobID); err == nil && job.RelationshipID != "" {
		relationshipID = job.RelationshipID
	}
	var usage llm.UsageEvent
	summaryCtx := llm.WithUsageReporter(ctx, func(event llm.UsageEvent) {
		usage.PromptTokens += event.PromptTokens
		usage.CompletionTokens += event.CompletionTokens
		usage.TotalTokens += event.TotalTokens
	})
	summary, err := summarizer.SummarizeTopic(summaryCtx, llm.TopicSummaryInput{
		RelationshipID: relationshipID,
		ScopeID:        item.ScopeID,
		Granularity:    item.Granularity,
		StartTime:      item.StartTime.Format(time.RFC3339),
		EndTime:        item.EndTime.Format(time.RFC3339),
		Messages:       messages,
	})
	if err != nil {
		_ = h.store.FailWorkItem(ctx, item.ID, err)
		return true
	}
	if err := h.store.CompleteWorkItemWithUsage(ctx, item.ID, summary, usage); err != nil {
		_ = h.store.FailWorkItem(ctx, item.ID, err)
		return true
	}
	h.indexWorkItemSummary(ctx, item, relationshipID, summary, usage)
	return true
}

func (h *Handler) indexWorkItemSummary(ctx context.Context, item AnalysisWorkItem, relationshipID string, result interface{}, usage llm.UsageEvent) {
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	if err := h.indexer.IndexSummary(ctx, search.SummaryDocument{
		ID:               item.ID,
		JobID:            item.JobID,
		RelationshipID:   relationshipID,
		Kind:             item.Kind,
		ScopeType:        item.ScopeType,
		ScopeID:          item.ScopeID,
		Granularity:      item.Granularity,
		StartTime:        item.StartTime,
		EndTime:          item.EndTime,
		Status:           "completed",
		Result:           json.RawMessage(data),
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}); err != nil && h.store != nil {
		_ = h.store.AddJobEvent(ctx, item.JobID, "OpenSearch 摘要索引失败，已保留 Postgres 结果："+err.Error())
	}
}

func (h *Handler) handleBranchPreview(w http.ResponseWriter, r *http.Request, jobID string, fromStore bool) {
	if h.store == nil || !fromStore {
		writeError(w, http.StatusBadRequest, fmt.Errorf("branch preview store unavailable"))
		return
	}
	messages, err := h.store.jobMessages(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	h.handleBranchPreviewFromMessages(w, r, messages, jobID)
}

func (h *Handler) handleBranchPreviewFromMessages(w http.ResponseWriter, r *http.Request, messages []model.Message, _ string) {
	var request struct {
		Granularity string    `json:"granularity"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid branch preview payload"))
		return
	}
	preview, err := service.PreviewBranch(messages, request.Granularity, request.StartTime, request.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *Handler) handleCreateBranch(w http.ResponseWriter, r *http.Request, jobID string, fromStore bool) {
	if h.store == nil || !fromStore {
		writeError(w, http.StatusBadRequest, fmt.Errorf("branch create store unavailable"))
		return
	}
	job, err := h.store.GetJob(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	messages, err := h.store.jobMessages(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	h.handleCreateBranchFromMessages(w, r, messages, jobID, job.RelationshipID)
}

func (h *Handler) handleCreateBranchFromMessages(w http.ResponseWriter, r *http.Request, messages []model.Message, jobID string, relationshipID string) {
	var request struct {
		Granularity string    `json:"granularity"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		Title       string    `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid branch payload"))
		return
	}
	preview, err := service.PreviewBranch(messages, request.Granularity, request.StartTime, request.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	now := time.Now()
	branch := &AnalysisBranch{
		ID:             "br_" + newSessionToken()[:16],
		JobID:          jobID,
		RelationshipID: relationshipID,
		Title:          strings.TrimSpace(request.Title),
		Granularity:    preview.Granularity,
		StartTime:      preview.StartTime,
		EndTime:        preview.EndTime,
		MessageCount:   preview.MessageCount,
		BucketIDs:      preview.BucketIDs,
		ClusterID:      preview.ClusterID,
		TopicHint:      preview.TopicHint,
		Status:         "ready",
		Stage:          "等待分析",
		Progress:       0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if branch.Title == "" {
		branch.Title = preview.TopicHint
		if branch.Title == "" {
			branch.Title = "未命名片段"
		}
	}
	if h.store != nil {
		if err := h.store.CreateBranch(r.Context(), branch); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		h.mu.Lock()
		for _, existing := range h.branches {
			if sameBranchWindow(existing, branch) {
				branch = existing
				break
			}
		}
		h.branches[branch.ID] = branch
		h.mu.Unlock()
	}
	writeJSON(w, http.StatusCreated, branch)
}

func sameBranchWindow(a *AnalysisBranch, b *AnalysisBranch) bool {
	if a == nil || b == nil {
		return false
	}
	return a.JobID == b.JobID &&
		a.Granularity == b.Granularity &&
		a.StartTime.Equal(b.StartTime) &&
		a.EndTime.Equal(b.EndTime)
}

func (h *Handler) handleListBranches(w http.ResponseWriter, r *http.Request, jobID string, fromStore bool) {
	if h.store == nil || !fromStore {
		writeError(w, http.StatusBadRequest, fmt.Errorf("branch list store unavailable"))
		return
	}
	items, err := h.store.ListBranches(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (h *Handler) listBranchesMemory(jobID string) []AnalysisBranch {
	items := make([]AnalysisBranch, 0)
	for _, branch := range h.branches {
		if branch.JobID == jobID {
			items = append(items, *branch)
		}
	}
	return items
}

func (h *Handler) handleSeedWorkItems(w http.ResponseWriter, r *http.Request, jobID string) {
	if h.store == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("work item store unavailable"))
		return
	}
	var request struct {
		Kind        string `json:"kind"`
		Granularity string `json:"granularity"`
		StartTime   string `json:"start_time"`
		EndTime     string `json:"end_time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid work item seed payload"))
		return
	}
	granularity := request.Granularity
	if granularity == "" {
		granularity = "day"
	}
	kind := request.Kind
	if kind == "" {
		kind = "word_cloud"
	}
	if kind != "word_cloud" && kind != "topic_summary" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported work item kind %q", kind))
		return
	}
	start, end, err := parseOptionalRFC3339Range(request.StartTime, request.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := h.store.SeedWorkItems(r.Context(), jobID, kind, granularity, start, end)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	go h.processWorkItemsBurst(3)
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"items": items})
}

func (h *Handler) handleListWorkItems(w http.ResponseWriter, r *http.Request, jobID string) {
	if h.store == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("work item store unavailable"))
		return
	}
	items, err := h.store.ListWorkItems(r.Context(), jobID, r.URL.Query().Get("kind"), r.URL.Query().Get("granularity"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (h *Handler) handleSearchMessages(w http.ResponseWriter, r *http.Request, jobID string, fromStore bool) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	start, end, err := parseTimelineRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.indexer.SearchMessages(r.Context(), search.MessageSearchRequest{
		JobID:    jobID,
		Query:    query,
		Start:    start,
		End:      end,
		Page:     page,
		PageSize: pageSize,
	})
	if err == nil {
		writeJSON(w, http.StatusOK, searchResultToMessagePage(result))
		return
	}
	if h.store == nil || !fromStore {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	messages, loadErr := h.store.jobMessagesInRange(r.Context(), jobID, start, end)
	if loadErr != nil {
		writeError(w, http.StatusInternalServerError, loadErr)
		return
	}
	writeJSON(w, http.StatusOK, messageSearchPage(filterMessagesByQuery(messages, query), page, pageSize, "postgres"))
}

func (h *Handler) handleSearchMessagesFromMessages(w http.ResponseWriter, r *http.Request, jobID string, messages []model.Message) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	start, end, err := parseTimelineRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	messages = service.FilterMessagesByTimeRange(messages, start, end)
	result, err := h.indexer.SearchMessages(r.Context(), search.MessageSearchRequest{
		JobID:    jobID,
		Query:    query,
		Start:    start,
		End:      end,
		Page:     page,
		PageSize: pageSize,
	})
	if err == nil {
		writeJSON(w, http.StatusOK, searchResultToMessagePage(result))
		return
	}
	writeJSON(w, http.StatusOK, messageSearchPage(filterMessagesByQuery(messages, query), page, pageSize, "memory"))
}

func searchResultToMessagePage(result search.MessageSearchResult) MessageSearchPage {
	items := make([]MessagePreview, 0, len(result.Items))
	for _, doc := range result.Items {
		content := doc.Content
		if len([]rune(content)) > 160 {
			content = string([]rune(content)[:160]) + "..."
		}
		sender := doc.DisplaySender
		if sender == "" {
			sender = doc.Sender
		}
		items = append(items, MessagePreview{
			ID:      doc.MessageID,
			Sender:  sender,
			Time:    doc.MessageTime.Format("2006-01-02 15:04:05"),
			Type:    doc.MessageType,
			Content: content,
		})
	}
	return MessageSearchPage{
		Items:      items,
		Total:      result.Total,
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.TotalPages,
		Source:     result.Source,
	}
}

func filterMessagesByQuery(messages []model.Message, query string) []model.Message {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return messages
	}
	filtered := make([]model.Message, 0)
	for _, message := range messages {
		if strings.Contains(strings.ToLower(message.Content), query) ||
			strings.Contains(strings.ToLower(message.DisplaySender()), query) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func (h *Handler) handleCreateSummaryMergeWorkItem(w http.ResponseWriter, r *http.Request, jobID string) {
	if h.store == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("work item store unavailable"))
		return
	}
	var request struct {
		Granularity string `json:"granularity"`
		StartTime   string `json:"start_time"`
		EndTime     string `json:"end_time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid summary merge payload"))
		return
	}
	if request.Granularity == "" {
		request.Granularity = "day"
	}
	start, err := time.Parse(time.RFC3339, request.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid start_time"))
		return
	}
	end, err := time.Parse(time.RFC3339, request.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid end_time"))
		return
	}
	if !end.After(start) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("end_time must be after start_time"))
		return
	}
	item, err := h.store.CreateMergeWorkItem(r.Context(), jobID, request.Granularity, start, end, 30)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	go h.processWorkItemsBurst(1)
	writeJSON(w, http.StatusAccepted, item)
}

func parseOptionalRFC3339Range(startText string, endText string) (*time.Time, *time.Time, error) {
	startText = strings.TrimSpace(startText)
	endText = strings.TrimSpace(endText)
	if startText == "" && endText == "" {
		return nil, nil, nil
	}
	if startText == "" || endText == "" {
		return nil, nil, fmt.Errorf("start_time and end_time must be provided together")
	}
	start, err := time.Parse(time.RFC3339, startText)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid start_time")
	}
	end, err := time.Parse(time.RFC3339, endText)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid end_time")
	}
	if !end.After(start) {
		return nil, nil, fmt.Errorf("end_time must be after start_time")
	}
	return &start, &end, nil
}

func (h *Handler) handlePrioritizeWorkItem(w http.ResponseWriter, r *http.Request, itemID string) {
	if h.store == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("work item store unavailable"))
		return
	}
	item, err := h.store.PrioritizeWorkItem(r.Context(), itemID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	go h.processWorkItemsBurst(1)
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) handleRunBranch(w http.ResponseWriter, r *http.Request, jobID string, branchID string, fromStore bool) {
	if h.store == nil || !fromStore {
		writeError(w, http.StatusBadRequest, fmt.Errorf("branch run store unavailable"))
		return
	}
	branch, err := h.store.GetBranch(r.Context(), branchID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	messages, err := h.store.jobMessages(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var request struct {
		AnalysisMode   string `json:"analysis_mode"`
		MaxLLMMessages int    `json:"max_llm_messages"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid branch run payload"))
			return
		}
	}
	if err := h.startBranchRun(&branch, messages, request.AnalysisMode, request.MaxLLMMessages, true); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, branch)
}

func (h *Handler) handleRunBranchMemory(w http.ResponseWriter, r *http.Request, jobID string, branchID string) {
	h.mu.Lock()
	branch, ok := h.branches[branchID]
	job, jobOK := h.jobs[jobID]
	var messages []model.Message
	if jobOK {
		messages = append([]model.Message(nil), job.messages...)
	}
	h.mu.Unlock()
	if !ok || !jobOK {
		writeError(w, http.StatusNotFound, fmt.Errorf("branch not found"))
		return
	}
	var request struct {
		AnalysisMode   string `json:"analysis_mode"`
		MaxLLMMessages int    `json:"max_llm_messages"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid branch run payload"))
			return
		}
	}
	if err := h.startBranchRun(branch, messages, request.AnalysisMode, request.MaxLLMMessages, false); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, branch)
}

func (h *Handler) startBranchRun(branch *AnalysisBranch, messages []model.Message, analysisMode string, maxLLMMessages int, persist bool) error {
	if analysisMode == "" {
		analysisMode = "quick"
	}
	branch.Status = "running"
	branch.Stage = "开始分析片段"
	branch.Progress = 0.1
	branch.Error = ""
	branch.UpdatedAt = time.Now()
	if persist && h.store != nil {
		if err := h.store.UpdateBranch(context.Background(), branch); err != nil {
			return err
		}
	}
	branchMessages := filterMessagesByWindow(messages, branch.StartTime, branch.EndTime)
	go h.runBranch(branch.ID, branchMessages, analysisMode, maxLLMMessages, persist)
	return nil
}

func (h *Handler) runBranch(branchID string, messages []model.Message, mode string, maxLLMMessages int, persist bool) {
	h.updateBranch(branchID, "running", "分析片段消息", 0.2, func(branch *AnalysisBranch) {})
	ctx := llm.WithUsageReporter(context.Background(), func(event llm.UsageEvent) {
		h.updateBranch(branchID, "running", stageLabel(event.SchemaName), 0, func(branch *AnalysisBranch) {
			branch.PromptTokens += event.PromptTokens
			branch.CompletionTokens += event.CompletionTokens
			branch.TotalTokens += event.TotalTokens
		})
	})
	result, err := analyzer.AnalyzeMessagesWithOptions(ctx, messages, branchRelationshipID(messages), analyzer.AnalyzeOptions{
		Extractor:      h.svcExtractor(),
		MaxLLMMessages: maxLLMMessages,
		Mode:           mode,
		Progress: func(stage string, current int, total int) {
			h.updateBranch(branchID, "running", stageLabel(stage), stageProgress(stage, current, total), nil)
		},
	})
	if err != nil {
		h.updateBranch(branchID, "failed", "执行失败", 1, func(branch *AnalysisBranch) {
			branch.Error = err.Error()
		})
		return
	}
	h.updateBranch(branchID, "completed", "片段分析完成", 1, func(branch *AnalysisBranch) {
		branch.ReportMarkdown = result.Report.Markdown
		branch.ModelName = result.Report.ModelName
		branch.Stage = "片段分析完成"
	})
	h.indexBranchReport(context.Background(), branchID)
	_ = persist
}

func (h *Handler) indexBranchReport(ctx context.Context, branchID string) {
	var branch AnalysisBranch
	if h.store != nil {
		loaded, err := h.store.GetBranch(ctx, branchID)
		if err != nil {
			return
		}
		branch = loaded
	} else {
		h.mu.Lock()
		if h.branches[branchID] != nil {
			branch = *h.branches[branchID]
		}
		h.mu.Unlock()
	}
	if branch.ID == "" {
		return
	}
	if err := h.indexer.IndexBranch(ctx, search.BranchDocument{
		ID:               branch.ID,
		JobID:            branch.JobID,
		RelationshipID:   branch.RelationshipID,
		Title:            branch.Title,
		Granularity:      branch.Granularity,
		StartTime:        branch.StartTime,
		EndTime:          branch.EndTime,
		MessageCount:     branch.MessageCount,
		TopicHint:        branch.TopicHint,
		Status:           branch.Status,
		Stage:            branch.Stage,
		ReportMarkdown:   branch.ReportMarkdown,
		ModelName:        branch.ModelName,
		PromptTokens:     branch.PromptTokens,
		CompletionTokens: branch.CompletionTokens,
		TotalTokens:      branch.TotalTokens,
	}); err != nil && h.store != nil {
		_ = h.store.AddJobEvent(ctx, branch.JobID, "OpenSearch Branch 索引失败，已保留 Postgres 结果："+err.Error())
	}
}

func (h *Handler) updateBranch(branchID string, status string, stage string, progress float64, mutate func(branch *AnalysisBranch)) {
	h.mu.Lock()
	branch, ok := h.branches[branchID]
	if !ok && h.store == nil {
		h.mu.Unlock()
		return
	}
	if ok {
		branch.Status = status
		if stage != "" {
			branch.Stage = stage
		}
		if progress > branch.Progress {
			branch.Progress = progress
		}
		branch.UpdatedAt = time.Now()
		if mutate != nil {
			mutate(branch)
		}
		clone := *branch
		h.mu.Unlock()
		if h.store != nil {
			_ = h.store.UpdateBranch(context.Background(), &clone)
		}
		return
	}
	h.mu.Unlock()
	if h.store != nil {
		branch, err := h.store.GetBranch(context.Background(), branchID)
		if err != nil {
			return
		}
		branch.Status = status
		if stage != "" {
			branch.Stage = stage
		}
		if progress > branch.Progress {
			branch.Progress = progress
		}
		branch.UpdatedAt = time.Now()
		if mutate != nil {
			mutate(&branch)
		}
		_ = h.store.UpdateBranch(context.Background(), &branch)
	}
}

func filterMessagesByWindow(messages []model.Message, start time.Time, end time.Time) []model.Message {
	filtered := make([]model.Message, 0, len(messages))
	for _, message := range messages {
		if !message.MsgTime.Before(start) && message.MsgTime.Before(end) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func branchRelationshipID(messages []model.Message) string {
	if len(messages) == 0 {
		return ""
	}
	return messages[0].RelationshipID
}

func (h *Handler) svcExtractor() llm.Extractor {
	return h.svc.Extractor()
}

func stageLabel(stage string) string {
	switch stage {
	case "object_store_upload":
		return "上传原始文件到对象存储"
	case "message_parse":
		return "解析聊天记录"
	case "llm_action_extraction":
		return "LLM 抽取行为动作"
	case "action_extraction":
		return "LLM 抽取行为动作"
	case "action_batch_extraction":
		return "LLM 批量抽取行为动作"
	case "llm_event_extraction":
		return "LLM 抽取关系事件"
	case "event_extraction":
		return "LLM 抽取关系事件"
	case "llm_dimension_generation":
		return "LLM 生成关系维度"
	case "dimension_generation":
		return "LLM 生成关系维度"
	case "llm_report_generation":
		return "LLM 生成分析报告"
	case "report_generation":
		return "LLM 生成分析报告"
	case "llm_quick_report_generation":
		return "LLM 快速生成报告"
	case "analysis_without_llm":
		return "未启用 LLM，生成基础统计报告"
	case "report_persist":
		return "保存报告与历史记录"
	default:
		return stage
	}
}

func stageProgress(stage string, current int, total int) float64 {
	base := map[string]float64{
		"object_store_upload":         0.18,
		"message_parse":               0.28,
		"llm_action_extraction":       0.45,
		"llm_event_extraction":        0.65,
		"llm_dimension_generation":    0.78,
		"llm_report_generation":       0.9,
		"llm_quick_report_generation": 0.82,
		"analysis_without_llm":        0.85,
		"report_persist":              0.96,
	}[stage]
	if base == 0 {
		base = 0.35
	}
	if total > 0 && current > 0 {
		return base + 0.08*(float64(current)/float64(total))
	}
	return base
}

func parseAnalysisMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "quick":
		return "quick"
	default:
		return "full"
	}
}

func filterServerPeriod(messages []model.Message, from *time.Time, to *time.Time) []model.Message {
	if from == nil && to == nil {
		return messages
	}
	filtered := make([]model.Message, 0, len(messages))
	for _, message := range messages {
		if from != nil && message.MsgTime.Before(*from) {
			continue
		}
		if to != nil && !message.MsgTime.Before(*to) {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered
}

func (h *Handler) validSession(token string) (string, bool) {
	if h.store != nil {
		return h.store.ValidSession(token)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	expiresAt, ok := h.sessions[token]
	if !ok || time.Now().After(expiresAt) {
		delete(h.sessions, token)
		return "", false
	}
	return h.options.AuthUsername, true
}

func splitAnalysisPath(path string) (string, string) {
	rest := strings.TrimPrefix(path, "/api/analyses/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func splitJobPath(path string) (string, string) {
	rest := strings.TrimPrefix(path, "/api/jobs/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], "/")
}

func serviceGranularity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "year", "month", "week", "day", "15m", "5m":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "hour"
	}
}

func parseTimelineRange(r *http.Request) (*time.Time, *time.Time, error) {
	var start *time.Time
	var end *time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("start_time")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid start_time")
		}
		start = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("end_time")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid end_time")
		}
		end = &parsed
	}
	if start != nil && end != nil && !start.Before(*end) {
		return nil, nil, fmt.Errorf("start_time must be before end_time")
	}
	return start, end, nil
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseFormTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05"} {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("invalid time %q; use YYYY-MM-DD or YYYY-MM-DD HH:MM:SS", value)
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func newSessionToken() string {
	var data [24]byte
	if _, err := rand.Read(data[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(data[:])
}

type userContextKey struct{}

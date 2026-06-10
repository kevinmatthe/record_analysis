package analyzer

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/llm"
	"github.com/kevinmatthe/record_analysis/internal/model"
)

type AnalyzeOptions struct {
	Extractor       llm.Extractor
	MaxLLMMessages  int
	Mode            string
	ChunkLLMReports bool
	Progress        func(stage string, current int, total int)
}

func AnalyzeMessages(messages []model.Message, relationshipID string) (model.AnalysisResult, error) {
	return AnalyzeMessagesWithOptions(context.Background(), messages, relationshipID, AnalyzeOptions{})
}

func AnalyzeMessagesWithOptions(ctx context.Context, messages []model.Message, relationshipID string, opts AnalyzeOptions) (model.AnalysisResult, error) {
	clean := deduplicateMessages(filterParticipants(messages))
	if len(clean) == 0 {
		return model.AnalysisResult{}, errors.New("cannot analyze empty participant message set")
	}

	segments := segmentMessages(clean, relationshipID, 30*time.Minute)
	metrics := computeMetrics(clean, nil, nil, relationshipID)
	dimensions := model.PsychologicalDimensions{
		RelationshipID: relationshipID,
		PeriodStart:    metrics.PeriodStart,
		PeriodEnd:      metrics.PeriodEnd,
		Values:         map[string]interface{}{},
	}

	var actions []model.MessageAction
	var events []model.RelationshipEvent
	var report model.PeriodReport
	var err error

	if opts.Extractor != nil {
		llmMessages := capMessages(clean, opts.MaxLLMMessages)
		llmSegments := segmentMessages(llmMessages, relationshipID, 30*time.Minute)
		if opts.Mode == "quick" {
			if opts.ChunkLLMReports && shouldChunkReportMessages(llmMessages) {
				report, err = generateChunkedReport(ctx, opts.Extractor, relationshipID, llmMessages, metrics, dimensions, opts.Progress)
				if err != nil {
					return model.AnalysisResult{}, err
				}
				return model.AnalysisResult{
					RelationshipID: relationshipID,
					Messages:       clean,
					Segments:       segments,
					Actions:        nil,
					Events:         nil,
					Metrics:        metrics,
					Dimensions:     dimensions,
					Report:         report,
				}, nil
			}
			reportProgress(opts.Progress, "llm_quick_report_generation", 0, 1)
			report, err = opts.Extractor.GenerateReport(ctx, llm.ReportInput{
				RelationshipID: relationshipID,
				Messages:       llmMessages,
				Segments:       llmSegments,
				Metrics:        metrics,
				Dimensions:     dimensions,
			})
			if err != nil {
				return model.AnalysisResult{}, err
			}
			reportProgress(opts.Progress, "llm_quick_report_generation", 1, 1)
			return model.AnalysisResult{
				RelationshipID: relationshipID,
				Messages:       clean,
				Segments:       segments,
				Actions:        nil,
				Events:         nil,
				Metrics:        metrics,
				Dimensions:     dimensions,
				Report:         report,
			}, nil
		}
		reportProgress(opts.Progress, "llm_action_extraction", 0, len(llmMessages))
		if batchExtractor, ok := opts.Extractor.(llm.BatchActionExtractor); ok {
			actions, err = batchExtractor.ExtractActionsBySegment(ctx, llmMessages, llmSegments)
		} else {
			actions, err = opts.Extractor.ExtractActions(ctx, llmMessages, llmSegments)
		}
		if err != nil {
			return model.AnalysisResult{}, err
		}
		reportProgress(opts.Progress, "llm_action_extraction", len(llmMessages), len(llmMessages))
		reportProgress(opts.Progress, "llm_event_extraction", 0, len(llmSegments))
		events, err = opts.Extractor.ExtractEvents(ctx, llmMessages, llmSegments, actions)
		if err != nil {
			return model.AnalysisResult{}, err
		}
		reportProgress(opts.Progress, "llm_event_extraction", len(llmSegments), len(llmSegments))
		metrics = computeMetrics(clean, actions, events, relationshipID)
		reportProgress(opts.Progress, "llm_dimension_generation", 0, 1)
		dimensions, err = opts.Extractor.GenerateDimensions(ctx, metrics, events)
		if err != nil {
			return model.AnalysisResult{}, err
		}
		reportProgress(opts.Progress, "llm_dimension_generation", 1, 1)
		reportProgress(opts.Progress, "llm_report_generation", 0, 1)
		report, err = opts.Extractor.GenerateReport(ctx, llm.ReportInput{
			RelationshipID: relationshipID,
			Messages:       llmMessages,
			Segments:       llmSegments,
			Actions:        actions,
			Events:         events,
			Metrics:        metrics,
			Dimensions:     dimensions,
		})
		if err != nil {
			return model.AnalysisResult{}, err
		}
		reportProgress(opts.Progress, "llm_report_generation", 1, 1)
	} else {
		reportProgress(opts.Progress, "analysis_without_llm", len(clean), len(clean))
		report = generateDisabledReport(metrics, clean, relationshipID)
	}

	return model.AnalysisResult{
		RelationshipID: relationshipID,
		Messages:       clean,
		Segments:       segments,
		Actions:        actions,
		Events:         events,
		Metrics:        metrics,
		Dimensions:     dimensions,
		Report:         report,
	}, nil
}

const defaultReportChunkMessages = 180

func shouldChunkReportMessages(messages []model.Message) bool {
	return len(messages) > defaultReportChunkMessages
}

func generateChunkedReport(ctx context.Context, extractor llm.Extractor, relationshipID string, messages []model.Message, metrics model.BehaviorMetrics, dimensions model.PsychologicalDimensions, progress func(stage string, current int, total int)) (model.PeriodReport, error) {
	chunks := chunkMessagesByCount(messages, defaultReportChunkMessages)
	reports := make([]model.PeriodReport, 0, len(chunks))
	reportProgress(progress, "llm_branch_chunk_reports", 0, len(chunks))
	for index, chunk := range chunks {
		chunkMetrics := computeMetrics(chunk, nil, nil, relationshipID)
		chunkDimensions := model.PsychologicalDimensions{
			RelationshipID: relationshipID,
			PeriodStart:    chunkMetrics.PeriodStart,
			PeriodEnd:      chunkMetrics.PeriodEnd,
			Values:         dimensions.Values,
		}
		report, err := extractor.GenerateReport(ctx, llm.ReportInput{
			RelationshipID: relationshipID,
			Messages:       chunk,
			Segments:       segmentMessages(chunk, relationshipID, 30*time.Minute),
			Metrics:        chunkMetrics,
			Dimensions:     chunkDimensions,
		})
		if err != nil {
			return model.PeriodReport{}, err
		}
		if report.PeriodStart.IsZero() {
			report.PeriodStart = chunkMetrics.PeriodStart
		}
		if report.PeriodEnd.IsZero() {
			report.PeriodEnd = chunkMetrics.PeriodEnd
		}
		reports = append(reports, report)
		reportProgress(progress, "llm_branch_chunk_reports", index+1, len(chunks))
	}
	if merger, ok := extractor.(llm.ReportMerger); ok {
		reportProgress(progress, "llm_branch_report_merge", 0, 1)
		merged, err := merger.MergeReports(ctx, llm.ReportMergeInput{
			RelationshipID: relationshipID,
			PeriodStart:    metrics.PeriodStart.Format(time.RFC3339),
			PeriodEnd:      metrics.PeriodEnd.Format(time.RFC3339),
			Reports:        reports,
		})
		if err != nil {
			return model.PeriodReport{}, err
		}
		if merged.PeriodStart.IsZero() {
			merged.PeriodStart = metrics.PeriodStart
		}
		if merged.PeriodEnd.IsZero() {
			merged.PeriodEnd = metrics.PeriodEnd
		}
		reportProgress(progress, "llm_branch_report_merge", 1, 1)
		return merged, nil
	}
	return mergeReportsLocally(relationshipID, metrics, reports), nil
}

func chunkMessagesByCount(messages []model.Message, chunkSize int) [][]model.Message {
	if chunkSize <= 0 || len(messages) <= chunkSize {
		return [][]model.Message{messages}
	}
	chunks := make([][]model.Message, 0, (len(messages)+chunkSize-1)/chunkSize)
	for start := 0; start < len(messages); start += chunkSize {
		end := start + chunkSize
		if end > len(messages) {
			end = len(messages)
		}
		chunks = append(chunks, messages[start:end])
	}
	return chunks
}

func mergeReportsLocally(relationshipID string, metrics model.BehaviorMetrics, reports []model.PeriodReport) model.PeriodReport {
	var builder strings.Builder
	builder.WriteString("# 分段关系互动报告汇总\n\n")
	builder.WriteString(fmt.Sprintf("本片段消息量较大，已拆分为 %d 个窗口分别分析。以下为分段结果汇总，建议继续使用聚合模型生成更精炼的总报告。\n", len(reports)))
	for index, report := range reports {
		builder.WriteString(fmt.Sprintf("\n## 分段 %d：%s - %s\n\n", index+1, report.PeriodStart.Format("2006-01-02 15:04"), report.PeriodEnd.Format("2006-01-02 15:04")))
		builder.WriteString(strings.TrimSpace(report.Markdown))
		builder.WriteString("\n")
	}
	return model.PeriodReport{
		RelationshipID:   relationshipID,
		PeriodType:       "branch_chunked",
		PeriodStart:      metrics.PeriodStart,
		PeriodEnd:        metrics.PeriodEnd,
		Markdown:         builder.String(),
		EvidenceEventIDs: uniqueEventIDs(reports),
		ModelName:        firstReportModelName(reports),
	}
}

func uniqueEventIDs(reports []model.PeriodReport) []string {
	seen := map[string]bool{}
	var ids []string
	for _, report := range reports {
		for _, id := range report.EvidenceEventIDs {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func firstReportModelName(reports []model.PeriodReport) string {
	for _, report := range reports {
		if report.ModelName != "" {
			return report.ModelName
		}
	}
	return ""
}

func reportProgress(progress func(stage string, current int, total int), stage string, current int, total int) {
	if progress != nil {
		progress(stage, current, total)
	}
}

func capMessages(messages []model.Message, max int) []model.Message {
	if max <= 0 || len(messages) <= max {
		return messages
	}
	return messages[len(messages)-max:]
}

func filterParticipants(messages []model.Message) []model.Message {
	filtered := make([]model.Message, 0, len(messages))
	for _, message := range messages {
		if message.Sender == "PERSON_A" || message.Sender == "PERSON_B" {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func deduplicateMessages(messages []model.Message) []model.Message {
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].MsgTime.Before(messages[j].MsgTime)
	})
	seen := map[string]bool{}
	deduped := make([]model.Message, 0, len(messages))
	for _, message := range messages {
		key := message.ContentHash
		if key == "" {
			key = fmt.Sprintf("%s|%s|%s", message.Sender, message.MsgTime.Format(time.RFC3339), message.Content)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, message)
	}
	return deduped
}

func segmentMessages(messages []model.Message, relationshipID string, gap time.Duration) []model.Segment {
	if len(messages) == 0 {
		return nil
	}
	var segments []model.Segment
	start := 0
	for i := 1; i < len(messages); i++ {
		prev := messages[i-1]
		cur := messages[i]
		if !sameDay(prev.MsgTime, cur.MsgTime) || cur.MsgTime.Sub(prev.MsgTime) > gap {
			segments = append(segments, makeSegment(len(segments)+1, relationshipID, messages[start:i]))
			start = i
		}
	}
	segments = append(segments, makeSegment(len(segments)+1, relationshipID, messages[start:]))
	return segments
}

func makeSegment(index int, relationshipID string, messages []model.Message) model.Segment {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	return model.Segment{
		ID:             fmt.Sprintf("SEG_%06d", index),
		RelationshipID: relationshipID,
		StartTime:      messages[0].MsgTime,
		EndTime:        messages[len(messages)-1].MsgTime,
		MessageIDs:     ids,
		SegmentType:    "conversation",
	}
}

func computeMetrics(messages []model.Message, actions []model.MessageAction, events []model.RelationshipEvent, relationshipID string) model.BehaviorMetrics {
	senderCount := map[string]int{}
	for _, message := range messages {
		senderCount[message.Sender]++
	}
	actionCount := map[string]int{}
	for _, action := range actions {
		actionCount[action.ActionType]++
	}
	eventCount := map[string]int{}
	for _, event := range events {
		eventCount[event.EventType]++
	}
	values := map[string]interface{}{
		"message_volume":                 len(messages),
		"person_a_message_ratio":         round(float64(senderCount["PERSON_A"]) / float64(len(messages))),
		"person_b_message_ratio":         round(float64(senderCount["PERSON_B"]) / float64(len(messages))),
		"initiation_rate":                initiationRate(messages),
		"avg_reply_latency_minutes":      avgReplyLatency(messages),
		"affection_expression_count":     actionCount["affection"],
		"vulnerability_expression_count": actionCount["express_need"],
		"conflict_count":                 eventCount["conflict"],
		"repair_attempt_count":           actionCount["repair_attempt"],
		"repair_success_rate":            repairSuccessRate(events),
		"withdrawal_count":               actionCount["withdraw"],
		"long_silence_after_conflict":    eventCount["withdrawal"],
	}
	return model.BehaviorMetrics{
		RelationshipID: relationshipID,
		PeriodStart:    messages[0].MsgTime,
		PeriodEnd:      messages[len(messages)-1].MsgTime,
		Values:         values,
	}
}

func generateDisabledReport(metrics model.BehaviorMetrics, messages []model.Message, relationshipID string) model.PeriodReport {
	markdown := fmt.Sprintf(`# 本周期关系互动报告

## 1. 状态

本周期共导入 %d 条参与者消息。当前未配置结构化 LLM extractor，因此没有生成 message_actions、relationship_events、心理/关系维度或叙事结论。

## 2. 可计算指标

- 时间范围：%s ~ %s
- 发起比例：%s
- 平均回复延迟：%s 分钟

## 3. 原始样本

%s

## 4. 下一步

请配置 LLM extractor 后运行完整流水线：消息级行为识别 -> 片段级事件抽取 -> 维度生成 -> 周期报告。未配置模型时，系统不会使用关键词规则替代分析。

## 5. 不确定性

以上内容只是导入和统计结果，不构成关系分析。
`,
		metricInt(metrics.Values["message_volume"]),
		metrics.PeriodStart.Format("2006-01-02 15:04"),
		metrics.PeriodEnd.Format("2006-01-02 15:04"),
		formatFloatMap(metrics.Values["initiation_rate"]),
		formatFloatMap(metrics.Values["avg_reply_latency_minutes"]),
		formatMessageSamples(messages, 5),
	)
	return model.PeriodReport{
		RelationshipID:   relationshipID,
		PeriodType:       "raw_import",
		PeriodStart:      metrics.PeriodStart,
		PeriodEnd:        metrics.PeriodEnd,
		Markdown:         markdown,
		EvidenceEventIDs: nil,
		ModelName:        "llm_not_configured",
	}
}

func formatMessageSamples(messages []model.Message, limit int) string {
	if len(messages) < limit {
		limit = len(messages)
	}
	lines := make([]string, 0, limit)
	for _, message := range messages[:limit] {
		lines = append(lines, fmt.Sprintf("- %s %s：%s", message.MsgTime.Format("2006-01-02 15:04"), message.Sender, truncate(message.Content, 80)))
	}
	return strings.Join(lines, "\n")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func avgReplyLatency(messages []model.Message) map[string]float64 {
	totals := map[string][]float64{}
	for i := 1; i < len(messages); i++ {
		prev := messages[i-1]
		cur := messages[i]
		if prev.Sender == cur.Sender {
			continue
		}
		minutes := cur.MsgTime.Sub(prev.MsgTime).Minutes()
		totals[cur.Sender] = append(totals[cur.Sender], minutes)
		totals["overall"] = append(totals["overall"], minutes)
	}
	result := map[string]float64{}
	for sender, values := range totals {
		var sum float64
		for _, value := range values {
			sum += value
		}
		result[sender] = round(sum / float64(len(values)))
	}
	return result
}

func initiationRate(messages []model.Message) map[string]float64 {
	starts := map[string]int{messages[0].Sender: 1}
	for i := 1; i < len(messages); i++ {
		if messages[i].MsgTime.Sub(messages[i-1].MsgTime) > 30*time.Minute {
			starts[messages[i].Sender]++
		}
	}
	total := starts["PERSON_A"] + starts["PERSON_B"]
	if total == 0 {
		total = 1
	}
	return map[string]float64{
		"PERSON_A": round(float64(starts["PERSON_A"]) / float64(total)),
		"PERSON_B": round(float64(starts["PERSON_B"]) / float64(total)),
	}
}

func repairSuccessRate(events []model.RelationshipEvent) float64 {
	var repairable, repaired int
	for _, event := range events {
		if event.EventType != "conflict" && event.EventType != "repair" {
			continue
		}
		repairable++
		if event.RepairStatus == "partial" {
			repaired++
		}
	}
	if repairable == 0 {
		return 0
	}
	return round(float64(repaired) / float64(repairable))
}

func formatFloatMap(value interface{}) string {
	m, ok := value.(map[string]float64)
	if !ok || len(m) == 0 {
		return "unknown"
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%.3f", key, m[key]))
	}
	return strings.Join(parts, ", ")
}

func metricInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}

func round(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func truncate(value string, limit int) string {
	if len([]rune(value)) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}

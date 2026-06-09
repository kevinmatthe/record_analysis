package llm

import (
	"testing"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

func taskMsg(id int, sender string, minute int, content string) model.Message {
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

func TestBuildActionTasksIncludesBoundedContext(t *testing.T) {
	messages := []model.Message{
		taskMsg(1, "PERSON_A", 0, "第一句"),
		taskMsg(2, "PERSON_B", 1, "第二句"),
		taskMsg(3, "PERSON_A", 2, "第三句"),
	}

	tasks := BuildActionTasks(messages, "rel_test", 1)

	if len(tasks) != 3 {
		t.Fatalf("tasks = %d, want 3", len(tasks))
	}
	if tasks[1].Message.ID != "MSG_000002" {
		t.Fatalf("middle task message id = %s", tasks[1].Message.ID)
	}
	if len(tasks[1].ContextBefore) != 1 || tasks[1].ContextBefore[0].ID != "MSG_000001" {
		t.Fatalf("unexpected context_before: %+v", tasks[1].ContextBefore)
	}
	if len(tasks[1].ContextAfter) != 1 || tasks[1].ContextAfter[0].ID != "MSG_000003" {
		t.Fatalf("unexpected context_after: %+v", tasks[1].ContextAfter)
	}
}

func TestBuildActionBatchTasksScopesMessagesToSegment(t *testing.T) {
	messages := []model.Message{
		taskMsg(1, "PERSON_A", 0, "第一句"),
		taskMsg(2, "PERSON_B", 1, "第二句"),
		taskMsg(3, "PERSON_A", 50, "第三句"),
	}
	segments := []model.Segment{
		{
			ID:             "SEG_000001",
			RelationshipID: "rel_test",
			MessageIDs:     []string{messages[0].ID, messages[1].ID},
		},
		{
			ID:             "SEG_000002",
			RelationshipID: "rel_test",
			MessageIDs:     []string{messages[2].ID},
		},
	}

	tasks := BuildActionBatchTasks(messages, segments)

	if len(tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(tasks))
	}
	if tasks[0].Segment.ID != "SEG_000001" || len(tasks[0].Messages) != 2 {
		t.Fatalf("first task = %+v", tasks[0])
	}
	if tasks[1].Messages[0].ID != "MSG_000003" {
		t.Fatalf("second task messages = %+v", tasks[1].Messages)
	}
}

func TestBuildEventTaskScopesMessagesAndActionsToSegment(t *testing.T) {
	messages := []model.Message{
		taskMsg(1, "PERSON_A", 0, "第一句"),
		taskMsg(2, "PERSON_B", 1, "第二句"),
		taskMsg(3, "PERSON_A", 50, "第三句"),
	}
	segment := model.Segment{
		ID:             "SEG_000001",
		RelationshipID: "rel_test",
		StartTime:      messages[0].MsgTime,
		EndTime:        messages[1].MsgTime,
		MessageIDs:     []string{messages[0].ID, messages[1].ID},
	}
	actions := []model.MessageAction{
		{ID: "ACT_000001", SegmentID: "SEG_000001", MsgID: messages[0].ID},
		{ID: "ACT_000002", SegmentID: "SEG_000002", MsgID: messages[2].ID},
	}

	task := BuildEventTask(segment, messages, actions)

	if len(task.Messages) != 2 {
		t.Fatalf("segment messages = %d, want 2", len(task.Messages))
	}
	if len(task.Actions) != 1 || task.Actions[0].ID != "ACT_000001" {
		t.Fatalf("segment actions = %+v", task.Actions)
	}
}

func TestBuildReportTaskCarriesEvidenceInputs(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)
	metrics := model.BehaviorMetrics{
		RelationshipID: "rel_test",
		PeriodStart:    start,
		PeriodEnd:      start.Add(24 * time.Hour),
		Values:         map[string]interface{}{"message_volume": 2},
	}
	dimensions := model.PsychologicalDimensions{RelationshipID: "rel_test", Values: map[string]interface{}{}}
	events := []model.RelationshipEvent{{ID: "EVT_000001", EvidenceMsgIDs: []string{"MSG_000001"}}}
	messages := []model.Message{taskMsg(1, "PERSON_A", 0, "第一句")}

	task := BuildReportTask("rel_test", messages, metrics, dimensions, events, []string{"证据不足"})

	if task.PeriodStart == "" || task.PeriodEnd == "" {
		t.Fatal("period should be formatted")
	}
	if len(task.Events) != 1 || task.Events[0].EvidenceMsgIDs[0] != "MSG_000001" {
		t.Fatalf("events not preserved: %+v", task.Events)
	}
	if task.CounterEvidence[0] != "证据不足" {
		t.Fatalf("counter evidence not preserved: %+v", task.CounterEvidence)
	}
	if len(task.Messages) != 1 || task.Messages[0].ID != "MSG_000001" {
		t.Fatalf("messages not preserved: %+v", task.Messages)
	}
}

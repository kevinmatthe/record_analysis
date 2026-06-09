package importer

import "testing"

func TestParseChatFileSupportsTxtCsvAndJson(t *testing.T) {
	formats := []string{
		"../../records/某人/chat.txt",
		"../../records/某人/chat.csv",
		"../../records/某人/chat.json",
	}

	for _, path := range formats {
		messages, err := ParseChatFile(path, "rel_test", true)
		if err != nil {
			t.Fatalf("ParseChatFile(%s) returned error: %v", path, err)
		}
		if len(messages) < 100 {
			t.Fatalf("ParseChatFile(%s) returned %d messages, want many", path, len(messages))
		}
		if messages[0].ID == "" || messages[0].RelationshipID != "rel_test" {
			t.Fatalf("message identity was not normalized: %+v", messages[0])
		}
	}
}

func TestParseChatFileCanFilterSystemMessages(t *testing.T) {
	messages, err := ParseChatFile("../../records/某人/chat.csv", "rel_test", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) == 0 {
		t.Fatal("expected non-empty messages")
	}
	for _, message := range messages {
		if message.Sender != "PERSON_A" && message.Sender != "PERSON_B" {
			t.Fatalf("unexpected sender after system filtering: %s", message.Sender)
		}
		if message.OriginalSender == "" {
			t.Fatal("expected original sender to be preserved")
		}
	}
}

func TestParseChatFilePreservesUTF8ChineseContent(t *testing.T) {
	messages, err := ParseChatFile("../../records/某人/chat.csv", "rel_test", false)
	if err != nil {
		t.Fatal(err)
	}
	if messages[0].Content != "我通过了你的朋友验证请求，现在我们可以开始聊天了" {
		t.Fatalf("content = %q", messages[0].Content)
	}
	if messages[0].MsgType != "文本" {
		t.Fatalf("msg type = %q", messages[0].MsgType)
	}
}

func TestNormalizeRowsFiltersNonTextPlaceholders(t *testing.T) {
	rows := []rawMessage{
		{Time: "2026-06-01 10:00:00", Sender: "我", TypeName: "文本", Content: "正常文本"},
		{Time: "2026-06-01 10:01:00", Sender: "我", TypeName: "图片", Content: "[图片]"},
		{Time: "2026-06-01 10:02:00", Sender: "小青", TypeName: "文本", Content: "小青撤回了一条消息"},
		{Time: "2026-06-01 10:03:00", Sender: "小青", TypeName: "文本", Content: "[语音]"},
	}

	messages, err := normalizeRows(rows, "rel_test", "csv")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %+v", messages)
	}
	if messages[0].Content != "正常文本" {
		t.Fatalf("content = %q", messages[0].Content)
	}
}

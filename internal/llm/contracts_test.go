package llm

import (
	"os"
	"strings"
	"testing"
)

func TestPromptContractsRequireEvidenceAndAvoidDiagnostics(t *testing.T) {
	paths := []string{
		"../../llm/prompts/action_extraction.md",
		"../../llm/prompts/action_batch_extraction.md",
		"../../llm/prompts/event_extraction.md",
		"../../llm/prompts/report_generation.md",
		"../../llm/prompts/dimension_generation.md",
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(content)
		for _, required := range []string{"evidence_msg_ids", "confidence", "不要使用"} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing required contract phrase %q", path, required)
			}
		}
		for _, forbidden := range []string{"人格障碍", "PUA", "冷暴力", "不爱了"} {
			if !strings.Contains(text, forbidden) {
				t.Fatalf("%s should explicitly forbid %q", path, forbidden)
			}
		}
	}
}

func TestJSONSchemasExistForStructuredLLMOutputs(t *testing.T) {
	paths := []string{
		"../../llm/schemas/action.schema.json",
		"../../llm/schemas/action_batch.schema.json",
		"../../llm/schemas/event.schema.json",
		"../../llm/schemas/report_claim.schema.json",
		"../../llm/schemas/dimensions.schema.json",
		"../../llm/schemas/report.schema.json",
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(content)
		for _, required := range []string{`"type"`, "evidence_msg_ids", "confidence"} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing %q", path, required)
			}
		}
	}
}

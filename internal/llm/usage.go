package llm

import "context"

type UsageEvent struct {
	SchemaName       string `json:"schema_name"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
}

type usageReporterKey struct{}

func WithUsageReporter(ctx context.Context, reporter func(UsageEvent)) context.Context {
	if reporter == nil {
		return ctx
	}
	return context.WithValue(ctx, usageReporterKey{}, reporter)
}

func reportUsage(ctx context.Context, event UsageEvent) {
	reporter, _ := ctx.Value(usageReporterKey{}).(func(UsageEvent))
	if reporter != nil {
		reporter(event)
	}
}

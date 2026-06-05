package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func decodeResponsesTools(t *testing.T, raw []byte) []map[string]any {
	t.Helper()

	var tools []map[string]any
	if len(raw) == 0 {
		return tools
	}
	if err := common.Unmarshal(raw, &tools); err != nil {
		t.Fatalf("failed to decode tools: %v", err)
	}
	return tools
}

func TestChatCompletionsRequestToResponsesRequestMapsWebSearchOptionsToTool(t *testing.T) {
	stream := true
	request := &dto.GeneralOpenAIRequest{
		Model:  "gpt-4.1",
		Stream: &stream,
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "search the web",
			},
		},
		WebSearchOptions: &dto.WebSearchOptions{
			SearchContextSize: "high",
		},
	}

	responsesRequest, err := ChatCompletionsRequestToResponsesRequest(request)
	if err != nil {
		t.Fatalf("expected conversion to succeed: %v", err)
	}

	tools := decodeResponsesTools(t, responsesRequest.Tools)
	if len(tools) != 1 {
		t.Fatalf("expected exactly one built-in tool, got %#v", tools)
	}
	if tools[0]["type"] != dto.BuildInToolWebSearchPreview {
		t.Fatalf("expected web search tool, got %#v", tools[0])
	}
	if tools[0]["search_context_size"] != "high" {
		t.Fatalf("expected high search context size, got %#v", tools[0])
	}
}

func TestChatCompletionsRequestToResponsesRequestAppendsWebSearchTool(t *testing.T) {
	stream := true
	request := &dto.GeneralOpenAIRequest{
		Model:  "gpt-4.1",
		Stream: &stream,
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "search the web and call a function",
			},
		},
		Tools: []dto.ToolCallRequest{
			{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        "lookup_weather",
					Description: "Look up weather",
					Parameters: map[string]any{
						"type": "object",
					},
				},
			},
		},
		WebSearchOptions: &dto.WebSearchOptions{
			SearchContextSize: "medium",
		},
	}

	responsesRequest, err := ChatCompletionsRequestToResponsesRequest(request)
	if err != nil {
		t.Fatalf("expected conversion to succeed: %v", err)
	}

	tools := decodeResponsesTools(t, responsesRequest.Tools)
	if len(tools) != 2 {
		t.Fatalf("expected function tool plus web search tool, got %#v", tools)
	}
	if tools[0]["type"] != "function" {
		t.Fatalf("expected first tool to preserve function call, got %#v", tools[0])
	}
	if tools[1]["type"] != dto.BuildInToolWebSearchPreview {
		t.Fatalf("expected appended web search tool, got %#v", tools[1])
	}
}

package groksubscription

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const grokCustomToolsContextKey = "grok_subscription_custom_tools"

func customToolInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{
				"type":        "string",
				"description": "The raw input for this tool, passed through verbatim.",
			},
		},
		"required": []string{"input"},
	}
}

func rewriteCustomToolHistory(value any, customTools map[string]bool) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			rewriteCustomToolHistory(item, customTools)
		}
	case map[string]any:
		switch itemType, _ := typed["type"].(string); itemType {
		case "custom_tool_call":
			name, _ := typed["name"].(string)
			if customTools[strings.TrimSpace(name)] {
				typed["type"] = "function_call"
				typed["arguments"] = customToolCallArguments(interfaceString(typed["input"]))
				delete(typed, "input")
			}
		case "custom_tool_call_output":
			typed["type"] = "function_call_output"
			normalizeCustomToolOutput(typed)
		}
		for _, child := range typed {
			rewriteCustomToolHistory(child, customTools)
		}
	}
}

func customToolCallArguments(input string) string {
	encoded, err := common.Marshal(map[string]string{"input": input})
	if err != nil {
		return `{"input":""}`
	}
	return string(encoded)
}

func normalizeCustomToolOutput(item map[string]any) {
	output, exists := item["output"]
	if !exists {
		return
	}
	if _, ok := output.(string); ok {
		return
	}
	if output == nil {
		item["output"] = ""
		return
	}
	encoded, err := common.Marshal(output)
	if err != nil {
		item["output"] = ""
		return
	}
	item["output"] = string(encoded)
}

func interfaceString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func extractCustomToolInput(arguments string) string {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return ""
	}
	var object map[string]any
	if err := common.Unmarshal([]byte(trimmed), &object); err != nil {
		return trimmed
	}
	if raw, ok := object["input"]; ok {
		if input, ok := raw.(string); ok {
			return input
		}
		return trimmed
	}
	if len(object) == 0 {
		return ""
	}
	return trimmed
}

func customToolsFromContext(c *gin.Context) map[string]bool {
	if c == nil {
		return nil
	}
	value, exists := c.Get(grokCustomToolsContextKey)
	if !exists {
		return nil
	}
	customTools, _ := value.(map[string]bool)
	return customTools
}

func prepareCustomToolResponse(c *gin.Context, resp *http.Response, stream bool) error {
	customTools := customToolsFromContext(c)
	if len(customTools) == 0 || resp == nil || resp.Body == nil {
		return nil
	}
	if stream {
		resp.Body = newCustomToolStreamBody(resp.Body, customTools)
		resp.ContentLength = -1
		resp.Header.Del("Content-Length")
		return nil
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	restored, err := restoreCustomToolPayload(payload, customTools)
	if err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(restored))
	resp.ContentLength = int64(len(restored))
	resp.Header.Set("Content-Length", fmt.Sprint(len(restored)))
	return nil
}

func restoreCustomToolPayload(payload []byte, customTools map[string]bool) ([]byte, error) {
	var value any
	if err := common.Unmarshal(payload, &value); err != nil {
		return nil, err
	}
	restoreCustomToolValue(value, customTools)
	return common.Marshal(value)
}

func restoreCustomToolValue(value any, customTools map[string]bool) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			restoreCustomToolValue(item, customTools)
		}
	case map[string]any:
		itemType, _ := typed["type"].(string)
		name, _ := typed["name"].(string)
		if itemType == "function_call" && customTools[strings.TrimSpace(name)] {
			typed["type"] = "custom_tool_call"
			typed["input"] = extractCustomToolInput(interfaceString(typed["arguments"]))
			delete(typed, "arguments")
			delete(typed, "namespace")
		}
		for _, child := range typed {
			restoreCustomToolValue(child, customTools)
		}
	}
}

type customToolStreamCall struct {
	name      string
	callID    string
	itemID    string
	outputKey string
	arguments strings.Builder
}

type customToolStreamRestorer struct {
	customTools map[string]bool
	calls       map[string]*customToolStreamCall
	byOutput    map[string]*customToolStreamCall
	nextSeq     int
	seenSeq     bool
}

func newCustomToolStreamRestorer(customTools map[string]bool) *customToolStreamRestorer {
	return &customToolStreamRestorer{
		customTools: customTools,
		calls:       make(map[string]*customToolStreamCall),
		byOutput:    make(map[string]*customToolStreamCall),
	}
}

func (r *customToolStreamRestorer) transform(payload []byte) ([][]byte, error) {
	var event map[string]any
	if err := common.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	if !r.seenSeq {
		r.nextSeq = intValue(event["sequence_number"])
		r.seenSeq = true
	}

	eventType, _ := event["type"].(string)
	switch eventType {
	case "response.output_item.added":
		if call := r.recordItem(event); call != nil {
			item, _ := event["item"].(map[string]any)
			item["type"] = "custom_tool_call"
			item["input"] = ""
			delete(item, "arguments")
			delete(item, "namespace")
		}
		return r.encodeEvents(event)
	case "response.function_call_arguments.delta":
		if call := r.callFor(event); call != nil {
			_, _ = call.arguments.WriteString(interfaceString(event["delta"]))
			return nil, nil
		}
		return r.encodeEvents(event)
	case "response.function_call_arguments.done":
		if call := r.callFor(event); call != nil {
			if arguments := interfaceString(event["arguments"]); arguments != "" {
				call.arguments.Reset()
				_, _ = call.arguments.WriteString(arguments)
			}
			input := extractCustomToolInput(call.arguments.String())
			events := make([]map[string]any, 0, 2)
			if input != "" {
				events = append(events, map[string]any{
					"type":         "response.custom_tool_call_input.delta",
					"output_index": event["output_index"],
					"item_id":      call.itemID,
					"delta":        input,
				})
			}
			events = append(events, map[string]any{
				"type":         "response.custom_tool_call_input.done",
				"output_index": event["output_index"],
				"item_id":      call.itemID,
				"call_id":      call.callID,
				"name":         call.name,
				"input":        input,
			})
			return r.encodeEvents(events...)
		}
		return r.encodeEvents(event)
	case "response.output_item.done":
		if call := r.recordItem(event); call != nil {
			item, _ := event["item"].(map[string]any)
			if arguments := interfaceString(item["arguments"]); arguments != "" {
				call.arguments.Reset()
				_, _ = call.arguments.WriteString(arguments)
			}
			item["type"] = "custom_tool_call"
			item["input"] = extractCustomToolInput(call.arguments.String())
			delete(item, "arguments")
			delete(item, "namespace")
			r.removeCall(call)
		}
		return r.encodeEvents(event)
	case "response.completed", "response.incomplete", "response.failed":
		restoreCustomToolValue(event, r.customTools)
		return r.encodeEvents(event)
	default:
		return r.encodeEvents(event)
	}
}

func (r *customToolStreamRestorer) encodeEvents(events ...map[string]any) ([][]byte, error) {
	output := make([][]byte, 0, len(events))
	for _, event := range events {
		event["sequence_number"] = r.nextSeq
		r.nextSeq++
		encoded, err := common.Marshal(event)
		if err != nil {
			return nil, err
		}
		output = append(output, encoded)
	}
	return output, nil
}

func (r *customToolStreamRestorer) recordItem(event map[string]any) *customToolStreamCall {
	item, _ := event["item"].(map[string]any)
	if item == nil || interfaceString(item["type"]) != "function_call" {
		return nil
	}
	name := strings.TrimSpace(interfaceString(item["name"]))
	if !r.customTools[name] {
		return nil
	}
	itemID := interfaceString(item["id"])
	callID := interfaceString(item["call_id"])
	outputKey := interfaceString(event["output_index"])
	call := r.calls[itemID]
	if call == nil {
		call = r.calls[callID]
	}
	if call == nil {
		call = &customToolStreamCall{
			name:      name,
			callID:    callID,
			itemID:    itemID,
			outputKey: outputKey,
		}
	}
	if itemID != "" {
		r.calls[itemID] = call
	}
	if callID != "" {
		r.calls[callID] = call
	}
	r.byOutput[outputKey] = call
	if arguments := interfaceString(item["arguments"]); arguments != "" {
		call.arguments.Reset()
		_, _ = call.arguments.WriteString(arguments)
	}
	return call
}

func (r *customToolStreamRestorer) callFor(event map[string]any) *customToolStreamCall {
	if call := r.calls[interfaceString(event["item_id"])]; call != nil {
		return call
	}
	if call := r.calls[interfaceString(event["call_id"])]; call != nil {
		return call
	}
	return r.byOutput[interfaceString(event["output_index"])]
}

func (r *customToolStreamRestorer) removeCall(call *customToolStreamCall) {
	delete(r.calls, call.itemID)
	delete(r.calls, call.callID)
	delete(r.byOutput, call.outputKey)
}

func intValue(value any) int {
	switch number := value.(type) {
	case float64:
		return int(number)
	case int:
		return number
	default:
		return 0
	}
}

type customToolTransformBody struct {
	reader io.ReadCloser
	source io.ReadCloser
	once   sync.Once
}

func newCustomToolStreamBody(source io.ReadCloser, customTools map[string]bool) io.ReadCloser {
	reader, writer := io.Pipe()
	body := &customToolTransformBody{reader: reader, source: source}
	go func() {
		scanner := bufio.NewScanner(source)
		scanner.Buffer(make([]byte, 64<<10), 128<<20)
		restorer := newCustomToolStreamRestorer(customTools)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				if _, err := io.WriteString(writer, line+"\n"); err != nil {
					return
				}
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || strings.HasPrefix(payload, "[DONE]") {
				if _, err := io.WriteString(writer, line+"\n"); err != nil {
					return
				}
				continue
			}
			events, err := restorer.transform([]byte(payload))
			if err != nil {
				_ = writer.CloseWithError(err)
				return
			}
			for _, event := range events {
				if _, err := io.WriteString(writer, "data: "+string(event)+"\n\n"); err != nil {
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()
	return body
}

func (b *customToolTransformBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *customToolTransformBody) Close() error {
	var closeErr error
	b.once.Do(func() {
		closeErr = b.source.Close()
		if err := b.reader.Close(); closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

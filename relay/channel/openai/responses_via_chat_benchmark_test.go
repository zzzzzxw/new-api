package openai

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiChatToResponsesStreamHandlerFlushesBeforeUpstreamDone(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	pr, pw := io.Pipe()
	recorder := &firstWriteRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		firstWrite:       make(chan time.Time, 1),
	}
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "responses-stream-flush-test")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-chat",
		},
		IsStream:           true,
		RelayFormat:        types.RelayFormatOpenAI,
		ShouldIncludeUsage: true,
		DisablePing:        true,
	}

	handlerDone := make(chan error, 1)
	go func() {
		_, err := OaiChatToResponsesStreamHandler(c, info, resp)
		if err != nil {
			handlerDone <- err
			return
		}
		handlerDone <- nil
	}()

	upstreamDone := make(chan struct{})
	go func() {
		defer close(upstreamDone)
		defer pw.Close()
		fmt.Fprintln(pw, `data: {"id":"chatcmpl_flush","object":"chat.completion.chunk","created":1710000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`)
		fmt.Fprintln(pw, `data: {"id":"chatcmpl_flush","object":"chat.completion.chunk","created":1710000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`)
		time.Sleep(150 * time.Millisecond)
		fmt.Fprintln(pw, `data: {"id":"chatcmpl_flush","object":"chat.completion.chunk","created":1710000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
		fmt.Fprintln(pw, `data: {"id":"chatcmpl_flush","object":"chat.completion.chunk","created":1710000000,"model":"deepseek-chat","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		fmt.Fprintln(pw, `data: [DONE]`)
	}()

	select {
	case <-recorder.firstWrite:
	case <-upstreamDone:
		t.Fatal("handler did not write before upstream completed")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first downstream write")
	}

	select {
	case err := <-handlerDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for handler completion")
	}
	require.Contains(t, recorder.Body.String(), `response.output_text.delta`)
}

func BenchmarkChatStreamProxyDirectVsResponsesCompat(b *testing.B) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	b.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	b.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	const tokenChunks = 800
	body := syntheticChatCompletionSSE(tokenChunks)

	b.Run("direct_chat_stream", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c, resp, info := newBenchmarkStreamContext("/v1/chat/completions", body)
			usage, err := OaiStreamHandler(c, info, resp)
			if err != nil {
				b.Fatalf("OaiStreamHandler error: %v", err)
			}
			if usage == nil || usage.TotalTokens != tokenChunks+1 {
				b.Fatalf("unexpected usage: %+v", usage)
			}
		}
	})

	b.Run("responses_compat_stream", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c, resp, info := newBenchmarkStreamContext("/v1/responses", body)
			usage, err := OaiChatToResponsesStreamHandler(c, info, resp)
			if err != nil {
				b.Fatalf("OaiChatToResponsesStreamHandler error: %v", err)
			}
			if usage == nil || usage.TotalTokens != tokenChunks+1 {
				b.Fatalf("unexpected usage: %+v", usage)
			}
		}
	})
}

type firstWriteRecorder struct {
	*httptest.ResponseRecorder
	firstWrite chan time.Time
	once       sync.Once
}

func (r *firstWriteRecorder) Write(data []byte) (int, error) {
	r.once.Do(func() {
		r.firstWrite <- time.Now()
	})
	return r.ResponseRecorder.Write(data)
}

func newBenchmarkStreamContext(path string, body string) (*gin.Context, *http.Response, *relaycommon.RelayInfo) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Set(common.RequestIdKey, "benchmark-stream")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-chat",
		},
		IsStream:           true,
		RelayFormat:        types.RelayFormatOpenAI,
		ShouldIncludeUsage: true,
		DisablePing:        true,
	}
	return c, resp, info
}

func syntheticChatCompletionSSE(tokenChunks int) string {
	lines := make([]string, 0, tokenChunks+4)
	lines = append(lines, `data: {"id":"chatcmpl_bench","object":"chat.completion.chunk","created":1710000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`)
	for i := 0; i < tokenChunks; i++ {
		lines = append(lines, fmt.Sprintf(`data: {"id":"chatcmpl_bench","object":"chat.completion.chunk","created":1710000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{"content":"t%d "},"finish_reason":null}]}`, i))
	}
	lines = append(lines,
		`data: {"id":"chatcmpl_bench","object":"chat.completion.chunk","created":1710000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		fmt.Sprintf(`data: {"id":"chatcmpl_bench","object":"chat.completion.chunk","created":1710000000,"model":"deepseek-chat","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":%d,"total_tokens":%d,"prompt_cache_hit_tokens":1}}`, tokenChunks, tokenChunks+1),
		`data: [DONE]`,
		``,
	)
	return strings.Join(lines, "\n")
}

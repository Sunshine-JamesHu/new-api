package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStreamTestContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(common.RequestIdKey, "stream-error-test")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"},
		IsStream:           true,
		RelayFormat:        types.RelayFormatOpenAI,
		RelayMode:          relayconstant.RelayModeChatCompletions,
		ShouldIncludeUsage: false,
		DisablePing:        true,
	}
	info.SetEstimatePromptTokens(13283)
	return c, recorder, resp, info
}

// Test 1: First chunk is error frame (client writer has not been written yet).
// Verifies error is returned without skipRetry (allowing channel retry), and no [DONE] is sent.
func TestOaiStreamHandler_FirstChunkError(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"error":{"message":"The server had an error processing your request. Sorry about that! You can retry your request","type":"server_error"}}`,
		``,
	}, "\n")

	c, recorder, resp, info := newStreamTestContext(t, body)

	usage, err := OaiStreamHandler(c, info, resp)
	require.NotNil(t, err)
	assert.Nil(t, usage)
	assert.Contains(t, err.Error(), "The server had an error")
	assert.False(t, types.IsSkipRetryError(err), "first chunk error before write should allow channel retry")
	assert.NotContains(t, recorder.Body.String(), "[DONE]")
}

// Test 2: Output chunk written, then subsequent chunk is error frame.
// Verifies error is forwarded, skipRetry is set to prevent cross-channel re-execution,
// and [DONE] is NOT sent, ensuring quota refund.
func TestOaiStreamHandler_MidStreamError(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"id":"chatcmpl-1","choices":[{"delta":{"content":"Hi"}},{"delta":{}}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"delta":{"content":" there"}},{"delta":{}}]}`,
		`data: {"error":{"message":"upstream overloaded. You can retry your request","type":"server_error"}}`,
		``,
	}, "\n")

	c, recorder, resp, info := newStreamTestContext(t, body)

	usage, err := OaiStreamHandler(c, info, resp)
	require.NotNil(t, err)
	assert.Nil(t, usage)
	assert.True(t, types.IsSkipRetryError(err), "error after client write should have skipRetry")
	assert.Contains(t, recorder.Body.String(), "upstream overloaded")
	assert.NotContains(t, recorder.Body.String(), "[DONE]")
}

// Test 3: Customer scenario - generated 1 token, then stream abruptly disconnected
// (EOF without finish_reason and without [DONE]).
// Verifies <= 5 token abnormal end is detected as BadResponse, returning error to trigger full refund.
func TestOaiStreamHandler_AbnormalEarlyDisconnect(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	// Only 1 token emitted, then immediate EOF
	body := strings.Join([]string{
		`data: {"id":"chatcmpl-1","choices":[{"delta":{"content":"1"}}]}`,
		``,
	}, "\n")

	c, recorder, resp, info := newStreamTestContext(t, body)

	usage, err := OaiStreamHandler(c, info, resp)
	require.NotNil(t, err)
	assert.Nil(t, usage)
	assert.Equal(t, types.ErrorCodeBadResponse, err.GetErrorCode())
	assert.NotContains(t, recorder.Body.String(), "[DONE]")
}

// Test 4: Normal 1-token response with finish_reason: "stop" and [DONE].
// Verifies legitimate short responses are billed normally without false error.
func TestOaiStreamHandler_NormalOneTokenResponse(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"id":"chatcmpl-1","choices":[{"delta":{"content":"Yes"}}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newStreamTestContext(t, body)

	usage, err := OaiStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 2, usage.CompletionTokens)
	assert.Contains(t, recorder.Body.String(), "[DONE]")
}

// Test 5: Substantive response (> 5 tokens) with premature disconnect.
// Verifies partial settlement occurs when substantial content was generated.
func TestOaiStreamHandler_SubstantiveContentDisconnect(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"id":"chatcmpl-1","choices":[{"delta":{"content":"This is a substantially long response with more than five tokens generated before disconnection."}}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"delta":{"content":" Additional text continuing here."}}]}`,
		``,
	}, "\n")

	c, _, resp, info := newStreamTestContext(t, body)

	usage, err := OaiStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)
	assert.Greater(t, usage.CompletionTokens, 5)
}

// Test 6: Responses stream receives response.failed event.
// Verifies OaiResponsesStreamHandler captures the failure and returns an error.
func TestOaiResponsesStreamHandler_FailureEvent(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-4o","created_at":1710000000}}`,
		`data: {"type":"response.failed","response":{"status":"failed","error":{"type":"server_error","message":"The server encountered an error"}}}`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"},
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAI,
		DisablePing: true,
	}

	usage, err := OaiResponsesStreamHandler(c, info, resp)
	require.NotNil(t, err)
	assert.Nil(t, usage)
	assert.Contains(t, err.Error(), "The server encountered an error")
}

func TestOaiResponsesToChatStreamHandler_ErrorFrame(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-4o","created_at":1710000000}}`,
		`data: {"error":{"message":"The server had an error processing your request. Sorry about that! You can retry your request","type":"server_error"}}`,
		``,
	}, "\n")

	c, _, resp, info := newStreamTestContext(t, body)
	info.RelayFormat = types.RelayFormatOpenAI

	usage, err := OaiResponsesToChatStreamHandler(c, info, resp)
	require.NotNil(t, err)
	assert.Nil(t, usage)
	assert.Contains(t, err.Error(), "The server had an error processing your request")
}

func TestOaiResponsesToChatStreamHandler_EarlyDisconnect(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	// Only 1 token generated before abnormal connection close without response.completed or [DONE]
	body := strings.Join([]string{
		`data: {"type":"response.output_item.added","item":{"type":"message","id":"msg_1","role":"assistant"}}`,
		`data: {"type":"response.output_text.delta","delta":"Hello"}`,
		``,
	}, "\n")

	c, _, resp, info := newStreamTestContext(t, body)
	info.RelayFormat = types.RelayFormatOpenAI

	usage, err := OaiResponsesToChatStreamHandler(c, info, resp)
	require.NotNil(t, err)
	assert.Nil(t, usage)
	assert.Equal(t, types.ErrorCodeBadResponse, err.GetErrorCode())
}

func TestOaiResponsesStreamHandler_EarlyDisconnect(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	// Only 1 token generated, no response.completed or [DONE]
	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"Hi"}`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"},
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAIResponses,
		DisablePing: true,
	}

	usage, err := OaiResponsesStreamHandler(c, info, resp)
	require.NotNil(t, err)
	assert.Nil(t, usage)
	assert.Equal(t, types.ErrorCodeBadResponse, err.GetErrorCode())
}

func TestOaiResponsesToChatBufferedStreamHandler_ErrorFrame(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"part"}`,
		`data: {"error":{"message":"The server had an error. Please retry","type":"server_error"}}`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"},
		RelayFormat: types.RelayFormatOpenAI,
	}

	usage, err := OaiResponsesToChatBufferedStreamHandler(c, info, resp)
	require.NotNil(t, err)
	assert.Nil(t, usage)
	assert.Contains(t, err.Error(), "The server had an error")
}

func TestOaiResponsesToChatBufferedStreamHandler_EarlyDisconnect(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	// Only 1 short token generated without response.completed or response.done
	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"Hi"}`,
		``,
	}, "\n")

	c, _, resp, info := newStreamTestContext(t, body)

	usage, err := OaiResponsesToChatBufferedStreamHandler(c, info, resp)
	require.NotNil(t, err)
	assert.Nil(t, usage)
	assert.Equal(t, types.ErrorCodeBadResponse, err.GetErrorCode())
}

package common

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func taskValidationContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestValidateBasicTaskRequestUsesEffectivePrompt(t *testing.T) {
	c := taskValidationContext(`{"model":"m","metadata":{"input":{"prompt":"inner"}}}`)
	info := &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}

	taskErr := ValidateBasicTaskRequest(c, info, constant.TaskActionTextGenerate)

	require.Nil(t, taskErr)
	req, err := GetTaskRequest(c)
	require.NoError(t, err)
	require.Equal(t, "inner", req.EffectivePrompt())
}

func TestValidateMetadataPassthroughTaskRequestRejectsMissingEffectivePrompt(t *testing.T) {
	c := taskValidationContext(`{"model":"m","metadata":{"duration":4}}`)
	info := &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}

	taskErr := ValidateMetadataPassthroughTaskRequest(c, info, constant.TaskActionTextGenerate)

	require.NotNil(t, taskErr)
	require.Equal(t, "invalid_request", taskErr.Code)
}

func TestValidateMultipartDirectUsesInputPromptFallback(t *testing.T) {
	c := taskValidationContext(`{"model":"m","input":{"prompt":"input prompt"}}`)
	info := &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}

	taskErr := ValidateMultipartDirect(c, info)

	require.Nil(t, taskErr)
	req, err := GetTaskRequest(c)
	require.NoError(t, err)
	require.Equal(t, "input prompt", req.EffectivePrompt())
}

func TestValidateMultipartDirectNormalizesImageField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.NewReader(`{"model":"wan2.7-i2v","prompt":"animate","image":" https://example.com/first.png "}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	info := &RelayInfo{
		TaskRelayInfo: &TaskRelayInfo{},
	}

	taskErr := ValidateMultipartDirect(context, info)

	require.Nil(t, taskErr)
	storedReq, err := GetTaskRequest(context)
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/first.png"}, storedReq.Images)
	require.Equal(t, constant.TaskActionGenerate, info.Action)
}

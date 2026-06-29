package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func HappyHorseOfficialRequestConvert() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Set(common.KeyTaskOfficialProvider, common.TaskOfficialProviderHappyHorse)
		if c.Request.Method == http.MethodGet {
			c.Set("relay_mode", relayconstant.RelayModeVideoFetchByID)
			c.Request.URL.Path = "/v1/video/generations/" + c.Param("task_id")
			c.Next()
			return
		}

		var originalReq map[string]any
		if err := common.UnmarshalBodyReusable(c, &originalReq); err != nil {
			c.Next()
			return
		}

		unifiedReq := map[string]any{"metadata": originalReq}
		copyStringField(originalReq, unifiedReq, "model")
		copyPromptField(originalReq, unifiedReq)
		copyDurationField(originalReq, unifiedReq)
		copyResolutionField(originalReq, unifiedReq)

		rewriteOfficialVideoRequest(c, unifiedReq)
		c.Next()
	}
}

func DoubaoOfficialRequestConvert() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Set(common.KeyTaskOfficialProvider, common.TaskOfficialProviderDoubao)
		if c.Request.Method == http.MethodGet {
			c.Set("relay_mode", relayconstant.RelayModeVideoFetchByID)
			c.Request.URL.Path = "/v1/video/generations/" + c.Param("task_id")
			c.Next()
			return
		}

		var originalReq map[string]any
		if err := common.UnmarshalBodyReusable(c, &originalReq); err != nil {
			c.Next()
			return
		}

		unifiedReq := map[string]any{"metadata": originalReq}
		copyStringField(originalReq, unifiedReq, "model")
		copyPromptField(originalReq, unifiedReq)
		copyDurationField(originalReq, unifiedReq)
		copyResolutionField(originalReq, unifiedReq)

		rewriteOfficialVideoRequest(c, unifiedReq)
		c.Next()
	}
}

func rewriteOfficialVideoRequest(c *gin.Context, body map[string]any) {
	data, err := common.Marshal(body)
	if err != nil {
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	c.Request.URL.Path = "/v1/video/generations"
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyBodyStorage, nil)
	c.Set(common.KeyRequestBody, data)
}

func copyStringField(source, target map[string]any, key string) {
	value, ok := source[key].(string)
	if ok && strings.TrimSpace(value) != "" {
		target[key] = value
	}
}

func copyPromptField(source, target map[string]any) {
	if prompt, ok := source["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		target["prompt"] = prompt
		return
	}
	if input, ok := source["input"].(map[string]any); ok {
		if prompt, ok := input["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
			target["prompt"] = prompt
			return
		}
	}
	if content, ok := source["content"].([]any); ok {
		for _, item := range content {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := itemMap["type"].(string)
			text, _ := itemMap["text"].(string)
			if strings.TrimSpace(text) != "" && (itemType == "" || itemType == "text") {
				target["prompt"] = text
				return
			}
		}
	}
}

func copyDurationField(source, target map[string]any) {
	for _, key := range []string{"duration", "seconds"} {
		if value, ok := source[key]; ok && value != nil {
			target[key] = value
			return
		}
	}
	if parameters, ok := source["parameters"].(map[string]any); ok {
		if value, ok := parameters["duration"]; ok && value != nil {
			target["duration"] = value
		}
	}
}

func copyResolutionField(source, target map[string]any) {
	if value, ok := source["resolution"].(string); ok && strings.TrimSpace(value) != "" {
		target["resolution"] = value
		return
	}
	if value, ok := source["size"].(string); ok && strings.TrimSpace(value) != "" {
		target["size"] = value
		return
	}
	if parameters, ok := source["parameters"].(map[string]any); ok {
		if value, ok := parameters["resolution"].(string); ok && strings.TrimSpace(value) != "" {
			target["resolution"] = value
		}
	}
}

package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func runOfficialVideoMiddleware(t *testing.T, path, body string, handler gin.HandlerFunc) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	handler(c)
	return c
}

func TestHappyHorseOfficialRequestConvertWrapsOfficialBody(t *testing.T) {
	c := runOfficialVideoMiddleware(t,
		"/api/v1/services/aigc/video-generation/video-synthesis",
		`{"model":"happyhorse-1.0-t2v","input":{"prompt":"move"},"parameters":{"duration":6,"resolution":"1080P"}}`,
		HappyHorseOfficialRequestConvert(),
	)

	data, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, common.Unmarshal(data, &got))
	require.Equal(t, "/v1/video/generations", c.Request.URL.Path)
	require.Equal(t, common.TaskOfficialProviderHappyHorse, c.GetString(common.KeyTaskOfficialProvider))
	require.Equal(t, "happyhorse-1.0-t2v", got["model"])
	require.Equal(t, "move", got["prompt"])
	require.Equal(t, float64(6), got["duration"])
	require.Equal(t, "1080P", got["resolution"])
	oldStorage, hasOldStorage := c.Get(common.KeyBodyStorage)
	require.True(t, hasOldStorage)
	require.Nil(t, oldStorage)
	require.Equal(t, map[string]any{
		"model": "happyhorse-1.0-t2v",
		"input": map[string]any{
			"prompt": "move",
		},
		"parameters": map[string]any{
			"duration":   float64(6),
			"resolution": "1080P",
		},
	}, got["metadata"])
}

func TestDoubaoOfficialRequestConvertWrapsOfficialBody(t *testing.T) {
	c := runOfficialVideoMiddleware(t,
		"/api/v3/contents/generations/tasks",
		`{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"pan"}],"duration":5,"resolution":"720p"}`,
		DoubaoOfficialRequestConvert(),
	)

	data, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, common.Unmarshal(data, &got))
	require.Equal(t, "/v1/video/generations", c.Request.URL.Path)
	require.Equal(t, common.TaskOfficialProviderDoubao, c.GetString(common.KeyTaskOfficialProvider))
	require.Equal(t, "doubao-seedance-2-0-260128", got["model"])
	require.Equal(t, float64(5), got["duration"])
	require.Equal(t, "720p", got["resolution"])
	require.Equal(t, "pan", got["prompt"])
	require.Contains(t, got["metadata"], "content")
}

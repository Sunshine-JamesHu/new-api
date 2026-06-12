package newapivideo

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withNewApiVideoBillingConfig(t *testing.T, values map[string]string) {
	t.Helper()
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(values))
}

func newApiVideoRelayInfo(baseURL string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: "happyhorse-1.0-t2v",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeNewApiVideo,
			ChannelBaseUrl:    baseURL,
			ApiKey:            "sk-newapi",
			UpstreamModelName: "happyhorse-1.0-t2v",
		},
	}
}

func setNewApiVideoTaskRequest(t *testing.T, c *gin.Context) {
	t.Helper()
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	taskErr := relaycommon.ValidateMultipartDirect(c, info)
	require.Nil(t, taskErr)
}

func TestNewApiVideoSubmitUsesVideoGenerationsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer sk-newapi", r.Header.Get("Authorization"))
		data, err := common.Marshal(dto.TaskResponse[any]{
			Code: "success",
			Data: map[string]any{
				"task_id":  "task_upstream",
				"status":   string(model.TaskStatusSubmitted),
				"progress": "10%",
			},
		})
		require.NoError(t, err)
		_, _ = w.Write(data)
	}))
	defer upstream.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewBufferString(`{"model":"happyhorse-1.0-t2v","prompt":"scene"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	bodyStorage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	c.Request.Body = io.NopCloser(bodyStorage)

	info := newApiVideoRelayInfo(upstream.URL)
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	resp, err := adaptor.DoRequest(c, info, body)
	require.NoError(t, err)
	taskID, _, taskErr := adaptor.DoResponse(c, resp, info)
	require.Nil(t, taskErr)
	require.Equal(t, "/v1/video/generations", gotPath)
	require.Equal(t, "task_upstream", taskID)
	require.Contains(t, recorder.Body.String(), "task_public")
}

func TestNewApiVideoBuildRequestBodyPassesThroughMissingResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewBufferString(`{"model":"happyhorse-1.0-t2v","prompt":"scene"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	bodyStorage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	c.Request.Body = io.NopCloser(bodyStorage)
	setNewApiVideoTaskRequest(t, c)

	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, newApiVideoRelayInfo("https://example.com"))
	require.NoError(t, err)
	bodyBytes, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(bodyBytes, &payload))
	require.NotContains(t, payload, "resolution")
}

func TestNewApiVideoBuildRequestBodyPassesThroughResolutionValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewBufferString(`{"model":"happyhorse-1.0-t2v","prompt":"scene","resolution":"1080p"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	bodyStorage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	c.Request.Body = io.NopCloser(bodyStorage)
	setNewApiVideoTaskRequest(t, c)

	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, newApiVideoRelayInfo("https://example.com"))
	require.NoError(t, err)
	bodyBytes, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(bodyBytes, &payload))
	require.Equal(t, "1080p", payload["resolution"])
}

func TestNewApiVideoEstimateBillingUsesLocalRequestParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withNewApiVideoBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.0-t2v":"per_second"}`,
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewBufferString(`{"model":"happyhorse-1.0-t2v","prompt":"scene","duration":5,"resolution":"1080"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	bodyStorage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	c.Request.Body = io.NopCloser(bodyStorage)
	setNewApiVideoTaskRequest(t, c)

	ratios := (&TaskAdaptor{}).EstimateBilling(c, newApiVideoRelayInfo("https://example.com"))

	require.Equal(t, map[string]float64{
		"seconds":          5,
		"resolution-1080P": 1,
	}, ratios)
}

func TestNewApiVideoEstimateBillingDefaultsResolutionTo720P(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withNewApiVideoBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.0-t2v":"per_second"}`,
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewBufferString(`{"model":"happyhorse-1.0-t2v","prompt":"scene","duration":5}`))
	c.Request.Header.Set("Content-Type", "application/json")
	bodyStorage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	c.Request.Body = io.NopCloser(bodyStorage)
	setNewApiVideoTaskRequest(t, c)

	ratios := (&TaskAdaptor{}).EstimateBilling(c, newApiVideoRelayInfo("https://example.com"))

	require.Equal(t, map[string]float64{
		"seconds":         5,
		"resolution-720P": 1,
	}, ratios)
}

func TestNewApiVideoFetchTaskParsesTaskDtoResultURL(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/video/generations/task_upstream", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		data, err := common.Marshal(dto.TaskResponse[any]{
			Code: "success",
			Data: map[string]any{
				"task_id":    "task_upstream",
				"status":     string(model.TaskStatusSuccess),
				"progress":   "100%",
				"result_url": "https://example.com/video.mp4",
			},
		})
		require.NoError(t, err)
		_, _ = w.Write(data)
	}))
	defer upstream.Close()

	adaptor := &TaskAdaptor{}
	resp, err := adaptor.FetchTask(upstream.URL, "sk-newapi", map[string]any{"task_id": "task_upstream"}, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	taskInfo, err := adaptor.ParseTaskResult(body)
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusSuccess), taskInfo.Status)
	require.Equal(t, "https://example.com/video.mp4", taskInfo.Url)
	require.Equal(t, "100%", taskInfo.Progress)
}

func TestParseNewApiTaskWrappedOpenAIVideoDoesNotMatchTaskDto(t *testing.T) {
	body, err := common.Marshal(dto.TaskResponse[dto.OpenAIVideo]{
		Code: "success",
		Data: dto.OpenAIVideo{
			ID:        "video_upstream",
			TaskID:    "task_upstream",
			Status:    dto.VideoStatusCompleted,
			Progress:  100,
			Model:     "happyhorse-1.0-t2v",
			Metadata:  map[string]any{"url": "https://example.com/wrapped.mp4"},
			CreatedAt: 123,
		},
	})
	require.NoError(t, err)

	task, err := parseNewApiTask(body)
	require.NoError(t, err)
	require.Equal(t, "task_upstream", task.TaskID)
	require.Equal(t, model.TaskStatus(model.TaskStatusSuccess), task.Status)
	require.Equal(t, "100%", task.Progress)
	require.Equal(t, "https://example.com/wrapped.mp4", task.GetResultURL())
	require.Equal(t, "happyhorse-1.0-t2v", task.Properties.OriginModelName)
}

func TestParseNewApiTaskBareOpenAIVideoDoesNotMatchModelTask(t *testing.T) {
	body, err := common.Marshal(dto.OpenAIVideo{
		ID:        "video_upstream",
		TaskID:    "task_upstream",
		Status:    dto.VideoStatusInProgress,
		Progress:  37,
		Model:     "happyhorse-1.0-t2v",
		CreatedAt: 123,
	})
	require.NoError(t, err)

	task, err := parseNewApiTask(body)
	require.NoError(t, err)
	require.Equal(t, "task_upstream", task.TaskID)
	require.Equal(t, model.TaskStatus(model.TaskStatusInProgress), task.Status)
	require.Equal(t, "37%", task.Progress)
	require.Equal(t, "happyhorse-1.0-t2v", task.Properties.OriginModelName)
}

package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestHappyHorseOfficialFetchResponseUsesPublicTaskID(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		FailReason: "legacy",
		Data:       []byte(`{"output":{"task_id":"upstream_task","task_status":"RUNNING","video_url":"https://old.example/video.mp4"},"request_id":"req_1"}`),
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://example.com/result.mp4",
		},
	}

	data, err := officialVideoFetchResponse(common.TaskOfficialProviderHappyHorse, task)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, common.Unmarshal(data, &got))
	output := got["output"].(map[string]any)
	require.Equal(t, "task_public", output["task_id"])
	require.Equal(t, "SUCCEEDED", output["task_status"])
	require.Equal(t, "https://example.com/result.mp4", output["video_url"])
}

func TestDoubaoOfficialFetchResponseUsesPublicTaskID(t *testing.T) {
	task := &model.Task{
		TaskID: "task_public",
		Status: model.TaskStatusSuccess,
		Data:   []byte(`{"id":"upstream_task","status":"running","content":{"video_url":"https://old.example/video.mp4"}}`),
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://example.com/result.mp4",
		},
	}

	data, err := officialVideoFetchResponse(common.TaskOfficialProviderDoubao, task)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, common.Unmarshal(data, &got))
	require.Equal(t, "task_public", got["id"])
	require.Equal(t, "succeeded", got["status"])
	content := got["content"].(map[string]any)
	require.Equal(t, "https://example.com/result.mp4", content["video_url"])
}

package newapivideo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if taskErr := relaycommon.ValidateMetadataPassthroughTaskRequest(c, info, constant.TaskActionTextGenerate); taskErr != nil {
		return taskErr
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Model) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model is required"), "missing_model", http.StatusBadRequest)
	}
	if _, err = taskcommon.ResolveVideoBillingResolution(req, "720P"); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_resolution", http.StatusBadRequest)
	}
	if req.HasImage() {
		info.Action = constant.TaskActionGenerate
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/video/generations", strings.TrimRight(a.baseURL, "/")), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_request_body_failed")
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "read_body_bytes_failed")
	}
	var body map[string]any
	if err := common.Unmarshal(cachedBody, &body); err != nil {
		return bytes.NewReader(cachedBody), nil
	}
	if info.UpstreamModelName != "" {
		body["model"] = info.UpstreamModelName
	}
	newBody, err := common.Marshal(body)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_request_body_failed")
	}
	return bytes.NewReader(newBody), nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if info == nil || !billing_setting.IsPerSecondBilling(info.OriginModelName) {
		return nil
	}
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	resolution, err := taskcommon.ResolveVideoBillingResolution(taskReq, "720P")
	if err != nil {
		return nil
	}
	return map[string]float64{
		"seconds":                                float64(taskcommon.ResolveVideoBillingDuration(taskReq, 5)),
		fmt.Sprintf("resolution-%s", resolution): 1,
	}
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	task, err := parseNewApiTask(responseBody)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if task.TaskID == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
	}

	upstreamTaskID := task.TaskID
	task.TaskID = info.PublicTaskID
	task.Properties.OriginModelName = info.OriginModelName
	taskData, err = common.Marshal(task)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "marshal_task_data_failed", http.StatusInternalServerError)
	}

	c.JSON(http.StatusOK, dto.TaskResponse[any]{
		Code: "success",
		Data: relayTaskToResponse(task),
	})
	return upstreamTaskID, taskData, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}
	uri := fmt.Sprintf("%s/v1/video/generations/%s", strings.TrimRight(baseUrl, "/"), taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	task, err := parseNewApiTask(respBody)
	if err != nil {
		return nil, err
	}
	return &relaycommon.TaskInfo{
		TaskID:   task.TaskID,
		Status:   string(task.Status),
		Reason:   task.FailReason,
		Url:      task.GetResultURL(),
		Progress: task.Progress,
	}, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return append([]string{}, ModelList...)
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	return common.Marshal(task.ToOpenAIVideo())
}

func parseNewApiTask(body []byte) (*model.Task, error) {
	var wrappedDto dto.TaskResponse[dto.TaskDto]
	if err := common.Unmarshal(body, &wrappedDto); err == nil && wrappedDto.IsSuccess() && validTaskDto(wrappedDto.Data) {
		return taskDtoToModelTask(wrappedDto.Data), nil
	}
	var wrapped dto.TaskResponse[model.Task]
	if err := common.Unmarshal(body, &wrapped); err == nil && wrapped.IsSuccess() && validModelTask(&wrapped.Data) {
		return &wrapped.Data, nil
	}
	var wrappedVideo dto.TaskResponse[dto.OpenAIVideo]
	if err := common.Unmarshal(body, &wrappedVideo); err == nil && wrappedVideo.IsSuccess() && validOpenAIVideo(wrappedVideo.Data) {
		return openAIVideoToModelTask(wrappedVideo.Data), nil
	}
	var video dto.OpenAIVideo
	if err := common.Unmarshal(body, &video); err == nil && validOpenAIVideo(video) {
		return openAIVideoToModelTask(video), nil
	}
	var task model.Task
	if err := common.Unmarshal(body, &task); err == nil && validModelTask(&task) {
		return &task, nil
	}
	return nil, errors.New("unmarshal newapi task failed")
}

func validTaskDto(taskDto dto.TaskDto) bool {
	return strings.TrimSpace(taskDto.TaskID) != "" && validInternalTaskStatus(taskDto.Status)
}

func validModelTask(task *model.Task) bool {
	return task != nil && strings.TrimSpace(task.TaskID) != "" && validInternalTaskStatus(string(task.Status))
}

func validInternalTaskStatus(status string) bool {
	switch model.TaskStatus(status) {
	case "", model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued,
		model.TaskStatusInProgress, model.TaskStatusFailure, model.TaskStatusSuccess,
		model.TaskStatusUnknown:
		return true
	default:
		return false
	}
}

func validOpenAIVideo(video dto.OpenAIVideo) bool {
	if strings.TrimSpace(video.TaskID) == "" && strings.TrimSpace(video.ID) == "" {
		return false
	}
	switch video.Status {
	case dto.VideoStatusUnknown, dto.VideoStatusQueued, dto.VideoStatusInProgress,
		dto.VideoStatusCompleted, dto.VideoStatusFailed:
		return true
	default:
		return false
	}
}

func openAIVideoToModelTask(video dto.OpenAIVideo) *model.Task {
	task := &model.Task{}
	task.TaskID = video.TaskID
	if task.TaskID == "" {
		task.TaskID = video.ID
	}
	task.Status = videoStatusToTaskStatus(video.Status)
	task.Progress = fmt.Sprintf("%d%%", video.Progress)
	task.CreatedAt = video.CreatedAt
	task.UpdatedAt = video.CompletedAt
	task.Properties.OriginModelName = video.Model
	if video.Error != nil {
		task.FailReason = video.Error.Message
	}
	if url, ok := video.Metadata["url"].(string); ok {
		task.PrivateData.ResultURL = url
	}
	return task
}

func taskDtoToModelTask(taskDto dto.TaskDto) *model.Task {
	task := &model.Task{
		ID:         taskDto.ID,
		CreatedAt:  taskDto.CreatedAt,
		UpdatedAt:  taskDto.UpdatedAt,
		TaskID:     taskDto.TaskID,
		Platform:   constant.TaskPlatform(taskDto.Platform),
		UserId:     taskDto.UserId,
		Group:      taskDto.Group,
		ChannelId:  taskDto.ChannelId,
		Quota:      taskDto.Quota,
		Action:     taskDto.Action,
		Status:     model.TaskStatus(taskDto.Status),
		FailReason: taskDto.FailReason,
		SubmitTime: taskDto.SubmitTime,
		StartTime:  taskDto.StartTime,
		FinishTime: taskDto.FinishTime,
		Progress:   taskDto.Progress,
		Data:       taskDto.Data,
		PrivateData: model.TaskPrivateData{
			ResultURL: taskDto.ResultURL,
		},
	}
	if taskDto.Properties != nil {
		if propertiesBytes, err := common.Marshal(taskDto.Properties); err == nil {
			_ = common.Unmarshal(propertiesBytes, &task.Properties)
		}
	}
	return task
}

func relayTaskToResponse(task *model.Task) map[string]any {
	return map[string]any{
		"task_id":     task.TaskID,
		"status":      string(task.Status),
		"progress":    task.Progress,
		"result_url":  task.GetResultURL(),
		"fail_reason": task.FailReason,
		"created_at":  task.CreatedAt,
		"updated_at":  task.UpdatedAt,
		"properties":  task.Properties,
	}
}

func videoStatusToTaskStatus(status string) model.TaskStatus {
	switch status {
	case dto.VideoStatusCompleted:
		return model.TaskStatusSuccess
	case dto.VideoStatusFailed:
		return model.TaskStatusFailure
	case dto.VideoStatusInProgress:
		return model.TaskStatusInProgress
	case dto.VideoStatusQueued:
		return model.TaskStatusQueued
	default:
		return model.TaskStatusSubmitted
	}
}

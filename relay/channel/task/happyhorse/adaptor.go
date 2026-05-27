package happyhorse

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
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

const videoSynthesisPath = "/api/v1/services/aigc/video-generation/video-synthesis"

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

type videoResponse struct {
	Output    videoOutput `json:"output"`
	RequestID string      `json:"request_id"`
	Code      string      `json:"code,omitempty"`
	Message   string      `json:"message,omitempty"`
}

type videoOutput struct {
	TaskID     string `json:"task_id"`
	TaskStatus string `json:"task_status"`
	VideoURL   string `json:"video_url,omitempty"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s%s", strings.TrimRight(a.baseURL, "/"), videoSynthesisPath), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")
	req.Header.Set("X-DashScope-OssResourceResolve", "enable")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_task_request_failed")
	}
	req, err := convertToRequest(info, taskReq)
	if err != nil {
		return nil, errors.Wrap(err, "convert_to_happyhorse_request_failed")
	}
	bodyBytes, err := common.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_happyhorse_request_failed")
	}
	return bytes.NewReader(bodyBytes), nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if info == nil || !billing_setting.IsPerSecondBilling(info.OriginModelName) {
		return nil
	}
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	req, err := convertToRequest(info, taskReq)
	if err != nil {
		return nil
	}
	seconds := numberFromMap(req.Parameters, "duration")
	if seconds <= 0 {
		seconds = 5
	}
	otherRatios := map[string]float64{"seconds": float64(seconds)}
	resolution := normalizeResolution(stringFromMap(req.Parameters, "resolution"))
	if resolution == "" {
		resolution = "1080P"
	}
	if resolution != "" {
		key := fmt.Sprintf("resolution-%s", resolution)
		if resolution == "1080P" {
			otherRatios[key] = 1.6 / 0.9
		} else {
			otherRatios[key] = 1
		}
	}
	applyConfiguredMultiplierFallbacks(info, otherRatios)
	return otherRatios
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

	var upstreamResp videoResponse
	if err := common.Unmarshal(responseBody, &upstreamResp); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if upstreamResp.Code != "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("%s: %s", upstreamResp.Code, upstreamResp.Message), "happyhorse_api_error", resp.StatusCode)
	}
	if upstreamResp.Output.TaskID == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
	}

	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = info.PublicTaskID
	openAIResp.TaskID = info.PublicTaskID
	openAIResp.Model = info.OriginModelName
	openAIResp.Status = convertStatus(upstreamResp.Output.TaskStatus)
	openAIResp.CreatedAt = common.GetTimestamp()
	c.JSON(http.StatusOK, openAIResp)

	return upstreamResp.Output.TaskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}
	uri := fmt.Sprintf("%s/api/v1/tasks/%s", strings.TrimRight(baseUrl, "/"), taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return append([]string{}, ModelList...)
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var upstreamResp videoResponse
	if err := common.Unmarshal(respBody, &upstreamResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}
	taskResult := relaycommon.TaskInfo{Code: 0}
	switch upstreamResp.Output.TaskStatus {
	case "PENDING":
		taskResult.Status = model.TaskStatusQueued
	case "RUNNING":
		taskResult.Status = model.TaskStatusInProgress
	case "SUCCEEDED":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Url = upstreamResp.Output.VideoURL
	case "FAILED", "CANCELED", "UNKNOWN":
		taskResult.Status = model.TaskStatusFailure
		if upstreamResp.Message != "" {
			taskResult.Reason = upstreamResp.Message
		} else if upstreamResp.Output.Message != "" {
			taskResult.Reason = fmt.Sprintf("task failed, code: %s , message: %s", upstreamResp.Output.Code, upstreamResp.Output.Message)
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusQueued
	}
	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	var upstreamResp videoResponse
	if err := common.Unmarshal(task.Data, &upstreamResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal happyhorse response failed")
	}
	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = task.TaskID
	openAIResp.Status = convertStatus(upstreamResp.Output.TaskStatus)
	openAIResp.Model = task.Properties.OriginModelName
	openAIResp.SetProgressStr(task.Progress)
	openAIResp.CreatedAt = task.CreatedAt
	openAIResp.CompletedAt = task.UpdatedAt
	openAIResp.SetMetadata("url", upstreamResp.Output.VideoURL)
	if upstreamResp.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{Code: upstreamResp.Code, Message: upstreamResp.Message}
	} else if upstreamResp.Output.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{Code: upstreamResp.Output.Code, Message: upstreamResp.Output.Message}
	}
	return common.Marshal(openAIResp)
}

type happyHorseRequest struct {
	Model      string         `json:"model"`
	Input      map[string]any `json:"input"`
	Parameters map[string]any `json:"parameters"`
}

func convertToRequest(info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) (*happyHorseRequest, error) {
	upstreamModel := req.Model
	if info != nil && info.IsModelMapped {
		upstreamModel = info.UpstreamModelName
	}
	if upstreamModel == "" && info != nil {
		upstreamModel = info.UpstreamModelName
	}
	if upstreamModel == "" {
		return nil, fmt.Errorf("model is required")
	}
	if _, err := validatedDuration(req); err != nil {
		return nil, err
	}

	out := &happyHorseRequest{
		Model:      upstreamModel,
		Input:      map[string]any{},
		Parameters: map[string]any{},
	}
	if req.Prompt != "" {
		out.Input["prompt"] = req.Prompt
	}
	if req.Mode != "" {
		out.Parameters["mode"] = req.Mode
	}
	if req.Size != "" {
		applySize(req.Size, out.Parameters)
	} else {
		out.Parameters["resolution"] = "1080P"
	}
	if strings.Contains(upstreamModel, "-t2v") || strings.Contains(upstreamModel, "-r2v") {
		out.Parameters["ratio"] = "16:9"
	}
	if duration, err := durationFromRequest(req); err != nil {
		return nil, err
	} else {
		out.Parameters["duration"] = duration
	}
	out.Parameters["prompt_extend"] = true
	out.Parameters["watermark"] = false

	if images := imagesFromRequest(req); len(images) > 0 && !requestHasMedia(req) {
		out.Input["media"] = requestMedia(upstreamModel, images)
	}
	if req.Metadata != nil {
		if err := applyMetadata(req.Metadata, out); err != nil {
			return nil, err
		}
	}
	if req.Input != nil {
		mergeMap(out.Input, req.Input)
	}
	if req.Parameters != nil {
		mergeMap(out.Parameters, req.Parameters)
	}
	if modelValue, ok := stringMapValue(out.Input, "model"); ok && modelValue != "" && modelValue != upstreamModel {
		return nil, errors.New("can't change model with input")
	}
	if modelValue, ok := stringMapValue(out.Parameters, "model"); ok && modelValue != "" && modelValue != upstreamModel {
		return nil, errors.New("can't change model with parameters")
	}
	delete(out.Input, "model")
	delete(out.Parameters, "model")
	return out, nil
}

func applyMetadata(metadata map[string]any, out *happyHorseRequest) error {
	if modelValue, ok := stringMapValue(metadata, "model"); ok && modelValue != "" && modelValue != out.Model {
		return errors.New("can't change model with metadata")
	}
	if input, ok := mapValue(metadata, "input"); ok {
		mergeMap(out.Input, input)
	}
	if parameters, ok := mapValue(metadata, "parameters"); ok {
		mergeMap(out.Parameters, parameters)
	}
	copyString(metadata, out.Input, "audio_url")
	copyString(metadata, out.Input, "img_url")
	copyString(metadata, out.Input, "image_url")
	copyString(metadata, out.Input, "first_frame_url")
	copyString(metadata, out.Input, "last_frame_url")
	copyString(metadata, out.Input, "negative_prompt")
	copyString(metadata, out.Input, "template")
	copyAny(metadata, out.Input, "media")
	copyAny(metadata, out.Input, "multi_prompt")
	copyAny(metadata, out.Input, "element_list")
	copyAny(metadata, out.Input, "reference_voice")
	copyString(metadata, out.Parameters, "resolution")
	copyString(metadata, out.Parameters, "size")
	copyAny(metadata, out.Parameters, "prompt_extend")
	copyAny(metadata, out.Parameters, "watermark")
	copyAny(metadata, out.Parameters, "audio")
	copyAny(metadata, out.Parameters, "seed")
	copyString(metadata, out.Parameters, "ratio")
	copyString(metadata, out.Parameters, "aspect_ratio")
	copyString(metadata, out.Parameters, "mode")
	copyString(metadata, out.Parameters, "shot_type")
	copyAny(metadata, out.Parameters, "audio_setting")
	if duration, err := durationFromAny(metadata["duration"]); err != nil {
		return err
	} else if duration > 0 {
		out.Parameters["duration"] = duration
	}
	if videoURL, ok := stringMapValue(metadata, "video_url"); ok && videoURL != "" {
		media := mediaFromAny(out.Input["media"])
		media = append(media, map[string]any{"type": mediaTypeForURL(out.Model, videoURL), "url": videoURL})
		out.Input["media"] = media
	}
	return nil
}

func applySize(size string, params map[string]any) {
	if strings.Contains(size, "*") || strings.Contains(size, "x") {
		params["size"] = strings.ReplaceAll(size, "x", "*")
		return
	}
	params["resolution"] = normalizeResolution(size)
}

func durationFromRequest(req relaycommon.TaskSubmitReq) (int, error) {
	if duration, err := validatedDuration(req); err != nil || duration > 0 {
		return duration, err
	}
	return 5, nil
}

func validatedDuration(req relaycommon.TaskSubmitReq) (int, error) {
	var values []int
	if req.Duration > 0 {
		values = append(values, req.Duration)
	}
	if req.Seconds != "" {
		seconds, err := parsePositiveInt(req.Seconds)
		if err != nil {
			return 0, errors.Wrap(err, "convert seconds to int failed")
		}
		if seconds > 0 {
			values = append(values, seconds)
		}
	}
	for _, source := range []map[string]any{req.Parameters, req.Metadata} {
		if duration, exists, err := durationFromMap(source); err != nil {
			return 0, err
		} else if exists {
			values = append(values, duration)
		}
	}
	if nested, ok := mapValue(req.Metadata, "parameters"); ok {
		if duration, exists, err := durationFromMap(nested); err != nil {
			return 0, err
		} else if exists {
			values = append(values, duration)
		}
	}
	if len(values) == 0 {
		return 0, nil
	}
	expected := values[0]
	for _, value := range values[1:] {
		if value != expected {
			return 0, fmt.Errorf("duration mismatch")
		}
	}
	return expected, nil
}

func durationFromMap(values map[string]any) (int, bool, error) {
	if values == nil {
		return 0, false, nil
	}
	value, ok := values["duration"]
	if !ok || value == nil {
		return 0, false, nil
	}
	duration, err := durationFromAny(value)
	return duration, duration > 0, err
}

func durationFromAny(value any) (int, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case int:
		return positiveInt(v), nil
	case int64:
		return positiveInt(int(v)), nil
	case float64:
		return positiveInt(int(math.Ceil(v))), nil
	case float32:
		return positiveInt(int(math.Ceil(float64(v)))), nil
	case string:
		return parsePositiveInt(v)
	default:
		return 0, nil
	}
}

func parsePositiveInt(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return positiveInt(int(math.Ceil(parsed))), nil
}

func positiveInt(value int) int {
	if value > 0 {
		return value
	}
	return 0
}

func imagesFromRequest(req relaycommon.TaskSubmitReq) []string {
	images := append([]string{}, req.Images...)
	if req.InputReference != "" && !contains(images, req.InputReference) {
		images = append(images, req.InputReference)
	}
	if req.Image != "" && !contains(images, req.Image) {
		images = append(images, req.Image)
	}
	return images
}

func requestHasMedia(req relaycommon.TaskSubmitReq) bool {
	if _, ok := req.Input["media"]; ok {
		return true
	}
	if _, ok := req.Metadata["media"]; ok {
		return true
	}
	if input, ok := mapValue(req.Metadata, "input"); ok {
		_, ok = input["media"]
		return ok
	}
	return false
}

func requestMedia(modelName string, urls []string) []map[string]any {
	media := make([]map[string]any, 0, len(urls))
	switch modelName {
	case "happyhorse-1.0-i2v":
		media = append(media, map[string]any{"type": "first_frame", "url": urls[0]})
	case "happyhorse-1.0-r2v":
		for _, url := range urls {
			media = append(media, map[string]any{"type": "reference_image", "url": url})
		}
	case "happyhorse-1.0-video-edit":
		for _, url := range urls {
			mediaType := "reference_image"
			if looksLikeVideoURL(url) {
				mediaType = "video"
			}
			media = append(media, map[string]any{"type": mediaType, "url": url})
		}
	}
	return media
}

func normalizeResolution(value string) string {
	resolution := strings.ToUpper(strings.TrimSpace(value))
	if resolution == "" {
		return ""
	}
	if !strings.HasSuffix(resolution, "P") {
		resolution += "P"
	}
	return resolution
}

func convertStatus(status string) string {
	switch status {
	case "PENDING":
		return dto.VideoStatusQueued
	case "RUNNING":
		return dto.VideoStatusInProgress
	case "SUCCEEDED":
		return dto.VideoStatusCompleted
	case "FAILED", "CANCELED", "UNKNOWN":
		return dto.VideoStatusFailed
	default:
		return dto.VideoStatusUnknown
	}
}

func applyConfiguredMultiplierFallbacks(info *relaycommon.RelayInfo, ratios map[string]float64) {
	if info == nil || !billing_setting.IsPerSecondBilling(info.OriginModelName) || len(ratios) == 0 {
		return
	}
	for key := range ratios {
		if value, ok := billing_setting.GetPerSecondMultiplier(info.OriginModelName, key); ok {
			ratios[key] = value
		}
	}
}

func mergeMap(target map[string]any, source map[string]any) {
	for key, value := range source {
		target[key] = value
	}
}

func mapValue(source map[string]any, key string) (map[string]any, bool) {
	if source == nil {
		return nil, false
	}
	value, ok := source[key].(map[string]any)
	return value, ok
}

func stringMapValue(source map[string]any, key string) (string, bool) {
	if source == nil {
		return "", false
	}
	value, ok := source[key].(string)
	return value, ok
}

func stringFromMap(source map[string]any, key string) string {
	value, _ := stringMapValue(source, key)
	return value
}

func numberFromMap(source map[string]any, key string) int {
	value, err := durationFromAny(source[key])
	if err != nil {
		return 0
	}
	return value
}

func copyString(source map[string]any, target map[string]any, key string) {
	if value, ok := stringMapValue(source, key); ok && value != "" {
		target[key] = value
	}
}

func copyAny(source map[string]any, target map[string]any, key string) {
	if source == nil {
		return
	}
	if value, ok := source[key]; ok {
		target[key] = value
	}
}

func mediaFromAny(value any) []map[string]any {
	if media, ok := value.([]map[string]any); ok {
		return media
	}
	if raw, ok := value.([]any); ok {
		media := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if m, ok := item.(map[string]any); ok {
				media = append(media, m)
			}
		}
		return media
	}
	return nil
}

func mediaTypeForURL(modelName, url string) string {
	if strings.Contains(modelName, "video-edit") {
		return "video"
	}
	return "video"
}

func looksLikeVideoURL(url string) bool {
	lower := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(lower, ".mp4") || strings.Contains(lower, ".mov") || strings.HasPrefix(lower, "data:video/")
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

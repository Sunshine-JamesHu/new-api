package ali

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const aliVideoSynthesisPath = "/api/v1/services/aigc/video-generation/video-synthesis"

// ============================
// Request / Response structures
// ============================

// AliVideoRequest 阿里通义万相视频生成请求
type AliVideoRequest struct {
	Model      string              `json:"model"`
	Input      AliVideoInput       `json:"input"`
	Parameters *AliVideoParameters `json:"parameters,omitempty"`
}

type aliVideoRequestV2 struct {
	Model      string                `json:"model"`
	Input      aliVideoInputV2       `json:"input"`
	Parameters *aliVideoParametersV2 `json:"parameters,omitempty"`
}

type aliVideoInputV2 struct {
	Prompt         string          `json:"prompt,omitempty"`
	ImgURL         string          `json:"img_url,omitempty"`
	FirstFrameURL  string          `json:"first_frame_url,omitempty"`
	LastFrameURL   string          `json:"last_frame_url,omitempty"`
	AudioURL       string          `json:"audio_url,omitempty"`
	NegativePrompt string          `json:"negative_prompt,omitempty"`
	Template       string          `json:"template,omitempty"`
	Media          []aliVideoMedia `json:"media,omitempty"`
	MultiPrompt    any             `json:"multi_prompt,omitempty"`
	ElementList    any             `json:"element_list,omitempty"`
	ReferenceVoice any             `json:"reference_voice,omitempty"`
}

type aliVideoMedia struct {
	Type              string `json:"type,omitempty"`
	URL               string `json:"url,omitempty"`
	ImageURL          string `json:"image_url,omitempty"`
	VideoURL          string `json:"video_url,omitempty"`
	AudioURL          string `json:"audio_url,omitempty"`
	KeepOriginalSound string `json:"keep_original_sound,omitempty"`
}

type aliVideoParametersV2 struct {
	Resolution   string `json:"resolution,omitempty"`
	Size         string `json:"size,omitempty"`
	Duration     int    `json:"duration,omitempty"`
	PromptExtend *bool  `json:"prompt_extend,omitempty"`
	Watermark    *bool  `json:"watermark,omitempty"`
	Audio        *bool  `json:"audio,omitempty"`
	Seed         int    `json:"seed,omitempty"`
	Ratio        string `json:"ratio,omitempty"`
	AspectRatio  string `json:"aspect_ratio,omitempty"`
	Mode         string `json:"mode,omitempty"`
	ShotType     string `json:"shot_type,omitempty"`
	AudioSetting any    `json:"audio_setting,omitempty"`
}

// AliVideoInput 视频输入参数
type AliVideoInput struct {
	Prompt         string `json:"prompt,omitempty"`          // 文本提示词
	ImgURL         string `json:"img_url,omitempty"`         // 首帧图像URL或Base64（图生视频）
	FirstFrameURL  string `json:"first_frame_url,omitempty"` // 首帧图片URL（首尾帧生视频）
	LastFrameURL   string `json:"last_frame_url,omitempty"`  // 尾帧图片URL（首尾帧生视频）
	AudioURL       string `json:"audio_url,omitempty"`       // 音频URL（wan2.5支持）
	NegativePrompt string `json:"negative_prompt,omitempty"` // 反向提示词
	Template       string `json:"template,omitempty"`        // 视频特效模板
}

// AliVideoParameters 视频参数
type AliVideoParameters struct {
	Resolution   string `json:"resolution,omitempty"`    // 分辨率: 480P/720P/1080P（图生视频、首尾帧生视频）
	Size         string `json:"size,omitempty"`          // 尺寸: 如 "832*480"（文生视频）
	Duration     int    `json:"duration,omitempty"`      // 时长: 3-10秒
	PromptExtend bool   `json:"prompt_extend,omitempty"` // 是否开启prompt智能改写
	Watermark    bool   `json:"watermark,omitempty"`     // 是否添加水印
	Audio        *bool  `json:"audio,omitempty"`         // 是否添加音频（wan2.5）
	Seed         int    `json:"seed,omitempty"`          // 随机数种子
}

// AliVideoResponse 阿里通义万相响应
type AliVideoResponse struct {
	Output    AliVideoOutput `json:"output"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code,omitempty"`
	Message   string         `json:"message,omitempty"`
	Usage     *AliUsage      `json:"usage,omitempty"`
}

// AliVideoOutput 输出信息
type AliVideoOutput struct {
	TaskID        string `json:"task_id"`
	TaskStatus    string `json:"task_status"`
	SubmitTime    string `json:"submit_time,omitempty"`
	ScheduledTime string `json:"scheduled_time,omitempty"`
	EndTime       string `json:"end_time,omitempty"`
	OrigPrompt    string `json:"orig_prompt,omitempty"`
	ActualPrompt  string `json:"actual_prompt,omitempty"`
	VideoURL      string `json:"video_url,omitempty"`
	Code          string `json:"code,omitempty"`
	Message       string `json:"message,omitempty"`
}

// AliUsage 使用统计
type AliUsage struct {
	Duration   dto.IntValue `json:"duration,omitempty"`
	VideoCount dto.IntValue `json:"video_count,omitempty"`
	SR         dto.IntValue `json:"SR,omitempty"`
}

type aliVideoMetadataV2 struct {
	Input          *aliVideoInputV2       `json:"input,omitempty"`
	Parameters     *aliVideoParametersV2  `json:"parameters,omitempty"`
	Model          string          `json:"model,omitempty"`
	AudioURL       string          `json:"audio_url,omitempty"`
	ImgURL         string          `json:"img_url,omitempty"`
	ImageURL       string          `json:"image_url,omitempty"`
	VideoURL       string          `json:"video_url,omitempty"`
	FirstFrameURL  string          `json:"first_frame_url,omitempty"`
	LastFrameURL   string          `json:"last_frame_url,omitempty"`
	NegativePrompt string          `json:"negative_prompt,omitempty"`
	Template       string          `json:"template,omitempty"`
	Media          []aliVideoMedia `json:"media,omitempty"`
	MultiPrompt    any             `json:"multi_prompt,omitempty"`
	ElementList    any             `json:"element_list,omitempty"`
	ReferenceVoice any             `json:"reference_voice,omitempty"`
	Resolution     string          `json:"resolution,omitempty"`
	Size           string          `json:"size,omitempty"`
	Duration       any             `json:"duration,omitempty"`
	PromptExtend   *bool           `json:"prompt_extend,omitempty"`
	Watermark      *bool           `json:"watermark,omitempty"`
	Audio          *bool           `json:"audio,omitempty"`
	Seed           int             `json:"seed,omitempty"`
	Ratio          string          `json:"ratio,omitempty"`
	AspectRatio    string          `json:"aspect_ratio,omitempty"`
	Mode           string          `json:"mode,omitempty"`
	ShotType       string          `json:"shot_type,omitempty"`
	AudioSetting   any             `json:"audio_setting,omitempty"`
}

type AliMetadata struct {
	// Input 相关
	AudioURL       string `json:"audio_url,omitempty"`       // 音频URL
	ImgURL         string `json:"img_url,omitempty"`         // 图片URL（图生视频）
	FirstFrameURL  string `json:"first_frame_url,omitempty"` // 首帧图片URL（首尾帧生视频）
	LastFrameURL   string `json:"last_frame_url,omitempty"`  // 尾帧图片URL（首尾帧生视频）
	NegativePrompt string `json:"negative_prompt,omitempty"` // 反向提示词
	Template       string `json:"template,omitempty"`        // 视频特效模板

	// Parameters 相关
	Resolution   *string `json:"resolution,omitempty"`    // 分辨率: 480P/720P/1080P
	Size         *string `json:"size,omitempty"`          // 尺寸: 如 "832*480"
	Duration     *int    `json:"duration,omitempty"`      // 时长
	PromptExtend *bool   `json:"prompt_extend,omitempty"` // 是否开启prompt智能改写
	Watermark    *bool   `json:"watermark,omitempty"`     // 是否添加水印
	Audio        *bool   `json:"audio,omitempty"`         // 是否添加音频
	Seed         *int    `json:"seed,omitempty"`          // 随机数种子
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// ValidateMultipartDirect 负责解析并将原始 TaskSubmitReq 存入 context
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s%s", a.baseURL, aliVideoSynthesisPath), nil
}

// BuildRequestHeader sets required headers for Ali API
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable") // 阿里异步任务必须设置
	if a.ChannelType == constant.ChannelTypeAliBailian {
		req.Header.Set("X-DashScope-OssResourceResolve", "enable")
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_task_request_failed")
	}

	var aliReq any
	if a.ChannelType == constant.ChannelTypeAliBailian {
		aliReq, err = a.convertToAliRequestV2(info, taskReq)
	} else {
		aliReq, err = a.convertToAliRequest(info, taskReq)
	}
	if err != nil {
		return nil, errors.Wrap(err, "convert_to_ali_request_failed")
	}
	logger.LogJson(c, "ali video request body", aliReq)

	bodyBytes, err := common.Marshal(aliReq)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_ali_request_failed")
	}
	return bytes.NewReader(bodyBytes), nil
}

var (
	size480p = []string{
		"832*480",
		"480*832",
		"624*624",
	}
	size720p = []string{
		"1280*720",
		"720*1280",
		"960*960",
		"1088*832",
		"832*1088",
	}
	size1080p = []string{
		"1920*1080",
		"1080*1920",
		"1440*1440",
		"1632*1248",
		"1248*1632",
	}
)

func sizeToResolution(size string) (string, error) {
	if lo.Contains(size480p, size) {
		return "480P", nil
	} else if lo.Contains(size720p, size) {
		return "720P", nil
	} else if lo.Contains(size1080p, size) {
		return "1080P", nil
	}
	return "", fmt.Errorf("invalid size: %s", size)
}

func ProcessAliOtherRatios(aliReq *aliVideoRequestV2) (map[string]float64, error) {
	otherRatios := make(map[string]float64)
	aliRatios := map[string]map[string]float64{
		"wan2.6-i2v": {
			"720P":  1,
			"1080P": 1 / 0.6,
		},
		"wan2.7-t2v": {
			"720P":  1,
			"1080P": 1.6 / 0.9,
		},
		"wan2.5-t2v-preview": {
			"480P":  1,
			"720P":  2,
			"1080P": 1 / 0.3,
		},
		"wan2.2-t2v-plus": {
			"480P":  1,
			"1080P": 0.7 / 0.14,
		},
		"wan2.5-i2v-preview": {
			"480P":  1,
			"720P":  2,
			"1080P": 1 / 0.3,
		},
		"wan2.2-i2v-plus": {
			"480P":  1,
			"1080P": 0.7 / 0.14,
		},
		"wan2.2-kf2v-flash": {
			"480P":  1,
			"720P":  2,
			"1080P": 4.8,
		},
		"wan2.2-i2v-flash": {
			"480P": 1,
			"720P": 2,
		},
		"wan2.2-s2v": {
			"480P": 1,
			"720P": 0.9 / 0.5,
		},
		"happyhorse-1.0-t2v": {
			"720P":  1,
			"1080P": 1.6 / 0.9,
		},
		"happyhorse-1.0-i2v": {
			"720P":  1,
			"1080P": 1.6 / 0.9,
		},
		"happyhorse-1.0-r2v": {
			"720P":  1,
			"1080P": 1.6 / 0.9,
		},
		"happyhorse-1.0-video-edit": {
			"720P":  1,
			"1080P": 1.6 / 0.9,
		},
		"kling/kling-v3-video-generation": {
			"720P":  1,
			"1080P": 2.8 / 1.4,
		},
		"kling/kling-v3-omni-video-generation": {
			"720P":  1,
			"1080P": 2.8 / 1.4,
		},
	}
	resolution := aliRequestResolution(aliReq)
	if otherRatio, ok := aliRatios[aliReq.Model]; ok {
		if ratio, ok := otherRatio[resolution]; ok {
			otherRatios[fmt.Sprintf("resolution-%s", resolution)] = ratio
		}
	}
	if resolution != "" {
		resolutionKey := fmt.Sprintf("resolution-%s", resolution)
		if _, exists := otherRatios[resolutionKey]; !exists {
			otherRatios[resolutionKey] = 1
		}
	}
	if isAliKlingModel(aliReq.Model) && aliReq.Parameters != nil && aliReq.Parameters.Audio != nil && *aliReq.Parameters.Audio {
		if resolution == "720P" {
			otherRatios["audio"] = 2.5 / 1.4
		} else {
			otherRatios["audio"] = 5.0 / 2.8
		}
	}
	if aliReq.Model == "kling/kling-v3-omni-video-generation" && aliHasReferenceVideo(aliReq.Input.Media) {
		otherRatios["reference_video"] = 5.0 / 2.8
	}
	return otherRatios, nil
}

func processLegacyAliOtherRatios(aliReq *AliVideoRequest) (map[string]float64, error) {
	if aliReq == nil || aliReq.Parameters == nil {
		return nil, nil
	}
	return ProcessAliOtherRatios(&aliVideoRequestV2{
		Model: aliReq.Model,
		Parameters: &aliVideoParametersV2{
			Resolution: aliReq.Parameters.Resolution,
			Size:       aliReq.Parameters.Size,
			Duration:   aliReq.Parameters.Duration,
			Audio:      aliReq.Parameters.Audio,
		},
	})
}

func (a *TaskAdaptor) convertToAliRequest(info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) (*AliVideoRequest, error) {
	upstreamModel := req.Model
	if info.IsModelMapped {
		upstreamModel = info.UpstreamModelName
	}
	aliReq := &AliVideoRequest{
		Model: upstreamModel,
		Input: AliVideoInput{
			Prompt: req.Prompt,
			ImgURL: req.InputReference,
		},
		Parameters: &AliVideoParameters{
			PromptExtend: true, // 默认开启智能改写
			Watermark:    false,
		},
	}

	// 处理分辨率映射
	if req.Size != "" {
		// text to video size must be contained *
		if strings.Contains(req.Model, "t2v") && !strings.Contains(req.Size, "*") {
			return nil, fmt.Errorf("invalid size: %s, example: %s", req.Size, "1920*1080")
		}
		if strings.Contains(req.Size, "*") {
			aliReq.Parameters.Size = req.Size
		} else {
			resolution := strings.ToUpper(req.Size)
			// 支持 480p, 720p, 1080p 或 480P, 720P, 1080P
			if !strings.HasSuffix(resolution, "P") {
				resolution = resolution + "P"
			}
			aliReq.Parameters.Resolution = resolution
		}
	} else {
		// 根据模型设置默认分辨率
		if strings.Contains(req.Model, "t2v") { // image to video
			if strings.HasPrefix(req.Model, "wan2.5") {
				aliReq.Parameters.Size = "1920*1080"
			} else if strings.HasPrefix(req.Model, "wan2.2") {
				aliReq.Parameters.Size = "1920*1080"
			} else {
				aliReq.Parameters.Size = "1280*720"
			}
		} else {
			if strings.HasPrefix(req.Model, "wan2.6") {
				aliReq.Parameters.Resolution = "1080P"
			} else if strings.HasPrefix(req.Model, "wan2.5") {
				aliReq.Parameters.Resolution = "1080P"
			} else if strings.HasPrefix(req.Model, "wan2.2-i2v-flash") {
				aliReq.Parameters.Resolution = "720P"
			} else if strings.HasPrefix(req.Model, "wan2.2-i2v-plus") {
				aliReq.Parameters.Resolution = "1080P"
			} else {
				aliReq.Parameters.Resolution = "720P"
			}
		}
	}

	// 处理时长
	if req.Duration > 0 {
		aliReq.Parameters.Duration = req.Duration
	} else if req.Seconds != "" {
		seconds, err := parseAliPositiveInt(req.Seconds)
		if err != nil {
			return nil, errors.Wrap(err, "convert seconds to int failed")
		} else {
			aliReq.Parameters.Duration = seconds
		}
	} else {
		aliReq.Parameters.Duration = 5 // 默认5秒
	}

	// 从 metadata 中提取额外参数
	if req.Metadata != nil {
		if metadataBytes, err := common.Marshal(req.Metadata); err == nil {
			err = common.Unmarshal(metadataBytes, aliReq)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal metadata failed")
			}
		} else {
			return nil, errors.Wrap(err, "marshal metadata failed")
		}
	}

	if aliReq.Model != upstreamModel {
		return nil, errors.New("can't change model with metadata")
	}

	return aliReq, nil
}

func (a *TaskAdaptor) convertToAliRequestV2(info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) (*aliVideoRequestV2, error) {
	upstreamModel := req.Model
	if info.IsModelMapped {
		upstreamModel = info.UpstreamModelName
	}
	if upstreamModel == "" {
		upstreamModel = info.UpstreamModelName
	}
	aliReq := &aliVideoRequestV2{
		Model: upstreamModel,
		Input: aliVideoInputV2{
			Prompt: req.Prompt,
		},
		Parameters: &aliVideoParametersV2{
			PromptExtend: lo.ToPtr(true),
			Watermark:    lo.ToPtr(false),
		},
	}

	if _, err := aliValidatedRequestDuration(req); err != nil {
		return nil, err
	}

	images := aliImagesFromRequest(req)
	if len(images) > 0 && !aliTaskRequestHasMedia(req) {
		if isAliNewFormatModel(upstreamModel) {
			applyAliRequestMedia(upstreamModel, images, aliReq)
		} else {
			aliReq.Input.ImgURL = images[0]
			aliReq.Input.FirstFrameURL = images[0]
			if len(images) > 1 {
				aliReq.Input.LastFrameURL = images[1]
			}
		}
	}
	if req.Mode != "" {
		aliReq.Parameters.Mode = req.Mode
	}
	if req.Size != "" {
		applyAliSize(upstreamModel, req.Size, aliReq.Parameters)
	} else {
		applyAliDefaultSize(upstreamModel, aliReq.Parameters)
	}

	duration, err := aliDurationFromRequest(req)
	if err != nil {
		return nil, err
	}
	aliReq.Parameters.Duration = duration
	if req.Metadata != nil {
		var metadata aliVideoMetadataV2
		metadataBytes, err := common.Marshal(req.Metadata)
		if err != nil {
			return nil, errors.Wrap(err, "marshal metadata failed")
		}
		if err := common.Unmarshal(metadataBytes, &metadata); err != nil {
			return nil, errors.Wrap(err, "unmarshal metadata failed")
		}
		if metadata.Model != "" && metadata.Model != upstreamModel {
			return nil, errors.New("can't change model with metadata")
		}
		if err := applyAliMetadata(&metadata, aliReq); err != nil {
			return nil, err
		}
	}
	if req.Input != nil {
		var input aliVideoInputV2
		inputBytes, err := common.Marshal(req.Input)
		if err != nil {
			return nil, errors.Wrap(err, "marshal input failed")
		}
		if err := common.Unmarshal(inputBytes, &input); err != nil {
			return nil, errors.Wrap(err, "unmarshal input failed")
		}
		applyAliInputOverride(&input, &aliReq.Input)
	}
	if req.Parameters != nil {
		var parameters aliVideoParametersV2
		parametersBytes, err := common.Marshal(req.Parameters)
		if err != nil {
			return nil, errors.Wrap(err, "marshal parameters failed")
		}
		if err := common.Unmarshal(parametersBytes, &parameters); err != nil {
			return nil, errors.Wrap(err, "unmarshal parameters failed")
		}
		applyAliParameterOverride(&parameters, aliReq.Parameters)
	}

	if aliReq.Model != upstreamModel {
		return nil, errors.New("can't change model with metadata")
	}
	if isAliNewFormatModel(upstreamModel) {
		applyAliNewFormatDefaults(upstreamModel, aliReq)
		ensureAliNewFormatMedia(upstreamModel, aliReq)
		aliReq.Input.Media = normalizeAliVideoMedia(aliReq.Input.Media)
	}

	return aliReq, nil
}

func aliTaskRequestHasMedia(req relaycommon.TaskSubmitReq) bool {
	if req.Input != nil {
		if value, ok := req.Input["media"]; ok && value != nil {
			return true
		}
	}
	return aliMetadataHasMedia(req.Metadata)
}

func aliMetadataHasMedia(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	if value, ok := metadata["media"]; ok && value != nil {
		return true
	}
	if input, ok := metadata["input"].(map[string]interface{}); ok {
		if value, ok := input["media"]; ok && value != nil {
			return true
		}
	}
	return false
}

func aliImagesFromRequest(req relaycommon.TaskSubmitReq) []string {
	images := append([]string{}, req.Images...)
	if req.InputReference != "" && !lo.Contains(images, req.InputReference) {
		images = append(images, req.InputReference)
	}
	if req.Image != "" && !lo.Contains(images, req.Image) {
		images = append(images, req.Image)
	}
	return images
}

func aliDurationFromRequest(req relaycommon.TaskSubmitReq) (int, error) {
	if duration, err := aliValidatedRequestDuration(req); err != nil || duration > 0 {
		return duration, err
	}
	return 5, nil
}

func aliValidatedRequestDuration(req relaycommon.TaskSubmitReq) (int, error) {
	var durations []struct {
		source string
		value  int
	}
	if req.Duration > 0 {
		durations = append(durations, struct {
			source string
			value  int
		}{"duration", req.Duration})
	}
	if req.Seconds != "" {
		seconds, err := parseAliPositiveInt(req.Seconds)
		if err != nil {
			return 0, errors.Wrap(err, "convert seconds to int failed")
		}
		if seconds > 0 {
			durations = append(durations, struct {
				source string
				value  int
			}{"seconds", seconds})
		}
	}
	if duration, exists, err := aliDurationFromMap(req.Parameters); err != nil {
		return 0, errors.Wrap(err, "convert parameters.duration to int failed")
	} else if exists {
		durations = append(durations, struct {
			source string
			value  int
		}{"parameters.duration", duration})
	}
	if duration, exists, err := aliNestedDurationFromMap(req.Metadata, "parameters"); err != nil {
		return 0, errors.Wrap(err, "convert metadata.parameters.duration to int failed")
	} else if exists {
		durations = append(durations, struct {
			source string
			value  int
		}{"metadata.parameters.duration", duration})
	}
	if duration, exists, err := aliDurationFromMap(req.Metadata); err != nil {
		return 0, errors.Wrap(err, "convert metadata.duration to int failed")
	} else if exists {
		durations = append(durations, struct {
			source string
			value  int
		}{"metadata.duration", duration})
	}
	if len(durations) == 0 {
		return 0, nil
	}
	expected := durations[0]
	for _, duration := range durations[1:] {
		if duration.value != expected.value {
			return 0, fmt.Errorf("duration mismatch: %s=%d, %s=%d", expected.source, expected.value, duration.source, duration.value)
		}
	}
	return expected.value, nil
}

func aliNestedDurationFromMap(values map[string]interface{}, key string) (int, bool, error) {
	if values == nil {
		return 0, false, nil
	}
	nested, ok := values[key].(map[string]interface{})
	if !ok {
		return 0, false, nil
	}
	return aliDurationFromMap(nested)
}

func aliDurationFromMap(values map[string]interface{}) (int, bool, error) {
	if values == nil {
		return 0, false, nil
	}
	value, ok := values["duration"]
	if !ok || value == nil {
		return 0, false, nil
	}
	duration, err := aliDurationFromAny(value)
	if err != nil {
		return 0, true, err
	}
	return duration, duration > 0, nil
}

func applyAliSize(modelName, size string, params *aliVideoParametersV2) {
	if isAliKlingModel(modelName) {
		resolution := normalizeAliResolution(size)
		if resolution == "720P" || resolution == "1080P" {
			params.Mode = aliModeFromResolution(resolution)
			params.Resolution = ""
			return
		}
		if aspectRatio := aliAspectRatioFromSize(size); aspectRatio != "" {
			params.AspectRatio = aspectRatio
			return
		}
	}
	if strings.Contains(size, "*") || strings.Contains(size, "x") {
		params.Size = strings.ReplaceAll(size, "x", "*")
		return
	}
	resolution := normalizeAliResolution(size)
	params.Resolution = resolution
}

func applyAliDefaultSize(modelName string, params *aliVideoParametersV2) {
	if strings.Contains(modelName, "t2v") && !isAliNewFormatModel(modelName) {
		if strings.HasPrefix(modelName, "wan2.5") || strings.HasPrefix(modelName, "wan2.2") {
			params.Size = "1920*1080"
		} else {
			params.Size = "1280*720"
		}
		return
	}
	params.Resolution = defaultAliResolution(modelName)
}

func applyAliNewFormatDefaults(modelName string, aliReq *aliVideoRequestV2) {
	aliReq.Parameters.Size = ""
	aliReq.Parameters.PromptExtend = nil
	if isAliKlingModel(modelName) {
		if aliReq.Parameters.Mode == "" && aliReq.Parameters.Resolution != "" {
			aliReq.Parameters.Mode = aliModeFromResolution(aliReq.Parameters.Resolution)
		}
		if aliReq.Parameters.Mode == "" {
			aliReq.Parameters.Mode = "pro"
		}
		if aliReq.Parameters.AspectRatio == "" && aliReq.Parameters.Ratio != "" {
			aliReq.Parameters.AspectRatio = aliReq.Parameters.Ratio
		}
		if aliReq.Parameters.AspectRatio == "" && aliKlingNeedsAspectRatio(aliReq) {
			aliReq.Parameters.AspectRatio = "16:9"
		}
		aliReq.Parameters.Resolution = ""
		aliReq.Parameters.Ratio = ""
		return
	}
	if aliReq.Parameters.Resolution == "" {
		aliReq.Parameters.Resolution = defaultAliResolution(modelName)
	}
	if aliReq.Parameters.Ratio == "" && aliReq.Parameters.AspectRatio != "" {
		aliReq.Parameters.Ratio = aliReq.Parameters.AspectRatio
		aliReq.Parameters.AspectRatio = ""
	}
	if aliReq.Parameters.Ratio == "" && (strings.Contains(modelName, "-t2v") || strings.Contains(modelName, "-r2v")) {
		aliReq.Parameters.Ratio = "16:9"
	}
}

func applyAliMetadata(metadata *aliVideoMetadataV2, aliReq *aliVideoRequestV2) error {
	if metadata.Input != nil {
		applyAliInputOverride(metadata.Input, &aliReq.Input)
	}
	if metadata.Parameters != nil {
		applyAliParameterOverride(metadata.Parameters, aliReq.Parameters)
	}
	if metadata.ImgURL != "" {
		aliReq.Input.ImgURL = metadata.ImgURL
		aliReq.Input.FirstFrameURL = metadata.ImgURL
	}
	if metadata.ImageURL != "" {
		aliReq.Input.ImgURL = metadata.ImageURL
		aliReq.Input.FirstFrameURL = metadata.ImageURL
	}
	if metadata.FirstFrameURL != "" {
		aliReq.Input.FirstFrameURL = metadata.FirstFrameURL
	}
	if metadata.LastFrameURL != "" {
		aliReq.Input.LastFrameURL = metadata.LastFrameURL
	}
	if metadata.AudioURL != "" {
		aliReq.Input.AudioURL = metadata.AudioURL
	}
	if metadata.NegativePrompt != "" {
		aliReq.Input.NegativePrompt = metadata.NegativePrompt
	}
	if metadata.Template != "" {
		aliReq.Input.Template = metadata.Template
	}
	if len(metadata.Media) > 0 {
		aliReq.Input.Media = normalizeAliVideoMedia(metadata.Media)
	}
	if metadata.MultiPrompt != nil {
		aliReq.Input.MultiPrompt = metadata.MultiPrompt
	}
	if metadata.ElementList != nil {
		aliReq.Input.ElementList = metadata.ElementList
	}
	if metadata.ReferenceVoice != nil {
		aliReq.Input.ReferenceVoice = metadata.ReferenceVoice
	}
	if metadata.Resolution != "" {
		if isAliKlingModel(aliReq.Model) {
			applyAliSize(aliReq.Model, metadata.Resolution, aliReq.Parameters)
		} else {
			aliReq.Parameters.Resolution = normalizeAliResolution(metadata.Resolution)
			aliReq.Parameters.Size = ""
		}
	}
	if metadata.Size != "" {
		applyAliSize(aliReq.Model, metadata.Size, aliReq.Parameters)
	}
	if duration, err := aliDurationFromAny(metadata.Duration); err != nil {
		return err
	} else if duration > 0 {
		aliReq.Parameters.Duration = duration
	}
	if metadata.PromptExtend != nil {
		aliReq.Parameters.PromptExtend = metadata.PromptExtend
	}
	if metadata.Watermark != nil {
		aliReq.Parameters.Watermark = metadata.Watermark
	}
	if metadata.Audio != nil {
		aliReq.Parameters.Audio = metadata.Audio
	}
	if metadata.Seed > 0 {
		aliReq.Parameters.Seed = metadata.Seed
	}
	if metadata.Ratio != "" {
		aliReq.Parameters.Ratio = metadata.Ratio
	}
	if metadata.AspectRatio != "" {
		aliReq.Parameters.AspectRatio = metadata.AspectRatio
	}
	if metadata.Mode != "" {
		aliReq.Parameters.Mode = metadata.Mode
	}
	if metadata.ShotType != "" {
		aliReq.Parameters.ShotType = metadata.ShotType
	}
	if metadata.AudioSetting != nil {
		aliReq.Parameters.AudioSetting = metadata.AudioSetting
	}
	if metadata.VideoURL != "" {
		aliReq.Input.Media = append(aliReq.Input.Media, newAliVideoMedia(aliVideoTypeForURL(aliReq.Model, metadata.VideoURL), metadata.VideoURL))
	}
	if isAliNewFormatModel(aliReq.Model) {
		aliReq.Input.Media = normalizeAliVideoMedia(aliReq.Input.Media)
	}
	return nil
}

func applyAliInputOverride(source *aliVideoInputV2, target *aliVideoInputV2) {
	if source.Prompt != "" {
		target.Prompt = source.Prompt
	}
	if source.ImgURL != "" {
		target.ImgURL = source.ImgURL
	}
	if source.FirstFrameURL != "" {
		target.FirstFrameURL = source.FirstFrameURL
	}
	if source.LastFrameURL != "" {
		target.LastFrameURL = source.LastFrameURL
	}
	if source.AudioURL != "" {
		target.AudioURL = source.AudioURL
	}
	if source.NegativePrompt != "" {
		target.NegativePrompt = source.NegativePrompt
	}
	if source.Template != "" {
		target.Template = source.Template
	}
	if len(source.Media) > 0 {
		target.Media = normalizeAliVideoMedia(source.Media)
	}
	if source.MultiPrompt != nil {
		target.MultiPrompt = source.MultiPrompt
	}
	if source.ElementList != nil {
		target.ElementList = source.ElementList
	}
	if source.ReferenceVoice != nil {
		target.ReferenceVoice = source.ReferenceVoice
	}
}

func applyAliParameterOverride(source *aliVideoParametersV2, target *aliVideoParametersV2) {
	if source.Resolution != "" {
		target.Resolution = source.Resolution
	}
	if source.Size != "" {
		target.Size = source.Size
	}
	if source.Duration > 0 {
		target.Duration = source.Duration
	}
	if source.PromptExtend != nil {
		target.PromptExtend = source.PromptExtend
	}
	if source.Watermark != nil {
		target.Watermark = source.Watermark
	}
	if source.Audio != nil {
		target.Audio = source.Audio
	}
	if source.Seed > 0 {
		target.Seed = source.Seed
	}
	if source.Ratio != "" {
		target.Ratio = source.Ratio
	}
	if source.AspectRatio != "" {
		target.AspectRatio = source.AspectRatio
	}
	if source.Mode != "" {
		target.Mode = source.Mode
	}
	if source.ShotType != "" {
		target.ShotType = source.ShotType
	}
	if source.AudioSetting != nil {
		target.AudioSetting = source.AudioSetting
	}
}

func aliDurationFromAny(value any) (int, error) {
	if value == nil {
		return 0, nil
	}
	switch v := value.(type) {
	case int:
		return positiveAliInt(v), nil
	case int64:
		return positiveAliInt(int(v)), nil
	case float64:
		return positiveAliInt(int(math.Ceil(v))), nil
	case float32:
		return positiveAliInt(int(math.Ceil(float64(v)))), nil
	case string:
		return parseAliPositiveInt(v)
	default:
		return 0, nil
	}
}

func parseAliPositiveInt(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return positiveAliInt(int(math.Ceil(parsed))), nil
}

func positiveAliInt(value int) int {
	if value > 0 {
		return value
	}
	return 0
}

func applyAliRequestMedia(modelName string, urls []string, aliReq *aliVideoRequestV2) {
	if len(urls) == 0 {
		return
	}
	switch {
	case modelName == "happyhorse-1.0-i2v":
		aliReq.Input.Media = append(aliReq.Input.Media, newAliVideoMedia("first_frame", urls[0]))
	case modelName == "happyhorse-1.0-r2v":
		for _, url := range urls {
			aliReq.Input.Media = append(aliReq.Input.Media, newAliVideoMedia("reference_image", url))
		}
	case modelName == "happyhorse-1.0-video-edit":
		for _, url := range urls {
			mediaType := "reference_image"
			if aliLooksLikeVideoURL(url) {
				mediaType = "video"
			}
			aliReq.Input.Media = append(aliReq.Input.Media, newAliVideoMedia(mediaType, url))
		}
	case isAliKlingModel(modelName):
		aliReq.Input.Media = append(aliReq.Input.Media, newAliVideoMedia("first_frame", urls[0]))
		if len(urls) > 1 {
			aliReq.Input.Media = append(aliReq.Input.Media, newAliVideoMedia("last_frame", urls[1]))
		}
	}
}

func ensureAliNewFormatMedia(modelName string, aliReq *aliVideoRequestV2) {
	if aliReq.Input.FirstFrameURL != "" && !aliHasMediaURL(aliReq.Input.Media, aliReq.Input.FirstFrameURL) {
		aliReq.Input.Media = append(aliReq.Input.Media, newAliVideoMedia(aliImageMediaType(modelName), aliReq.Input.FirstFrameURL))
	}
	if aliReq.Input.LastFrameURL != "" && !aliHasMediaURL(aliReq.Input.Media, aliReq.Input.LastFrameURL) {
		mediaType := "last_frame"
		if strings.Contains(modelName, "-r2v") || strings.Contains(modelName, "video-edit") {
			mediaType = "reference_image"
		}
		aliReq.Input.Media = append(aliReq.Input.Media, newAliVideoMedia(mediaType, aliReq.Input.LastFrameURL))
	}
}

func newAliVideoMedia(mediaType, url string) aliVideoMedia {
	return aliVideoMedia{Type: mediaType, URL: url}
}

func normalizeAliVideoMedia(media []aliVideoMedia) []aliVideoMedia {
	if len(media) == 0 {
		return media
	}
	normalized := make([]aliVideoMedia, 0, len(media))
	for _, item := range media {
		if item.URL == "" {
			switch {
			case item.ImageURL != "":
				item.URL = item.ImageURL
			case item.VideoURL != "":
				item.URL = item.VideoURL
			case item.AudioURL != "":
				item.URL = item.AudioURL
			}
		}
		item.ImageURL = ""
		item.VideoURL = ""
		item.AudioURL = ""
		normalized = append(normalized, item)
	}
	return normalized
}

func aliHasMediaURL(media []aliVideoMedia, url string) bool {
	for _, item := range media {
		if item.URL == url || item.ImageURL == url || item.VideoURL == url || item.AudioURL == url {
			return true
		}
	}
	return false
}

func aliImageMediaType(modelName string) string {
	if strings.Contains(modelName, "-r2v") || strings.Contains(modelName, "video-edit") {
		return "reference_image"
	}
	return "first_frame"
}

func aliVideoTypeForURL(modelName, key string) string {
	if strings.Contains(modelName, "video-edit") {
		return "video"
	}
	if isAliKlingModel(modelName) {
		return "base"
	}
	return "video"
}

func isAliNewFormatModel(modelName string) bool {
	return strings.HasPrefix(modelName, "happyhorse-1.0") || strings.HasPrefix(modelName, "kling/")
}

func isAliKlingModel(modelName string) bool {
	return strings.HasPrefix(modelName, "kling/")
}

func defaultAliResolution(modelName string) string {
	if strings.HasPrefix(modelName, "happyhorse-1.0") || strings.HasPrefix(modelName, "wan2.7") {
		return "1080P"
	}
	if strings.HasPrefix(modelName, "kling/") || strings.HasPrefix(modelName, "wan2.6") || strings.HasPrefix(modelName, "wan2.5") {
		return "1080P"
	}
	if strings.HasPrefix(modelName, "wan2.2-i2v-flash") {
		return "720P"
	}
	if strings.HasPrefix(modelName, "wan2.2-i2v-plus") {
		return "1080P"
	}
	return "720P"
}

func normalizeAliResolution(value string) string {
	resolution := strings.ToUpper(strings.TrimSpace(value))
	if resolution == "" {
		return ""
	}
	if !strings.HasSuffix(resolution, "P") {
		resolution += "P"
	}
	return resolution
}

func aliRequestResolution(aliReq *aliVideoRequestV2) string {
	if aliReq.Parameters == nil {
		return defaultAliResolution(aliReq.Model)
	}
	if aliReq.Parameters.Size != "" {
		if resolution, err := sizeToResolution(aliReq.Parameters.Size); err == nil {
			return resolution
		}
	}
	if isAliKlingModel(aliReq.Model) && aliReq.Parameters.Mode != "" {
		return aliResolutionFromMode(aliReq.Parameters.Mode)
	}
	resolution := normalizeAliResolution(aliReq.Parameters.Resolution)
	if resolution == "" {
		resolution = defaultAliResolution(aliReq.Model)
	}
	return resolution
}

func aliModeFromResolution(resolution string) string {
	if normalizeAliResolution(resolution) == "720P" {
		return "std"
	}
	return "pro"
}

func aliResolutionFromMode(mode string) string {
	if strings.EqualFold(mode, "std") {
		return "720P"
	}
	return "1080P"
}

func aliAspectRatioFromSize(size string) string {
	normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(size)), "*", "x")
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return ""
	}
	width, errW := strconv.Atoi(parts[0])
	height, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return ""
	}
	switch {
	case width == height:
		return "1:1"
	case width > height:
		return "16:9"
	default:
		return "9:16"
	}
}

func aliKlingNeedsAspectRatio(aliReq *aliVideoRequestV2) bool {
	if len(aliReq.Input.Media) == 0 {
		return true
	}
	for _, media := range aliReq.Input.Media {
		switch media.Type {
		case "base", "feature", "refer":
			return true
		}
	}
	return false
}

func aliLooksLikeVideoURL(url string) bool {
	lower := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(lower, ".mp4") || strings.Contains(lower, ".mov") || strings.HasPrefix(lower, "data:video/")
}

func aliHasReferenceVideo(media []aliVideoMedia) bool {
	for _, item := range normalizeAliVideoMedia(media) {
		if item.Type == "base" || item.Type == "feature" || item.Type == "video" || aliLooksLikeVideoURL(item.URL) {
			return true
		}
	}
	return false
}

// EstimateBilling 根据用户请求参数计算 OtherRatios（时长、分辨率等）。
// 在 ValidateRequestAndSetAction 之后、价格计算之前调用。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	if a.ChannelType != constant.ChannelTypeAliBailian {
		aliReq, err := a.convertToAliRequest(info, taskReq)
		if err != nil {
			return nil
		}
		otherRatios := map[string]float64{
			"seconds": float64(aliReq.Parameters.Duration),
		}
		ratios, err := processLegacyAliOtherRatios(aliReq)
		if err != nil {
			return otherRatios
		}
		for k, v := range ratios {
			otherRatios[k] = v
		}
		applyAliConfiguredMultiplierFallbacks(info, otherRatios)
		return otherRatios
	}

	aliReq, err := a.convertToAliRequestV2(info, taskReq)
	if err != nil {
		return nil
	}

	otherRatios := map[string]float64{
		"seconds": float64(aliReq.Parameters.Duration),
	}
	ratios, err := ProcessAliOtherRatios(aliReq)
	if err != nil {
		return otherRatios
	}
	for k, v := range ratios {
		otherRatios[k] = v
	}
	applyAliConfiguredMultiplierFallbacks(info, otherRatios)
	return otherRatios
}

func applyAliConfiguredMultiplierFallbacks(info *relaycommon.RelayInfo, ratios map[string]float64) {
	if info == nil || !billing_setting.IsPerSecondBilling(info.OriginModelName) || len(ratios) == 0 {
		return
	}
	configured := billing_setting.GetPerSecondMultipliers(info.OriginModelName)
	if len(configured) == 0 {
		return
	}
	for key := range ratios {
		if value, ok := configured[key]; ok {
			ratios[key] = value
		}
	}
}

// DoRequest delegates to common helper
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// 解析阿里响应
	var aliResp AliVideoResponse
	if err := common.Unmarshal(responseBody, &aliResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	// 检查错误
	if aliResp.Code != "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("%s: %s", aliResp.Code, aliResp.Message), "ali_api_error", resp.StatusCode)
		return
	}

	if aliResp.Output.TaskID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// 转换为 OpenAI 格式响应
	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = info.PublicTaskID
	openAIResp.TaskID = info.PublicTaskID
	openAIResp.Model = c.GetString("model")
	if openAIResp.Model == "" && info != nil {
		openAIResp.Model = info.OriginModelName
	}
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.CreatedAt = common.GetTimestamp()

	// 返回 OpenAI 格式
	c.JSON(http.StatusOK, openAIResp)

	return aliResp.Output.TaskID, responseBody, nil
}

// FetchTask 查询任务状态
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v1/tasks/%s", baseUrl, taskID)

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
	return ModelListForChannelType(a.ChannelType)
}

func (a *TaskAdaptor) GetChannelName() string {
	if a.ChannelType == constant.ChannelTypeAliBailian {
		return BailianMediaChannelName
	}
	return ChannelName
}

// ParseTaskResult 解析任务结果
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(respBody, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// 状态映射
	switch aliResp.Output.TaskStatus {
	case "PENDING":
		taskResult.Status = model.TaskStatusQueued
	case "RUNNING":
		taskResult.Status = model.TaskStatusInProgress
	case "SUCCEEDED":
		taskResult.Status = model.TaskStatusSuccess
		// 阿里直接返回视频URL，不需要额外的代理端点
		taskResult.Url = aliResp.Output.VideoURL
	case "FAILED", "CANCELED", "UNKNOWN":
		taskResult.Status = model.TaskStatusFailure
		if aliResp.Message != "" {
			taskResult.Reason = aliResp.Message
		} else if aliResp.Output.Message != "" {
			taskResult.Reason = fmt.Sprintf("task failed, code: %s , message: %s", aliResp.Output.Code, aliResp.Output.Message)
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusQueued
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(task.Data, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal ali response failed")
	}

	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = task.TaskID
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.Model = task.Properties.OriginModelName
	openAIResp.SetProgressStr(task.Progress)
	openAIResp.CreatedAt = task.CreatedAt
	openAIResp.CompletedAt = task.UpdatedAt

	// 设置视频URL（核心字段）
	openAIResp.SetMetadata("url", aliResp.Output.VideoURL)

	// 错误处理
	if aliResp.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{
			Code:    aliResp.Code,
			Message: aliResp.Message,
		}
	} else if aliResp.Output.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{
			Code:    aliResp.Output.Code,
			Message: aliResp.Output.Message,
		}
	}

	return common.Marshal(openAIResp)
}

func convertAliStatus(aliStatus string) string {
	switch aliStatus {
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

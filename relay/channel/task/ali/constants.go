package ali

import "github.com/QuantumNous/new-api/constant"

var WanVideoModelList = []string{
	"wan2.5-i2v-preview",
	"wan2.2-i2v-flash",
	"wan2.2-i2v-plus",
	"wanx2.1-i2v-plus",
	"wanx2.1-i2v-turbo",
}

var BailianMediaModelList = []string{
	"kling/kling-v3-video-generation",
	"kling/kling-v3-omni-video-generation",
	"happyhorse-1.0-t2v",
	"happyhorse-1.0-i2v",
	"happyhorse-1.0-r2v",
	"happyhorse-1.0-video-edit",
}

var ModelList = append([]string{}, WanVideoModelList...)

var ChannelName = "ali"
var BailianMediaChannelName = "ali-bailian"

func ModelListForChannelType(channelType int) []string {
	if channelType == constant.ChannelTypeAliBailian {
		return append(append([]string{}, BailianMediaModelList...), WanVideoModelList...)
	}
	return append([]string{}, ModelList...)
}

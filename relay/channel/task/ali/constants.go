package ali

var WanVideoModelList = []string{
	"wan2.7-i2v",
	"wan2.7-t2v",
	"wan2.5-i2v-preview",
	"wan2.2-i2v-flash",
	"wan2.2-i2v-plus",
	"wanx2.1-i2v-plus",
	"wanx2.1-i2v-turbo",
}

var ModelList = append([]string{}, WanVideoModelList...)

var ChannelName = "ali"

func ModelListForChannelType(channelType int) []string {
	return append([]string{}, ModelList...)
}

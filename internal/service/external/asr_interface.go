package external

// ASRService 语音转文本服务接口
type ASRService interface {
	Transcribe(filePath string) (string, error)
}

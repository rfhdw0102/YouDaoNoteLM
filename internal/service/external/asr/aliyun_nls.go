package asr

import (
	"YoudaoNoteLm/internal/service/external/storage"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"YoudaoNoteLm/pkg/logger"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"go.uber.org/zap"
)

const (
	nlsRegionID     = "cn-shanghai"
	nlsProduct      = "nls-filetrans"
	nlsDomain       = "filetrans.cn-shanghai.aliyuncs.com"
	nlsAPIVersion   = "2018-08-17"
	nlsPollInterval = 3 * time.Second
	nlsPollTimeout  = 10 * time.Minute
)

// aliyunNLSASRService 阿里云智能语音交互 ASR 服务
type aliyunNLSASRService struct {
	accessKeyID     string
	accessKeySecret string
	appKey          string
	storage         storage.FileStorage
	client          *sdk.Client

	tokenMu    sync.RWMutex
	token      string
	tokenExpAt time.Time
}

// NewAliyunNLSASRService 创建阿里云 NLS ASR 服务
func NewAliyunNLSASRService(accessKeyID, accessKeySecret, appKey string) ASRService {
	// 创建 SDK 客户端
	c := sdk.NewConfig()
	credential := credentials.NewAccessKeyCredential(accessKeyID, accessKeySecret)
	client, err := sdk.NewClientWithOptions(nlsRegionID, c, credential)
	if err != nil {
		logger.Error("创建阿里云SDK客户端失败", zap.Error(err))
		// 返回一个会报错的实例
		return &aliyunNLSASRService{
			accessKeyID:     accessKeyID,
			accessKeySecret: accessKeySecret,
			appKey:          appKey,
		}
	}

	return &aliyunNLSASRService{
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		appKey:          appKey,
		client:          client,
	}
}

// SetStorage 设置文件存储（用于生成预签名 URL 给阿里云下载音频）
func (s *aliyunNLSASRService) SetStorage(storage storage.FileStorage) {
	s.storage = storage
}

// Transcribe 音频文件转文本
// filePath 为 MinIO 对象路径，如 "uploads/12345.mp3"
func (s *aliyunNLSASRService) Transcribe(filePath string) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("阿里云 SDK 客户端未初始化")
	}
	if s.storage == nil {
		return "", fmt.Errorf("ASR 服务未配置文件存储，无法获取文件 URL")
	}

	minioStore, ok := s.storage.(interface {
		GetPresignedURL(string, time.Duration) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("存储类型不支持预签名 URL")
	}
	audioURL, err := minioStore.GetPresignedURL(filePath, 2*time.Hour)
	if err != nil {
		return "", fmt.Errorf("生成音频文件URL失败: %w", err)
	}

	logger.Info("ASR转写开始",
		zap.String("file", filePath),
		zap.String("audio_url", audioURL),
	)

	// 1. 提交录音文件识别任务
	taskID, err := s.submitTask(audioURL)
	if err != nil {
		return "", err
	}

	// 2. 轮询查询结果
	text, err := s.pollResult(taskID)
	if err != nil {
		return "", err
	}

	logger.Info("ASR转写成功", zap.String("file", filePath))
	return text, nil
}

// submitTask 提交录音文件识别任务（使用官方 SDK）
func (s *aliyunNLSASRService) submitTask(audioURL string) (string, error) {
	postRequest := requests.NewCommonRequest()
	postRequest.Domain = nlsDomain
	postRequest.Version = nlsAPIVersion
	postRequest.Product = nlsProduct
	postRequest.ApiName = "SubmitTask"
	postRequest.Method = "POST"

	mapTask := make(map[string]string)
	mapTask["appkey"] = s.appKey
	mapTask["file_link"] = audioURL
	mapTask["version"] = "4.0"
	mapTask["enable_words"] = "false"

	task, err := json.Marshal(mapTask)
	if err != nil {
		return "", fmt.Errorf("序列化任务参数失败: %w", err)
	}
	postRequest.FormParams["Task"] = string(task)

	logger.Info("提交ASR任务", zap.String("params", string(task)))

	postResponse, err := s.client.ProcessCommonRequest(postRequest)
	if err != nil {
		return "", fmt.Errorf("提交转写任务请求失败: %w", err)
	}

	postResponseContent := postResponse.GetHttpContentString()
	logger.Info("ASR提交任务响应",
		zap.Int("http_status", postResponse.GetHttpStatus()),
		zap.String("response", postResponseContent),
	)

	if postResponse.GetHttpStatus() != 200 {
		return "", fmt.Errorf("提交转写任务返回错误 HTTP %d", postResponse.GetHttpStatus())
	}

	var postMapResult map[string]interface{}
	if err := json.Unmarshal([]byte(postResponseContent), &postMapResult); err != nil {
		return "", fmt.Errorf("解析转写任务响应失败: %w", err)
	}

	statusText, ok := postMapResult["StatusText"].(string)
	if !ok {
		return "", fmt.Errorf("转写任务响应中缺少 StatusText")
	}
	if statusText != "SUCCESS" {
		return "", fmt.Errorf("提交转写任务失败: %s", statusText)
	}

	taskID, ok := postMapResult["TaskId"].(string)
	if !ok || taskID == "" {
		return "", fmt.Errorf("转写任务响应中缺少 TaskId")
	}
	logger.Info("ASR转写任务已提交", zap.String("task_id", taskID))
	return taskID, nil
}

// pollResult 轮询转写结果（使用官方 SDK）
func (s *aliyunNLSASRService) pollResult(taskID string) (string, error) {
	getRequest := requests.NewCommonRequest()
	getRequest.Domain = nlsDomain
	getRequest.Version = nlsAPIVersion
	getRequest.Product = nlsProduct
	getRequest.ApiName = "GetTaskResult"
	getRequest.Method = "GET"
	getRequest.QueryParams["TaskId"] = taskID

	deadline := time.Now().Add(nlsPollTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(nlsPollInterval)

		getResponse, err := s.client.ProcessCommonRequest(getRequest)
		if err != nil {
			return "", fmt.Errorf("查询转写结果失败: %w", err)
		}

		getResponseContent := getResponse.GetHttpContentString()
		if getResponse.GetHttpStatus() != 200 {
			return "", fmt.Errorf("查询转写结果返回错误 HTTP %d", getResponse.GetHttpStatus())
		}

		var getMapResult map[string]interface{}
		if err := json.Unmarshal([]byte(getResponseContent), &getMapResult); err != nil {
			return "", fmt.Errorf("解析转写结果失败: %w", err)
		}

		statusText, ok := getMapResult["StatusText"].(string)
		if !ok {
			return "", fmt.Errorf("转写结果响应中缺少 StatusText")
		}
		switch statusText {
		case "RUNNING", "QUEUEING":
			logger.Debug("ASR任务处理中",
				zap.String("task_id", taskID),
				zap.String("status", statusText),
			)
		case "SUCCESS":
			// 提取识别结果
			result, ok := getMapResult["Result"].(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("转写结果格式错误")
			}
			sentences, ok := result["Sentences"].([]interface{})
			if !ok {
				return "", fmt.Errorf("转写结果句子格式错误")
			}
			var texts []string
			for _, sentence := range sentences {
				if sent, ok := sentence.(map[string]interface{}); ok {
					if text, ok := sent["Text"].(string); ok && text != "" {
						texts = append(texts, text)
					}
				}
			}
			return strings.Join(texts, ""), nil
		default:
			return "", fmt.Errorf("ASR转写失败，状态: %s", statusText)
		}
	}

	return "", fmt.Errorf("ASR转写超时，任务ID: %s", taskID)
}

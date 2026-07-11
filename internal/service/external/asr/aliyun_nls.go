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
// 构造失败（如 AccessKey 凭证格式非法）返回 nil，调用方应检查 nil
func NewAliyunNLSASRService(accessKeyID, accessKeySecret, appKey string) ASRService {
	// 创建 SDK 客户端
	c := sdk.NewConfig()
	c.AutoRetry = true
	c.MaxRetryTime = 3
	c.Timeout = 30 * time.Second
	c.Scheme = "HTTPS" // 使用 HTTPS（阿里云 ASR 推荐）
	c.Debug = true     // 开启调试日志
	credential := credentials.NewAccessKeyCredential(accessKeyID, accessKeySecret)
	client, err := sdk.NewClientWithOptions(nlsRegionID, c, credential)
	if err != nil {
		logger.Error("创建阿里云SDK客户端失败", zap.Error(err))
		return nil
	}

	logger.Info("阿里云ASR SDK客户端创建成功",
		zap.String("region", nlsRegionID),
		zap.String("scheme", c.Scheme),
	)

	return &aliyunNLSASRService{
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		appKey:          appKey,
		client:          client,
	}
}

// ValidateAliyunCredentials 验证阿里云 ASR 凭证是否有效
// 通过发送一个轻量请求（GetTaskResult 携带假 TaskId）触发阿里云鉴权：
//   - 鉴权失败（AccessKey 无效/签名不匹配） → 返回错误
//   - 鉴权通过（即使返回业务错误如 TaskNotFound） → 返回 nil
func ValidateAliyunCredentials(srv ASRService) error {
	s, ok := srv.(*aliyunNLSASRService)
	if !ok {
		return fmt.Errorf("非阿里云 ASR 服务实例")
	}
	return s.validateCredentials()
}

func (s *aliyunNLSASRService) validateCredentials() error {
	if s.client == nil {
		return fmt.Errorf("阿里云 SDK 客户端未初始化")
	}
	req := requests.NewCommonRequest()
	req.Domain = nlsDomain
	req.Version = nlsAPIVersion
	req.Product = nlsProduct
	req.ApiName = "GetTaskResult"
	req.Method = "GET"
	req.Scheme = requests.HTTPS
	// 假 TaskId，仅用于触发阿里云鉴权流程
	req.QueryParams["TaskId"] = "000000000000000000000000"

	_, err := s.client.ProcessCommonRequest(req)
	if err != nil {
		errStr := err.Error()
		// 鉴权类错误 → AccessKey 凭证无效
		if strings.Contains(errStr, "InvalidAccessKeyId") ||
			strings.Contains(errStr, "SignatureDoesNotMatch") ||
			strings.Contains(errStr, "Forbidden.AccessKeyDisabled") {
			return fmt.Errorf("AccessKey 凭证无效: %s", errStr)
		}
		// 其他错误（网络不通、SDK 异常等）
		return fmt.Errorf("连接阿里云失败: %w", err)
	}
	// 请求成功（HTTP 200），说明鉴权通过；TaskId 不存在的业务错误不影响凭证有效性判断
	return nil
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

	// 获取文件访问 URL（预签名或代理 URL）
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
	postRequest.Scheme = requests.HTTPS // 使用 HTTPS（阿里云 ASR 推荐）

	mapTask := make(map[string]string)
	mapTask["appkey"] = s.appKey
	mapTask["file_link"] = audioURL
	mapTask["version"] = "4.0"
	mapTask["enable_words"] = "false"

	task, err := json.Marshal(mapTask)
	if err != nil {
		return "", fmt.Errorf("序列化任务参数失败: %w", err)
	}
	// json.Marshal 默认 HTML 转义会把 file_link 里的 & 转成 \u0026，
	// 导致阿里云拉取音频时 URL 查询参数分隔符损坏（FILE_403_FORBIDDEN）。还原 & 确保 file_link 完整。
	taskStr := strings.ReplaceAll(string(task), `\u0026`, "&")
	postRequest.FormParams["Task"] = taskStr

	logger.Info("提交ASR任务",
		zap.String("domain", nlsDomain),
		zap.String("scheme", string(postRequest.Scheme)),
		zap.String("params", taskStr),
	)

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
	getRequest.Scheme = requests.HTTPS // 使用 HTTPS（阿里云 ASR 推荐）
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
			// 记录完整的响应内容以便调试
			logger.Error("ASR转写失败",
				zap.String("task_id", taskID),
				zap.String("status", statusText),
				zap.Any("response", getMapResult),
			)
			return "", fmt.Errorf("ASR转写失败，状态: %s", statusText)
		}
	}

	return "", fmt.Errorf("ASR转写超时，任务ID: %s", taskID)
}

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service/external/asr"
	externalMarkitdown "YoudaoNoteLm/internal/service/external/markitdown"
	"YoudaoNoteLm/internal/service/external/storage"
	"YoudaoNoteLm/pkg/cache"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var allowedFileTypes = map[string]bool{
	".txt": true, ".md": true, ".docx": true, ".pdf": true, ".pptx": true,
}

var allowedAudioTypes = map[string]bool{
	".mp3": true, ".wav": true,
}

const maxFileSize int64 = 30 << 20   // 30MB
const maxAudioSize int64 = 300 << 20 // 300MB

type importerService struct {
	configSvc    ConfigService
	markitdown   externalMarkitdown.Client
	storage      storage.FileStorage
	sourceRepo   repository.SourceRepository
	importCache  *cache.ImportTaskCache
	previewCache *cache.AudioPreviewCache
	ingestionSvc rag.IngestionService
	cancelFuncs  sync.Map // taskID -> context.CancelFunc，用于中止运行中的任务
}

// NewImporterService 创建导入服务
func NewImporterService(
	configSvc ConfigService,
	markitdown externalMarkitdown.Client,
	storage storage.FileStorage,
	sourceRepo repository.SourceRepository,
	importCache *cache.ImportTaskCache,
	previewCache *cache.AudioPreviewCache,
	ingestionSvc rag.IngestionService,
) ImporterService {
	return &importerService{
		markitdown:   markitdown,
		configSvc:    configSvc,
		storage:      storage,
		sourceRepo:   sourceRepo,
		importCache:  importCache,
		previewCache: previewCache,
		ingestionSvc: ingestionSvc,
	}
}

// ImportFile 文件上传导入
func (s *importerService) ImportFile(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedFileTypes[ext] {
		return nil, bizerrors.ErrUnsupportedFormat
	}
	if file.Size > maxFileSize {
		return nil, bizerrors.ErrFileTooLarge
	}

	// 读取文件内容（用于 MarkItDown 转换）
	src, err := file.Open()
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "打开上传文件失败", err)
	}
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {
			logger.Errorf("关闭文件失败:%s", err)
		}
	}(src)
	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "读取上传文件失败", err)
	}

	// 上传到 MinIO 存储
	filePath, err := s.storage.Upload(file)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "文件上传失败", err)
	}

	// 通过 io.Reader 传给 MarkItDown 转换
	markdown, err := s.markitdown.ConvertReader(file.Filename, bytes.NewReader(fileBytes))
	if err != nil {
		logger.Warn("MarkItDown转换失败，使用原始文件内容", zap.String("file", file.Filename), zap.Error(err))
		// 降级：对于文本文件，直接返回内容
		if ext == ".txt" || ext == ".md" {
			markdown = string(fileBytes)
		} else {
			return nil, bizerrors.NewWithErr(bizerrors.CodeFileParseFailed, "文件解析失败", err)
		}
	}

	source := &entity.Source{
		UserID:          userID,
		NotebookID:      notebookID,
		Name:            file.Filename,
		Type:            "file",
		FilePath:        filePath,
		FileSize:        file.Size,
		MimeType:        file.Header.Get("Content-Type"),
		MarkdownContent: markdown,
		Status:          "ready",
	}

	if err := s.sourceRepo.Create(source); err != nil {
		return nil, err
	}

	// 同步触发 RAG 入库
	if s.ingestionSvc != nil {
		if err := s.ingestionSvc.IngestSingle(context.Background(), source.ID); err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "RAG 入库失败", err)
		}
	}

	return source, nil
}

// PreviewAudio 异步音频转写：上传文件后立即返回 previewID，后台执行 ASR 转写
func (s *importerService) PreviewAudio(userID, notebookID uint, file *multipart.FileHeader) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedAudioTypes[ext] {
		return "", "", bizerrors.ErrUnsupportedFormat
	}
	if file.Size > maxAudioSize {
		return "", "", bizerrors.ErrFileTooLarge
	}

	// 上传原始文件到 MinIO
	filePath, err := s.storage.Upload(file)
	if err != nil {
		return "", "", bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "音频上传失败", err)
	}

	previewID := uuid.New().String()
	preview := &cache.AudioPreview{
		PreviewID:  previewID,
		UserID:     userID,
		NotebookID: notebookID,
		FileName:   file.Filename,
		FilePath:   filePath,
		FileSize:   file.Size,
		Status:     "pending",
		ExpiresAt:  time.Now().Add(30 * time.Minute).Unix(),
	}

	ctx := context.Background()
	if err := s.previewCache.Save(ctx, preview); err != nil {
		return "", "", err
	}

	// 后台异步执行 ASR 转写
	go s.doAudioTranscribe(previewID, userID, file, filePath, ext)

	return previewID, file.Filename, nil
}

// doAudioTranscribe 后台执行音频转写，完成后更新缓存
func (s *importerService) doAudioTranscribe(previewID string, userID uint, file *multipart.FileHeader, filePath, ext string) {
	ctx := context.Background()

	// 标记为处理中
	if err := s.previewCache.UpdateStatus(ctx, previewID, "processing"); err != nil {
		logger.Error("更新预览状态为processing失败", zap.String("preview_id", previewID), zap.Error(err))
		return
	}

	// 转换音频为 ASR 兼容格式
	asrFilePath := filePath
	convertedData, convErr := s.convertAudioForASR(file, ext)
	if convErr != nil {
		logger.Warn("音频格式转换失败，将使用原始文件", zap.String("file", filePath), zap.Error(convErr))
	} else if convertedData != nil {
		asrPath := strings.TrimSuffix(filePath, ext) + "_16k.wav"
		if uploadErr := s.storage.UploadBytes(asrPath, convertedData, "audio/wav"); uploadErr != nil {
			logger.Warn("上传转换后音频失败，将使用原始文件", zap.String("file", filePath), zap.Error(uploadErr))
		} else {
			asrFilePath = asrPath
			logger.Info("音频格式转换成功",
				zap.String("original", filePath),
				zap.String("converted", asrPath),
			)
		}
	}

	// 获取 ASR 服务
	asrSvc, err := s.getASR(userID)
	if err != nil {
		logger.Error("获取ASR服务失败", zap.String("preview_id", previewID), zap.Error(err))
		s.markPreviewFailed(ctx, previewID, "未配置 ASR 服务")
		return
	}

	// 执行转写
	text, err := asrSvc.Transcribe(asrFilePath)
	if err != nil {
		logger.Error("ASR转写失败", zap.String("preview_id", previewID), zap.Error(err))
		s.markPreviewFailed(ctx, previewID, fmt.Sprintf("音频转写失败: %v", err))
		return
	}

	// 转写成功，更新缓存
	preview, err := s.previewCache.Get(ctx, previewID)
	if err != nil || preview == nil {
		logger.Error("转写完成但获取预览缓存失败", zap.String("preview_id", previewID), zap.Error(err))
		return
	}
	preview.TranscribedText = text
	preview.Status = "ready"
	if err := s.previewCache.Save(ctx, preview); err != nil {
		logger.Error("保存转写结果失败", zap.String("preview_id", previewID), zap.Error(err))
		return
	}

	logger.Info("音频转写完成", zap.String("preview_id", previewID), zap.Int("text_len", len(text)))
}

// markPreviewFailed 标记预览转写失败
func (s *importerService) markPreviewFailed(ctx context.Context, previewID, errMsg string) {
	preview, err := s.previewCache.Get(ctx, previewID)
	if err != nil || preview == nil {
		return
	}
	preview.Status = "failed"
	preview.ErrorMsg = errMsg
	if saveErr := s.previewCache.Save(ctx, preview); saveErr != nil {
		logger.Error("保存预览失败状态出错", zap.String("preview_id", previewID), zap.Error(saveErr))
	}
}

// GetAudioPreviewStatus 查询音频预览状态（前端轮询用）
func (s *importerService) GetAudioPreviewStatus(userID uint, previewID string) (interface{}, error) {
	ctx := context.Background()
	preview, err := s.previewCache.Get(ctx, previewID)
	if err != nil {
		return nil, bizerrors.ErrNotFound
	}
	if preview == nil {
		return nil, bizerrors.ErrNotFound
	}
	if preview.UserID != userID {
		return nil, bizerrors.ErrForbidden
	}
	return preview, nil
}

// ConfirmAudio 确认音频导入
func (s *importerService) ConfirmAudio(userID uint, previewID string, editedContent *string) (*entity.Source, error) {
	ctx := context.Background()
	preview, err := s.previewCache.Get(ctx, previewID)
	if err != nil {
		return nil, bizerrors.ErrNotFound
	}
	if preview == nil {
		return nil, bizerrors.ErrNotFound
	}
	if preview.UserID != userID {
		return nil, bizerrors.ErrForbidden
	}
	if time.Now().Unix() > preview.ExpiresAt {
		return nil, bizerrors.ErrPreviewExpired
	}
	if preview.Status == "failed" {
		return nil, bizerrors.New(bizerrors.CodeASTranscriptionFailed, preview.ErrorMsg)
	}
	if preview.Status != "ready" {
		return nil, bizerrors.New(bizerrors.CodeBadRequest, "音频转写尚未完成，请稍后再试")
	}

	content := preview.TranscribedText
	if editedContent != nil && *editedContent != "" {
		content = *editedContent
	}

	source := &entity.Source{
		UserID:          userID,
		NotebookID:      preview.NotebookID,
		Name:            preview.FileName,
		Type:            "audio",
		FilePath:        preview.FilePath,
		FileSize:        preview.FileSize,
		MarkdownContent: content,
		Status:          "ready",
	}

	if err := s.sourceRepo.Create(source); err != nil {
		return nil, err
	}

	// 同步触发 RAG 入库
	if s.ingestionSvc != nil {
		if err := s.ingestionSvc.IngestSingle(context.Background(), source.ID); err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "RAG 入库失败", err)
		}
	}

	if err := s.previewCache.UpdateStatus(ctx, previewID, "confirmed"); err != nil {
		logger.Warn("更新预览状态失败", zap.String("preview_id", previewID), zap.Error(err))
	}

	return source, nil
}

// convertAudioForASR 转换音频为 ASR 兼容格式
// 如果已经是 16kHz 单声道则返回 nil（无需转换）
func (s *importerService) convertAudioForASR(file *multipart.FileHeader, ext string) ([]byte, error) {
	// 读取文件内容
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开音频文件失败: %w", err)
	}
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {
			logger.Errorf("关闭文件失败:%s", err)
		}
	}(src)

	audioData, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("读取音频文件失败: %w", err)
	}

	// 转换为 16kHz 单声道 WAV
	converted, err := utils.ConvertBytesToASRFormat(audioData, ext)
	if err != nil {
		return nil, fmt.Errorf("音频转换失败: %w", err)
	}

	return converted, nil
}

// ImportSearchResults 批量导入搜索结果
// 为每个 URL 先创建 pending 状态的 Source 记录，然后异步处理
// 返回创建的 Source ID 列表，前端可通过 Source 列表 API 查看每条的独立状态
func (s *importerService) ImportSearchResults(userID, notebookID uint, items []SearchResultItem) (string, []uint, error) {
	// 去重：同一个 URL 只创建一条记录（保留第一次出现的标题）
	seen := make(map[string]string, len(items)) // url -> title
	for _, item := range items {
		if _, exists := seen[item.URL]; !exists {
			seen[item.URL] = item.Title
		}
	}

	sourceIDs := make([]uint, 0, len(seen))

	// 为每个 URL 创建 pending 状态的 Source
	for url, title := range seen {
		// 如果标题为空，使用 URL 作为标题
		name := title
		if name == "" {
			name = url
		}

		source := &entity.Source{
			UserID:      userID,
			NotebookID:  notebookID,
			Name:        name,
			Type:        "url",
			OriginalURL: url,
			Status:      "pending",
		}
		if err := s.sourceRepo.Create(source); err != nil {
			logger.Error("创建待导入Source失败", zap.String("url", url), zap.Error(err))
			continue
		}
		sourceIDs = append(sourceIDs, source.ID)
	}

	if len(sourceIDs) == 0 {
		return "", nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "创建导入记录失败", nil)
	}

	// 创建可取消的 context，注册 cancel func 以便批量取消
	taskID := uuid.New().String()
	taskCtx, cancel := context.WithCancel(context.Background())
	s.cancelFuncs.Store(taskID, cancel)

	// 异步处理每个 Source
	go s.processSources(taskCtx, taskID, sourceIDs)

	return taskID, sourceIDs, nil
}

// processSources 异步处理 Source 列表（带并发控制，支持取消）
func (s *importerService) processSources(taskCtx context.Context, taskID string, sourceIDs []uint) {
	// 任务结束后清理 cancel func
	defer s.cancelFuncs.Delete(taskID)

	// 并发控制：最多同时处理 3 个
	concurrency := 3
	if len(sourceIDs) < concurrency {
		concurrency = len(sourceIDs)
	}

	idCh := make(chan uint, concurrency)
	doneCh := make(chan struct{}, len(sourceIDs))

	// 启动 worker
	for i := 0; i < concurrency; i++ {
		go func() {
			for sourceID := range idCh {
				if taskCtx.Err() != nil {
					doneCh <- struct{}{}
					continue
				}
				s.processSingleSource(taskCtx, sourceID)
				doneCh <- struct{}{}
			}
		}()
	}

	// 分发任务（支持取消中断分发）
	go func() {
		for _, sourceID := range sourceIDs {
			if taskCtx.Err() != nil {
				break
			}
			idCh <- sourceID
		}
		close(idCh)
	}()

	// 等待所有任务完成
	for i := 0; i < len(sourceIDs); i++ {
		<-doneCh
	}

	// 将仍然处于 pending 状态的 Source 标记为 cancelled（被取消的任务）
	if taskCtx.Err() != nil {
		for _, sourceID := range sourceIDs {
			src, err := s.sourceRepo.FindByID(sourceID)
			if err != nil || src == nil {
				continue
			}
			if src.Status == "pending" {
				if err := s.sourceRepo.UpdateStatus(sourceID, "cancelled", "任务已取消"); err != nil {
					logger.Warn("更新Source状态为cancelled失败", zap.Uint("source_id", sourceID), zap.Error(err))
				}
			}
		}
	}
}

// processSingleSource 处理单个 Source（支持取消）
func (s *importerService) processSingleSource(taskCtx context.Context, sourceID uint) {
	// 处理前检查取消
	if taskCtx.Err() != nil {
		return
	}

	// 获取 Source 记录
	source, err := s.sourceRepo.FindByID(sourceID)
	if err != nil || source == nil {
		logger.Error("获取Source失败", zap.Uint("source_id", sourceID), zap.Error(err))
		return
	}

	// 更新状态为 processing
	if err := s.sourceRepo.UpdateStatus(sourceID, "processing", ""); err != nil {
		logger.Warn("更新Source状态为processing失败", zap.Uint("source_id", sourceID), zap.Error(err))
	}

	// 转换 URL 内容
	markdown, err := s.markitdown.ConvertFromURLWithContext(taskCtx, source.OriginalURL)
	if err != nil {
		// 如果是因为取消导致的错误
		if taskCtx.Err() != nil {
			logger.Info("任务已取消，跳过Source处理", zap.Uint("source_id", sourceID))
			return
		}

		// 处理结构化错误，返回用户友好的错误信息
		var userMsg string
		var convertErr *externalMarkitdown.ConvertError
		if errors.As(err, &convertErr) {
			// 记录详细的技术错误信息到日志
			logger.Warn("URL转换失败",
				zap.String("url", source.OriginalURL),
				zap.String("error_code", convertErr.Code),
				zap.String("detail", convertErr.DetailMsg),
				zap.Int("http_status", convertErr.HTTPStatus),
			)
			// 使用用户友好的错误消息
			userMsg = convertErr.UserMsg
		} else {
			// 未知错误类型
			logger.Warn("URL转换失败", zap.String("url", source.OriginalURL), zap.Error(err))
			userMsg = "网页内容获取失败，请稍后重试"
		}

		if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", userMsg); updateErr != nil {
			logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
		}
		return
	}

	// 转换完成后再检查一次 source 是否还存在（可能在转换期间被用户删除）
	existing, _ := s.sourceRepo.FindByID(sourceID)
	if existing == nil {
		logger.Info("Source已被删除，丢弃转换结果", zap.Uint("source_id", sourceID))
		return
	}

	// 更新 Source 内容和状态为 ready
	source.MarkdownContent = markdown
	source.Status = "ready"
	if err := s.sourceRepo.Update(source); err != nil {
		logger.Error("更新Source内容失败", zap.Uint("source_id", sourceID), zap.Error(err))
		if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", fmt.Sprintf("保存失败: %v", err)); updateErr != nil {
			logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
		}
		return
	}

	// 同步触发 RAG 入库
	if s.ingestionSvc != nil {
		if err := s.ingestionSvc.IngestSingle(taskCtx, sourceID); err != nil {
			logger.Error("RAG 入库失败", zap.Uint("source_id", sourceID), zap.Error(err))
			if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", fmt.Sprintf("RAG 入库失败: %v", err)); updateErr != nil {
				logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
			}
			return
		}
	}

	logger.Info("Source导入成功", zap.Uint("source_id", sourceID), zap.String("url", source.OriginalURL))
}

// GetImportTask 获取导入任务状态
func (s *importerService) GetImportTask(taskID string) (interface{}, error) {
	ctx := context.Background()
	task, err := s.importCache.Get(ctx, taskID)
	if err != nil {
		return nil, bizerrors.ErrNotFound
	}
	if task == nil {
		return nil, bizerrors.ErrNotFound
	}
	return task, nil
}

// DeleteImportTask 删除/取消导入任务
func (s *importerService) DeleteImportTask(taskID string) error {
	ctx := context.Background()

	// 1. 尝试从 cancelFuncs 中取消正在运行的异步任务（新架构：Source-based 导入）
	if cancel, ok := s.cancelFuncs.Load(taskID); ok {
		cancel.(context.CancelFunc)()
		s.cancelFuncs.Delete(taskID)
		logger.Info("已发送取消信号给运行中的导入任务", zap.String("task_id", taskID))
		return nil
	}

	// 2. 尝试从 importCache 中查找（旧架构：Redis-based 任务）
	task, err := s.importCache.Get(ctx, taskID)
	if err != nil {
		return bizerrors.ErrNotFound
	}
	if task == nil {
		return bizerrors.ErrNotFound
	}

	// 如果任务正在运行中，标记为取消状态
	if task.Status == "running" {
		task.Status = "cancelled"
		if err := s.importCache.Save(ctx, task); err != nil {
			logger.Warn("更新任务状态为取消失败", zap.String("task_id", taskID), zap.Error(err))
		}
	}

	// 删除任务缓存
	return s.importCache.Delete(ctx, taskID)
}

// getASR 获取 ASR 服务（从 ConfigService 动态加载）
func (s *importerService) getASR(userID uint) (asr.ASRService, error) {
	if s.configSvc == nil {
		return nil, fmt.Errorf("ConfigService 未初始化")
	}
	return s.configSvc.GetASRService(userID)
}

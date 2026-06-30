package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"YoudaoNoteLm/internal/llm"
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

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
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
	structurer   MarkdownStructurer // LLM 结构化服务
	summaryCache *cache.SourceSummaryCache
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
	structurer MarkdownStructurer,
	summaryCache *cache.SourceSummaryCache,
) ImporterService {
	return &importerService{
		markitdown:   markitdown,
		configSvc:    configSvc,
		storage:      storage,
		sourceRepo:   sourceRepo,
		importCache:  importCache,
		previewCache: previewCache,
		ingestionSvc: ingestionSvc,
		structurer:   structurer,
		summaryCache: summaryCache,
	}
}

// ImportFile 文件上传导入（异步：立即创建 source，后台处理解析和入库）
func (s *importerService) ImportFile(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedFileTypes[ext] {
		return nil, bizerrors.ErrUnsupportedFormat
	}
	if file.Size > maxFileSize {
		return nil, bizerrors.ErrFileTooLarge
	}

	logger.Info("开始文件导入",
		zap.String("file", file.Filename),
		zap.Int64("size", file.Size),
		zap.Uint("user_id", userID),
	)

	// 上传到 MinIO 存储（必须同步，拿到 filePath）
	filePath, err := s.storage.Upload(file)
	if err != nil {
		logger.Error("文件上传到存储服务失败",
			zap.String("file", file.Filename),
			zap.Int64("size", file.Size),
			zap.Error(err),
		)
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "文件上传失败", err)
	}

	// 立即创建 source（status=processing），前端可以马上看到
	source := &entity.Source{
		UserID:     userID,
		NotebookID: notebookID,
		Name:       file.Filename,
		Type:       "file",
		FilePath:   filePath,
		FileSize:   file.Size,
		MimeType:   file.Header.Get("Content-Type"),
		Status:     "processing",
	}

	if err := s.sourceRepo.Create(source); err != nil {
		logger.Error("创建 Source 记录失败",
			zap.String("file", file.Filename),
			zap.Error(err),
		)
		return nil, err
	}

	logger.Info("Source 记录创建成功，后台开始处理",
		zap.String("file", file.Filename),
		zap.Uint("source_id", source.ID),
	)

	// 读取文件内容（后台 goroutine 需要，必须在 goroutine 外读取，避免 file 指针失效）
	src, err := file.Open()
	if err != nil {
		s.sourceRepo.UpdateStatus(source.ID, "failed", "打开上传文件失败")
		return source, nil
	}
	fileBytes, err := io.ReadAll(src)
	src.Close()
	if err != nil {
		s.sourceRepo.UpdateStatus(source.ID, "failed", "读取上传文件失败")
		return source, nil
	}

	// 后台异步处理：MarkItDown → LLM 结构化 → 更新内容 → RAG 入库
	go s.processFileImport(source.ID, file.Filename, ext, filePath, file.Header.Get("Content-Type"), file.Size, userID, fileBytes)

	return source, nil
}

// processFileImport 后台处理文件导入（解析、结构化、入库）
func (s *importerService) processFileImport(sourceID uint, fileName, ext, filePath, mimeType string, fileSize int64, userID uint, fileBytes []byte) {
	totalStart := time.Now()
	logger.Info("后台开始处理文件导入",
		zap.String("file", fileName),
		zap.Uint("source_id", sourceID),
		zap.Int64("file_size", fileSize),
	)

	// 1. MarkItDown 转换
	stepStart := time.Now()
	markdown, err := s.markitdown.ConvertReader(fileName, bytes.NewReader(fileBytes))
	if err != nil {
		logger.Error("MarkItDown 转换失败",
			zap.String("file", fileName),
			zap.Duration("elapsed", time.Since(stepStart)),
			zap.Error(err),
		)
		// 降级：对于文本文件，直接使用原始内容
		if ext == ".txt" || ext == ".md" {
			markdown = string(fileBytes)
			logger.Info("文本文件降级处理，使用原始内容",
				zap.String("file", fileName),
				zap.Int("content_len", len(markdown)),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
		} else {
			s.sourceRepo.UpdateStatus(sourceID, "failed", "文件解析失败")
			return
		}
	} else {
		logger.Info("MarkItDown 转换成功",
			zap.String("file", fileName),
			zap.Int("content_len", len(markdown)),
			zap.Duration("elapsed", time.Since(stepStart)),
		)
	}

	// 2. LLM 结构化
	stepStart = time.Now()
	if s.structurer != nil {
		result, err := s.structurer.Structure(context.Background(), userID, markdown, StructureMeta{
			Title:      fileName,
			SourceType: "file",
		})
		if err != nil {
			logger.Error("LLM 结构化失败，使用原始内容",
				zap.String("file", fileName),
				zap.Duration("elapsed", time.Since(stepStart)),
				zap.Error(err),
			)
		} else if result.ActuallyCalled {
			markdown = result.Content
			logger.Info("LLM 结构化完成",
				zap.String("file", fileName),
				zap.Int("content_len", len(markdown)),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
		} else {
			logger.Warn("LLM 结构化被跳过（模型配置问题或 API Key 过期）",
				zap.String("file", fileName),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
		}
	} else {
		logger.Warn("MarkdownStructurer 未配置，跳过结构化", zap.String("file", fileName))
	}

	// 3. 更新 source 内容和状态
	stepStart = time.Now()
	if err := s.sourceRepo.UpdateContent(sourceID, markdown, "ready"); err != nil {
		logger.Error("更新 Source 内容失败",
			zap.String("file", fileName),
			zap.Uint("source_id", sourceID),
			zap.Duration("elapsed", time.Since(stepStart)),
			zap.Error(err),
		)
		s.sourceRepo.UpdateStatus(sourceID, "failed", fmt.Sprintf("保存失败: %v", err))
		return
	}

	logger.Info("Source 内容更新成功",
		zap.String("file", fileName),
		zap.Uint("source_id", sourceID),
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	// 4. RAG 入库
	stepStart = time.Now()
	if s.ingestionSvc != nil {
		if err := s.ingestionSvc.IngestSingle(context.Background(), sourceID); err != nil {
			logger.Error("RAG 入库失败",
				zap.String("file", fileName),
				zap.Uint("source_id", sourceID),
				zap.Duration("elapsed", time.Since(stepStart)),
				zap.Error(err),
			)
			// RAG 入库失败不影响 source 可见性，只记录日志
			return
		}
		logger.Info("RAG 入库成功",
			zap.String("file", fileName),
			zap.Uint("source_id", sourceID),
			zap.Duration("elapsed", time.Since(stepStart)),
		)
	}

	// 5. 生成摘要（异步，不阻塞主流程）
	go s.generateAndSaveSummary(sourceID, userID, markdown)

	logger.Info("文件导入完成",
		zap.String("file", fileName),
		zap.Uint("source_id", sourceID),
		zap.Duration("total_elapsed", time.Since(totalStart)),
	)
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
		logger.Error("音频上传到存储服务失败",
			zap.String("file", file.Filename),
			zap.Int64("size", file.Size),
			zap.Error(err),
		)
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
	totalStart := time.Now()
	ctx := context.Background()

	// 标记为处理中
	if err := s.previewCache.UpdateStatus(ctx, previewID, "processing"); err != nil {
		logger.Error("更新预览状态为processing失败", zap.String("preview_id", previewID), zap.Error(err))
		return
	}

	// 使用 ffmpeg 流式转换为 16kHz 单声道 WAV（内存占用低，支持各种格式）
	asrFilePath := filePath
	convertedPath, convertErr := s.convertAudioWithFFMPEG(filePath, ext)
	if convertErr != nil {
		logger.Warn("ffmpeg音频转换失败，使用原始文件",
			zap.String("file", filePath),
			zap.Error(convertErr),
		)
	} else {
		asrFilePath = convertedPath
		logger.Info("音频已通过ffmpeg转换为16kHz单声道WAV",
			zap.String("original", filePath),
			zap.String("converted", asrFilePath),
		)
	}

	// 获取 ASR 服务
	stepStart := time.Now()
	asrSvc, err := s.getASR(userID)
	if err != nil {
		logger.Error("获取ASR服务失败",
			zap.String("preview_id", previewID),
			zap.Duration("elapsed", time.Since(stepStart)),
			zap.Error(err),
		)
		s.markPreviewFailed(ctx, previewID, "未配置 ASR 服务")
		return
	}
	logger.Info("获取 ASR 服务完成",
		zap.String("preview_id", previewID),
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	// 执行转写
	stepStart = time.Now()
	logger.Info("开始 ASR 转写",
		zap.String("preview_id", previewID),
		zap.String("asr_file", asrFilePath),
	)
	text, err := asrSvc.Transcribe(asrFilePath)
	if err != nil {
		logger.Error("ASR转写失败",
			zap.String("preview_id", previewID),
			zap.Duration("elapsed", time.Since(stepStart)),
			zap.Error(err),
		)
		s.markPreviewFailed(ctx, previewID, fmt.Sprintf("音频转写失败: %v", err))
		return
	}
	logger.Info("ASR 转写完成",
		zap.String("preview_id", previewID),
		zap.Int("text_len", len(text)),
		zap.Duration("elapsed", time.Since(stepStart)),
	)

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

	logger.Info("音频转写流程完成",
		zap.String("preview_id", previewID),
		zap.Int("text_len", len(text)),
		zap.Duration("total_elapsed", time.Since(totalStart)),
	)
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
	totalStart := time.Now()

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

	logger.Info("开始确认音频导入",
		zap.String("preview_id", previewID),
		zap.String("file_name", preview.FileName),
		zap.Uint("user_id", userID),
	)

	content := preview.TranscribedText
	if editedContent != nil && *editedContent != "" {
		content = *editedContent
		logger.Info("使用用户编辑后的内容",
			zap.String("preview_id", previewID),
			zap.Int("content_len", len(content)),
		)
	} else {
		logger.Info("使用 ASR 转写结果",
			zap.String("preview_id", previewID),
			zap.Int("content_len", len(content)),
		)
	}

	// LLM 结构化
	stepStart := time.Now()
	if s.structurer != nil {
		result, err := s.structurer.Structure(ctx, userID, content, StructureMeta{
			Title:      preview.FileName,
			SourceType: "audio",
		})
		if err != nil {
			logger.Error("LLM 结构化失败，使用原始内容",
				zap.String("preview_id", previewID),
				zap.Duration("elapsed", time.Since(stepStart)),
				zap.Error(err),
			)
		} else if result.ActuallyCalled && result.Content != content {
			logger.Info("LLM 结构化成功，内容已优化",
				zap.String("preview_id", previewID),
				zap.Int("original_len", len(content)),
				zap.Int("structured_len", len(result.Content)),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
			content = result.Content
		} else if result.ActuallyCalled {
			logger.Info("LLM 判断内容已有结构，无需结构化",
				zap.String("preview_id", previewID),
				zap.Int("content_len", len(content)),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
		} else {
			logger.Warn("LLM 结构化被跳过（模型配置问题或 API Key 过期）",
				zap.String("preview_id", previewID),
				zap.Int("content_len", len(content)),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
		}
	} else {
		logger.Warn("MarkdownStructurer 未配置，跳过结构化", zap.String("preview_id", previewID))
	}

	// 创建 Source 记录
	stepStart = time.Now()
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
		logger.Error("创建 Source 记录失败",
			zap.String("preview_id", previewID),
			zap.Duration("elapsed", time.Since(stepStart)),
			zap.Error(err),
		)
		return nil, err
	}

	logger.Info("Source 记录创建成功",
		zap.String("preview_id", previewID),
		zap.Uint("source_id", source.ID),
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	// 同步触发 RAG 入库
	stepStart = time.Now()
	if s.ingestionSvc != nil {
		if err := s.ingestionSvc.IngestSingle(context.Background(), source.ID); err != nil {
			logger.Error("RAG 入库失败",
				zap.String("preview_id", previewID),
				zap.Uint("source_id", source.ID),
				zap.Duration("elapsed", time.Since(stepStart)),
				zap.Error(err),
			)
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "RAG 入库失败", err)
		}
		logger.Info("RAG 入库成功",
			zap.String("preview_id", previewID),
			zap.Uint("source_id", source.ID),
			zap.Duration("elapsed", time.Since(stepStart)),
		)
		source.Vectorized = true
	}

	// 生成摘要（异步，不阻塞主流程）
	go s.generateAndSaveSummary(source.ID, userID, content)

	if err := s.previewCache.UpdateStatus(ctx, previewID, "confirmed"); err != nil {
		logger.Warn("更新预览状态失败", zap.String("preview_id", previewID), zap.Error(err))
	}

	logger.Info("音频导入确认完成",
		zap.String("preview_id", previewID),
		zap.String("file_name", preview.FileName),
		zap.Uint("source_id", source.ID),
		zap.Duration("total_elapsed", time.Since(totalStart)),
	)

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

// convertAudioWithFFMPEG 使用 ffmpeg 流式转换音频为 16kHz 单声道 WAV
// 从 MinIO 下载 → ffmpeg 转换 → 上传回 MinIO，全程流式处理，内存占用低
func (s *importerService) convertAudioWithFFMPEG(filePath, ext string) (string, error) {
	// 1. 下载原始文件到临时文件
	srcData, err := s.storage.Download(filePath)
	if err != nil {
		return "", fmt.Errorf("下载原始音频失败: %w", err)
	}

	tmpInput, err := os.CreateTemp("", "asr-input-*"+ext)
	if err != nil {
		return "", fmt.Errorf("创建临时输入文件失败: %w", err)
	}
	defer os.Remove(tmpInput.Name())
	defer tmpInput.Close()

	if _, err := tmpInput.Write(srcData); err != nil {
		return "", fmt.Errorf("写入临时输入文件失败: %w", err)
	}
	tmpInput.Close()

	// 2. ffmpeg 转换为 16kHz 单声道 WAV
	tmpOutput := tmpInput.Name() + "_16k.wav"
	defer os.Remove(tmpOutput)

	cmd := exec.Command("ffmpeg", "-y", "-i", tmpInput.Name(),
		"-ar", "16000", "-ac", "1", "-sample_fmt", "s16",
		"-f", "wav", tmpOutput)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg转换失败: %w, stderr: %s", err, stderr.String())
	}

	// 3. 读取转换后的文件
	convertedData, err := os.ReadFile(tmpOutput)
	if err != nil {
		return "", fmt.Errorf("读取转换后文件失败: %w", err)
	}

	// 4. 上传到 MinIO
	convertedPath := filePath[:len(filePath)-len(filepath.Ext(filePath))] + "_16k.wav"
	if err := s.storage.UploadBytes(convertedPath, convertedData, "audio/wav"); err != nil {
		return "", fmt.Errorf("上传转换后音频失败: %w", err)
	}

	return convertedPath, nil
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
	// 设置整体超时：每个 URL 最多 2 分钟，整体最多 10 分钟
	taskID := uuid.New().String()
	maxTimeout := 10 * time.Minute
	urlTimeout := time.Duration(len(seen)) * 2 * time.Minute
	if urlTimeout > maxTimeout {
		urlTimeout = maxTimeout
	}
	taskCtx, cancel := context.WithTimeout(context.Background(), urlTimeout)
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
	totalStart := time.Now()

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

	logger.Info("开始处理 URL 导入",
		zap.Uint("source_id", sourceID),
		zap.String("url", source.OriginalURL),
	)

	// 更新状态为 processing
	if err := s.sourceRepo.UpdateStatus(sourceID, "processing", ""); err != nil {
		logger.Warn("更新Source状态为processing失败", zap.Uint("source_id", sourceID), zap.Error(err))
	}

	// 转换 URL 内容
	stepStart := time.Now()
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
			logger.Error("URL 转换失败",
				zap.Uint("source_id", sourceID),
				zap.String("url", source.OriginalURL),
				zap.String("error_code", convertErr.Code),
				zap.String("detail", convertErr.DetailMsg),
				zap.Int("http_status", convertErr.HTTPStatus),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
			// 使用用户友好的错误消息
			userMsg = convertErr.UserMsg
		} else {
			// 未知错误类型
			logger.Error("URL 转换失败",
				zap.Uint("source_id", sourceID),
				zap.String("url", source.OriginalURL),
				zap.Duration("elapsed", time.Since(stepStart)),
				zap.Error(err),
			)
			userMsg = "网页内容获取失败，请稍后重试"
		}

		if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", userMsg); updateErr != nil {
			logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
		}
		return
	}

	logger.Info("URL 转换成功",
		zap.Uint("source_id", sourceID),
		zap.String("url", source.OriginalURL),
		zap.Int("content_len", len(markdown)),
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	// 转换完成后再检查一次 source 是否还存在（可能在转换期间被用户删除）
	existing, _ := s.sourceRepo.FindByID(sourceID)
	if existing == nil {
		logger.Info("Source已被删除，丢弃转换结果", zap.Uint("source_id", sourceID))
		return
	}

	// LLM 结构化
	stepStart = time.Now()
	if s.structurer != nil {
		result, err := s.structurer.Structure(taskCtx, source.UserID, markdown, StructureMeta{
			Title:      source.Name,
			SourceType: "url",
		})
		if err != nil {
			logger.Error("LLM 结构化失败，使用原始内容",
				zap.Uint("source_id", sourceID),
				zap.String("url", source.OriginalURL),
				zap.Duration("elapsed", time.Since(stepStart)),
				zap.Error(err),
			)
		} else if result.ActuallyCalled && result.Content != markdown {
			logger.Info("LLM 结构化成功，内容已优化",
				zap.Uint("source_id", sourceID),
				zap.Int("original_len", len(markdown)),
				zap.Int("structured_len", len(result.Content)),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
			markdown = result.Content
		} else if result.ActuallyCalled {
			logger.Info("LLM 判断内容已有结构，无需结构化",
				zap.Uint("source_id", sourceID),
				zap.Int("content_len", len(markdown)),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
		} else {
			logger.Warn("LLM 结构化被跳过（模型配置问题或 API Key 过期）",
				zap.Uint("source_id", sourceID),
				zap.Int("content_len", len(markdown)),
				zap.Duration("elapsed", time.Since(stepStart)),
			)
		}
	} else {
		logger.Warn("MarkdownStructurer 未配置，跳过结构化", zap.Uint("source_id", sourceID))
	}

	// 更新 Source 内容和状态为 ready
	stepStart = time.Now()
	source.MarkdownContent = markdown
	source.Status = "ready"
	if err := s.sourceRepo.Update(source); err != nil {
		logger.Error("更新Source内容失败", zap.Uint("source_id", sourceID), zap.Duration("elapsed", time.Since(stepStart)), zap.Error(err))
		if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", fmt.Sprintf("保存失败: %v", err)); updateErr != nil {
			logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
		}
		return
	}

	logger.Info("Source 记录更新成功",
		zap.Uint("source_id", sourceID),
		zap.String("url", source.OriginalURL),
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	// 同步触发 RAG 入库
	stepStart = time.Now()
	if s.ingestionSvc != nil {
		if err := s.ingestionSvc.IngestSingle(taskCtx, sourceID); err != nil {
			logger.Error("RAG 入库失败",
				zap.Uint("source_id", sourceID),
				zap.String("url", source.OriginalURL),
				zap.Duration("elapsed", time.Since(stepStart)),
				zap.Error(err),
			)
			if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", fmt.Sprintf("RAG 入库失败: %v", err)); updateErr != nil {
				logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
			}
			return
		}
		logger.Info("RAG 入库成功",
			zap.Uint("source_id", sourceID),
			zap.String("url", source.OriginalURL),
			zap.Duration("elapsed", time.Since(stepStart)),
		)
	}

	// 生成摘要（异步，不阻塞主流程）
	go s.generateAndSaveSummary(sourceID, source.UserID, markdown)

	logger.Info("URL 导入完成",
		zap.Uint("source_id", sourceID),
		zap.String("url", source.OriginalURL),
		zap.Duration("total_elapsed", time.Since(totalStart)),
	)
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

// summarySystemPrompt 摘要生成的系统提示词
const summarySystemPrompt = `你是一个资料摘要助手。请为以下文档内容生成一份简洁的摘要。

要求：
1. 摘要长度：200-400字
2. 涵盖文档的核心主题、主要观点和关键信息
3. 使用中文
4. 保持客观，不添加个人评价
5. 直接输出摘要内容，不要加任何前缀或解释`

// generateAndSaveSummary 生成资料摘要并保存到 MySQL 和 Redis（importerService 的方法）
func (s *importerService) generateAndSaveSummary(sourceID uint, userID uint, content string) {
	doGenerateAndSaveSummary(s.sourceRepo, s.configSvc, s.summaryCache, sourceID, userID, content)
}

// fallbackSummaryLength 降级摘要的最大字符数
const fallbackSummaryLength = 300

// doGenerateAndSaveSummary 生成资料摘要的包级别共享实现
// LLM 失败时自动降级为截取内容前 N 个字符作为兜底摘要
func doGenerateAndSaveSummary(
	sourceRepo repository.SourceRepository,
	configSvc ConfigService,
	summaryCache *cache.SourceSummaryCache,
	sourceID uint, userID uint, content string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	startTime := time.Now()

	summary, usedFallback := tryGenerateWithLLM(ctx, configSvc, userID, content)
	if usedFallback {
		// LLM 失败，使用降级摘要
		summary = buildFallbackSummary(content)
		logger.Warn("LLM 摘要生成失败，使用降级摘要",
			zap.Uint("source_id", sourceID),
			zap.Int("fallback_len", len(summary)),
		)
	}

	if summary == "" {
		logger.Warn("摘要生成失败且内容为空，跳过",
			zap.Uint("source_id", sourceID),
		)
		return
	}

	// 保存到 MySQL
	if err := sourceRepo.UpdateSummary(sourceID, summary); err != nil {
		logger.Error("保存摘要到 MySQL 失败",
			zap.Uint("source_id", sourceID),
			zap.Error(err),
		)
		return
	}

	// 保存到 Redis
	if summaryCache != nil {
		if err := summaryCache.Set(ctx, sourceID, summary); err != nil {
			logger.Warn("保存摘要到 Redis 失败",
				zap.Uint("source_id", sourceID),
				zap.Error(err),
			)
		}
	}

	logger.Info("资料摘要生成完成",
		zap.Uint("source_id", sourceID),
		zap.Int("summary_len", len(summary)),
		zap.Bool("fallback", usedFallback),
		zap.Duration("elapsed", time.Since(startTime)),
	)
}

// tryGenerateWithLLM 尝试用 LLM 生成摘要，返回 (摘要内容, 是否需要降级)
func tryGenerateWithLLM(ctx context.Context, configSvc ConfigService, userID uint, content string) (string, bool) {
	chatModel, err := getChatModelForSummary(ctx, configSvc, userID)
	if err != nil || chatModel == nil {
		return "", true
	}

	userMsg := fmt.Sprintf("请为以下文档生成摘要：\n\n%s", content)
	msg, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage(summarySystemPrompt),
		schema.UserMessage(userMsg),
	}, model.WithMaxTokens(1024))
	if err != nil {
		return "", true
	}
	if msg == nil || strings.TrimSpace(msg.Content) == "" {
		return "", true
	}

	return strings.TrimSpace(msg.Content), false
}

// buildFallbackSummary 从内容中提取降级摘要 、截取前 fallbackSummaryLength 个字符，尝试在句子边界截断
func buildFallbackSummary(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	runes := []rune(content)
	if len(runes) <= fallbackSummaryLength {
		return content
	}

	// 截取前 N 个字符，尝试在句号、换行处断开
	truncated := runes[:fallbackSummaryLength]
	cutPoints := []rune{'。', '\n', '；', '！', '？', '.', '!', '?'}
	bestCut := fallbackSummaryLength
	for i := fallbackSummaryLength - 1; i >= fallbackSummaryLength/2; i-- {
		for _, cp := range cutPoints {
			if truncated[i] == cp {
				bestCut = i + 1
				break
			}
		}
		if bestCut != fallbackSummaryLength {
			break
		}
	}

	return string(runes[:bestCut]) + "..."
}

// getChatModelForSummary 获取用于生成摘要的 ChatModel（包级别共享函数）
func getChatModelForSummary(ctx context.Context, configSvc ConfigService, userID uint) (model.ToolCallingChatModel, error) {
	llmConfig, err := configSvc.GetUserLLMConfig(userID)
	if err != nil {
		return nil, fmt.Errorf("获取 LLM 配置失败: %w", err)
	}
	if llmConfig == nil || !llmConfig.Enabled {
		return nil, nil
	}

	chatModel, err := llm.NewChatModel(ctx, llmConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModel 失败: %w", err)
	}
	return chatModel, nil
}

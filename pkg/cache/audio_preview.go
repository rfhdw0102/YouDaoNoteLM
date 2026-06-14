package cache

import (
	"context"
	"fmt"
	"time"
)

// AudioPreview 音频预览缓存结构
type AudioPreview struct {
	PreviewID       string `json:"preview_id"`       // 预览ID(UUID)
	UserID          uint   `json:"user_id"`          // 所属用户
	NotebookID      uint   `json:"notebook_id"`      // 所属笔记本
	FileName        string `json:"file_name"`        // 文件名
	FilePath        string `json:"file_path"`        // 对象存储文件路径
	FileSize        int64  `json:"file_size"`        // 文件大小(字节)
	TranscribedText string `json:"transcribed_text"` // ASR转写文本
	Status          string `json:"status"`           // 状态: pending/processing/ready/failed
	ExpiresAt       int64  `json:"expires_at"`       // 过期时间戳
}

// AudioPreviewCache 音频预览缓存操作
type AudioPreviewCache struct {
	cache *Cache
}

// NewAudioPreviewCache 创建音频预览缓存
func NewAudioPreviewCache(cache *Cache) *AudioPreviewCache {
	return &AudioPreviewCache{cache: cache}
}

// key 前缀
const audioPreviewPrefix = "audio:preview:"

// Save 保存音频预览（按过期时间设置TTL）
func (c *AudioPreviewCache) Save(ctx context.Context, preview *AudioPreview) error {
	key := fmt.Sprintf("%s%s", audioPreviewPrefix, preview.PreviewID)
	expireAt := time.Unix(preview.ExpiresAt, 0)
	return c.cache.SetWithExpire(ctx, key, preview, expireAt)
}

// Get 获取音频预览
func (c *AudioPreviewCache) Get(ctx context.Context, previewID string) (*AudioPreview, error) {
	key := fmt.Sprintf("%s%s", audioPreviewPrefix, previewID)
	var preview AudioPreview
	err := c.cache.Get(ctx, key, &preview)
	if err != nil {
		return nil, err
	}
	return &preview, nil
}

// UpdateStatus 更新预览状态
func (c *AudioPreviewCache) UpdateStatus(ctx context.Context, previewID string, status string) error {
	preview, err := c.Get(ctx, previewID)
	if err != nil {
		return err
	}
	preview.Status = status
	return c.Save(ctx, preview)
}

// Delete 删除音频预览
func (c *AudioPreviewCache) Delete(ctx context.Context, previewID string) error {
	key := fmt.Sprintf("%s%s", audioPreviewPrefix, previewID)
	return c.cache.Delete(ctx, key)
}

// Exists 检查预览是否存在
func (c *AudioPreviewCache) Exists(ctx context.Context, previewID string) (bool, error) {
	key := fmt.Sprintf("%s%s", audioPreviewPrefix, previewID)
	return c.cache.Exists(ctx, key)
}

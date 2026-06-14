package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	recentMessagesKey = "chat:%d:recent_messages"
	summaryKey        = "chat:%d:summary"
	lockKey           = "chat:%d:lock"

	messagesTTL = 7 * 24 * time.Hour
	summaryTTL  = 7 * 24 * time.Hour
	lockTTL     = 120 * time.Second
)

// MessagePair 消息对
type MessagePair struct {
	User      string `json:"user"`
	Assistant string `json:"assistant"`
}

// ChatCache 对话缓存
type ChatCache struct {
	rdb *redis.Client
}

// NewChatCache 创建对话缓存
func NewChatCache(rdb *redis.Client) *ChatCache {
	return &ChatCache{rdb: rdb}
}

// GetRecentMessages 获取最近消息
func (c *ChatCache) GetRecentMessages(ctx context.Context, conversationID uint) ([]MessagePair, error) {
	key := fmt.Sprintf(recentMessagesKey, conversationID)
	data, err := c.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var pairs []MessagePair
	for _, item := range data {
		var pair MessagePair
		if err := json.Unmarshal([]byte(item), &pair); err != nil {
			continue
		}
		pairs = append(pairs, pair)
	}
	return pairs, nil
}

// AddMessage 添加消息对到缓存
func (c *ChatCache) AddMessage(ctx context.Context, conversationID uint, userMsg, assistantMsg string) error {
	key := fmt.Sprintf(recentMessagesKey, conversationID)

	pair := MessagePair{
		User:      userMsg,
		Assistant: assistantMsg,
	}
	data, err := json.Marshal(pair)
	if err != nil {
		return err
	}

	pipe := c.rdb.Pipeline()
	pipe.RPush(ctx, key, string(data))
	pipe.LTrim(ctx, key, -20, -1) // 保留最近 10 轮（20 条）
	pipe.Expire(ctx, key, messagesTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// GetSummary 获取对话摘要
func (c *ChatCache) GetSummary(ctx context.Context, conversationID uint) (string, bool, error) {
	key := fmt.Sprintf(summaryKey, conversationID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

// SetSummary 设置对话摘要
func (c *ChatCache) SetSummary(ctx context.Context, conversationID uint, summary string) error {
	key := fmt.Sprintf(summaryKey, conversationID)
	return c.rdb.Set(ctx, key, summary, summaryTTL).Err()
}

// AcquireLock 获取并发锁，返回 lockValue 用于释放
func (c *ChatCache) AcquireLock(ctx context.Context, conversationID uint) (string, bool, error) {
	key := fmt.Sprintf(lockKey, conversationID)
	lockValue := uuid.New().String()
	ok, err := c.rdb.SetNX(ctx, key, lockValue, lockTTL).Result()
	if err != nil {
		return "", false, err
	}
	return lockValue, ok, nil
}

var releaseLockScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	end
	return 0
`)

// ReleaseLock 释放并发锁
func (c *ChatCache) ReleaseLock(ctx context.Context, conversationID uint, lockValue string) error {
	key := fmt.Sprintf(lockKey, conversationID)
	return releaseLockScript.Run(ctx, c.rdb, []string{key}, lockValue).Err()
}

// DeleteConversationCache 删除对话的所有缓存（消息+摘要）
func (c *ChatCache) DeleteConversationCache(ctx context.Context, conversationID uint) error {
	keys := []string{
		fmt.Sprintf(recentMessagesKey, conversationID),
		fmt.Sprintf(summaryKey, conversationID),
	}
	return c.rdb.Del(ctx, keys...).Err()
}

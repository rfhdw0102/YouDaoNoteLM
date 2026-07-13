package search

import (
	"sync"

	bizerrors "YoudaoNoteLm/pkg/errors"
)

// userLimiter per-user 并发限流器（eino 未提供，自行实现）
// 用 per-user 的 buffered channel 做信号量，非阻塞获取，防止滥用和费用失控。
type userLimiter struct {
	mu    sync.Mutex
	semas map[uint]chan struct{}
	max   int
}

func newUserLimiter(max int) *userLimiter {
	return &userLimiter{semas: make(map[uint]chan struct{}), max: max}
}

// acquire 尝试获取用户的执行槽位。成功返回释放函数，失败返回业务错误。
// 调用方应在 defer 中调用返回的释放函数（流式场景在 goroutine 内 defer）。
func (l *userLimiter) acquire(userID uint) (func(), error) {
	ch := l.getOrCreate(userID)
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	default:
		return nil, bizerrors.New(bizerrors.CodeConflict, "请等待当前搜索任务完成")
	}
}

func (l *userLimiter) getOrCreate(userID uint) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	ch, ok := l.semas[userID]
	if !ok {
		ch = make(chan struct{}, l.max)
		l.semas[userID] = ch
	}
	return ch
}

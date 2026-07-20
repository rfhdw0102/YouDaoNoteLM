// task_event_hub.go 实现任务事件订阅中心。
//
// generationTaskEventHub 维护订阅者列表，publish 方法将任务状态变更事件
// 推送给所有匹配的订阅者（按 userID 和 notebookID 过滤）。
//
// 推送策略：
//   - 每个 subscriber 拥有独立的有缓冲 channel（generationTaskEventChannelSize=256）
//   - channel 满时优先丢弃最旧的非终态事件（pending/running），保留终态事件
//     （completed/failed/cancelled），避免前端永久卡在 running
//   - WatchTasks（controller.go）消费 channel 中的事件并通过 WebSocket 推送给前端
//
// 该组件是进程内存级的，不跨实例广播。多实例部署时需配合 Redis Pub/Sub 扩展。
package generation

import (
	"sync"
)

// generationTaskEventChannelSize 订阅 channel 的缓冲区大小。
// 调大到 256 显著降低快速提交多个任务时丢弃关键状态事件的概率；
// 即使订阅者短暂消费缓慢，running/completed 事件也能留在 channel 中等待消费。
const generationTaskEventChannelSize = 256

type generationTaskSubscriber struct {
	id         uint64
	userID     uint
	notebookID uint
	ch         chan GenerationTaskEvent
}

type generationTaskEventHub struct {
	mu          sync.Mutex
	nextID      uint64
	subscribers map[uint64]*generationTaskSubscriber
}

func newGenerationTaskEventHub() *generationTaskEventHub {
	return &generationTaskEventHub{subscribers: map[uint64]*generationTaskSubscriber{}}
}

// subscribe 注册订阅者，返回事件 channel 和取消订阅函数。
func (h *generationTaskEventHub) subscribe(userID, notebookID uint) (<-chan GenerationTaskEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	sub := &generationTaskSubscriber{
		id:         h.nextID,
		userID:     userID,
		notebookID: notebookID,
		ch:         make(chan GenerationTaskEvent, generationTaskEventChannelSize),
	}
	h.subscribers[sub.id] = sub
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if existing, ok := h.subscribers[sub.id]; ok {
				delete(h.subscribers, sub.id)
				close(existing.ch)
			}
		})
	}
	return sub.ch, unsubscribe
}

// publish 推送事件到所有匹配的订阅者。
//
// 推送策略：channel 满时优先丢弃最旧的"非终态"事件，保留终态（completed/failed/cancelled）。
// 这样即使订阅者短暂消费缓慢，关键状态变更也能被前端感知。
// 如果旧事件全是终态（理论上不该出现），则丢弃最旧的一条腾出空间。
func (h *generationTaskEventHub) publish(event GenerationTaskEvent) {
	if event.Task == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, sub := range h.subscribers {
		if sub.userID != event.Task.UserID {
			continue
		}
		if sub.notebookID != 0 && sub.notebookID != event.Task.NotebookID {
			continue
		}
		eventCopy := event
		eventCopy.Task = cloneGenerationTask(event.Task)
		select {
		case sub.ch <- eventCopy:
		default:
			// channel 满：丢弃最旧的非终态事件，为当前事件腾出空间。
			// 终态事件（completed/failed/cancelled）必须保留，否则前端会永久卡在 running。
			dropOldestNonTerminal(sub.ch)
			select {
			case sub.ch <- eventCopy:
			default:
				// 极端情况：所有事件都是终态，丢弃最旧的一条。
				<-sub.ch
				sub.ch <- eventCopy
			}
		}
	}
}

// dropOldestNonTerminal 从 channel 中尝试弹出一个最旧的非终态事件。
// 如果前若干条都是终态事件，则保留它们，避免丢失关键状态。
func dropOldestNonTerminal(ch chan GenerationTaskEvent) {
	for i := 0; i < 8; i++ {
		select {
		case ev := <-ch:
			if ev.Task != nil && isTerminalTaskStatus(ev.Task.Status) {
				// 终态事件不能丢，放回 channel 尾部。
				// 注意：放回后 channel 仍满，下一次 select default 会走到 <-ch 丢弃最旧的。
				ch <- ev
				return
			}
			// 非终态事件（pending/running）丢弃，腾出空间。
			return
		default:
			return
		}
	}
}

// isTerminalTaskStatus 判断任务状态是否为终态。
func isTerminalTaskStatus(status GenerationTaskStatus) bool {
	return status == GenerationTaskStatusCompleted ||
		status == GenerationTaskStatusFailed ||
		status == GenerationTaskStatusCancelled
}

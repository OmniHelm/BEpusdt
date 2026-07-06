package task

import (
	"context"
	"sync"
	"time"

	"github.com/v03413/bepusdt/app/model"
)

// 区块扫描队列最大长度，避免可能因为 Rpc Rate Limit 问题导致消费队列堆积，进而导致OOM，暂时简单限制队列长度
// 如果直接使用固定长度的 Channel 控制，会导致区块高度同步时也彻底阻塞，无法对外界输出日志导致无法观察，彻底垮掉
// 如果确实是因为 Rate Limit 问题导致的异常，优先考虑的是提升 Rpc 节点的质量和稳定性
const blockQueueLimit = 100

type Task struct {
	Duration time.Duration
	Callback func(ctx context.Context)
}

var (
	tasks []Task
	mu    sync.Mutex
)

func Init() error {
	model.RefreshC()

	bscInit()
	ethInit()
	plasmaInit()
	polygonInit()
	arbitrumInit()
	xlayerInit()
	baseInit()

	return nil
}

func Register(t Task) {
	mu.Lock()
	defer mu.Unlock()

	if t.Callback == nil {

		panic("Task Callback cannot be nil")
	}

	tasks = append(tasks, t)
}

// waitQueueIdle 阻塞等待扫描队列长度降到拥堵阈值以下，ctx 取消时返回 false。
// 回溯推送用它取代旧的"拥堵即放弃"逻辑：一次回溯要么完整推完所有区块，
// 要么随进程退出（内存标记同时消失），不存在推送到一半被跳过却再也不重试的状态。
func waitQueueIdle(ctx context.Context, queueLen func() int) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		if queueLen() < blockQueueLimit {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Second * 3):
		}
	}
}

func Start(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()

	for _, t := range tasks {
		go func(t Task) {
			if t.Duration <= 0 {
				t.Callback(ctx)

				return
			}

			t.Callback(ctx)

			ticker := time.NewTicker(t.Duration)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					t.Callback(ctx)
				}
			}
		}(t)
	}
}

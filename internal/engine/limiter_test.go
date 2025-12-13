package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alitto/pond/v2"
)

func TestLimit(t *testing.T) {
	limiter := NewSmartLimiter(0.5, 1, 100, 500)

	// 2. 创建 Pond 池
	pool := pond.NewPool(5, pond.WithQueueSize(100)) // 最大并发 5，任务队列最多 50

	//var wg sync.WaitGroup
	totalTasks := 5

	fmt.Println("Start...")
	ctx := context.Background()

	group := pool.NewGroup()
	for i := 1; i <= totalTasks; i++ {
		//wg.Add(1)
		taskID := i

		group.Submit(func() {
			//defer wg.Done()

			t1 := time.Now()

			// 等待限流器许可
			if err := limiter.Wait(ctx); err != nil {
				fmt.Printf("Task %d limiter error: %v\n", taskID, err)
				return
			}

			t2 := time.Now()

			fmt.Printf("[%s] Task %d START after limiter wait: %s\n",
				t2.Format("15:04:05.000"),
				taskID,
				t2.Sub(t1),
			)

			// 模拟任务执行
			time.Sleep(200 * time.Millisecond)

			fmt.Printf("[%s] Task %d END\n",
				time.Now().Format("15:04:05.000"),
				taskID,
			)
		})
	}

	//wg.Wait()
	group.Wait()

	fmt.Println("Done.")

}

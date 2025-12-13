package engine

import (
	"context"
	"math/rand"
	"time"

	"golang.org/x/time/rate"
)

type SmartLimiter struct {
	limiter *rate.Limiter
	minWait time.Duration // 最小随机等待
	maxWait time.Duration // 最大随机等待
}

// NewSmartLimiter 创建一个智能限流器
// r: 每秒允许的请求数 (QPS)，例如 0.5 表示每 2 秒 1 次
// burst: 允许的突发数，例如 1 表示不允许突发
func NewSmartLimiter(r float64, burst int, minMs, maxMs int) *SmartLimiter {
	return &SmartLimiter{
		limiter: rate.NewLimiter(rate.Limit(r), burst),
		minWait: time.Duration(minMs) * time.Millisecond,
		maxWait: time.Duration(maxMs) * time.Millisecond,
	}
}

// Wait 等待许可，并叠加随机抖动
func (s *SmartLimiter) Wait(ctx context.Context) error {
	// 1. 硬限流：等待令牌桶（保证不会超速）
	if err := s.limiter.Wait(ctx); err != nil {
		return err
	}

	// 2. 软伪装：增加随机抖动（Jitter）
	// 如果配置了随机时间，则额外睡一会儿
	if s.maxWait > 0 {
		// 生成 [minWait, maxWait) 范围内的随机时间
		jitter := s.minWait + time.Duration(rand.Int63n(int64(s.maxWait-s.minWait)))
		time.Sleep(jitter)
	}

	return nil
}

package utils

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

const nameWidth = 28

var (
	once      sync.Once
	container *mpb.Progress
)

func ensure() {
	once.Do(func() {
		// 非终端环境（输出被重定向、后台服务等）不渲染进度条，避免写入控制字符
		if !isatty.IsTerminal(os.Stderr.Fd()) && !isatty.IsCygwinTerminal(os.Stderr.Fd()) {
			return
		}
		container = mpb.New(mpb.WithRefreshRate(200 * time.Millisecond))
	})
}

// Bar 进度条的 nil 安全封装：非终端环境下 container 为空，各方法自动退化为空操作
type Bar struct {
	inner *mpb.Bar
}

// Increment 完成计数 +1（作品级进度条用）
func (b *Bar) Increment() {
	if b != nil && b.inner != nil {
		b.inner.Increment()
	}
}

// Complete 以 current 为最终值触发完成并移除进度条
func (b *Bar) Complete(current int64) {
	if b != nil && b.inner != nil {
		b.inner.SetTotal(current, true)
	}
}

// Abort 放弃进度条并立即移除（下载失败/中断时用，避免阻塞 Wait）
func (b *Bar) Abort() {
	if b != nil && b.inner != nil {
		b.inner.Abort(false)
	}
}

// ProxyReader 包装响应体以测量速率，供 Ewma 速度/ETA 装饰器使用；无进度条时原样返回
func (b *Bar) ProxyReader(r io.Reader) io.Reader {
	if b == nil || b.inner == nil {
		return r
	}
	proxy := b.inner.ProxyReader(r)
	if proxy == nil {
		return r
	}
	return proxy
}

// Wait 阻塞直到所有进度条完成并移除；非终端环境为空操作
func Wait() {
	ensure()
	if container != nil {
		container.Wait()
	}
}

// AddFileBar 新建单文件下载进度条，展示百分比、已下载/总大小、速率与剩余时间；
// total<=0（服务器未返回 Content-Length，如透明 gzip 解压）时为不定长模式。
func AddFileBar(name string, total int64) *Bar {
	ensure()
	if container == nil {
		return nil
	}

	name = truncate(name, nameWidth)
	prepend := []decor.Decorator{
		decor.Name(name, decor.WC{W: nameWidth + 2, C: decor.DindentRight}),
		decor.CountersKibiByte("% .2f / % .2f", decor.WC{W: 24, C: decor.DindentRight}),
	}
	appendChild := []decor.Decorator{}
	if total > 0 {
		appendChild = append(appendChild, decor.Percentage(decor.WC{W: 7}))
	}
	appendChild = append(appendChild,
		decor.EwmaSpeed(decor.SizeB1024(0), "% .2f/s", 30, decor.WC{W: 18}),
		decor.EwmaETA(decor.ET_STYLE_GO, 30, decor.WC{W: 10}),
	)

	bar, err := container.Add(total,
		mpb.BarStyle().Lbound("[").Rbound("]").Filler("=").Tip(">").Padding("-").Build(),
		mpb.PrependDecorators(prepend...),
		mpb.AppendDecorators(appendChild...),
		mpb.BarRemoveOnComplete(),
	)
	if err != nil {
		return nil
	}
	return &Bar{inner: bar}
}

// AddCountBar 新建作品级进度条，统计已完成的文件数（如 RJ01234567  12/34 个文件）
func AddCountBar(name string, total int64) *Bar {
	ensure()
	if container == nil {
		return nil
	}

	name = truncate(name, nameWidth)
	bar, err := container.Add(total,
		mpb.BarStyle().Lbound("[").Rbound("]").Filler("=").Tip(">").Padding("-").Build(),
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: nameWidth + 2, C: decor.DindentRight}),
			decor.CountersNoUnit("%d / %d 个文件", decor.WC{W: 20, C: decor.DindentRight}),
		),
		mpb.AppendDecorators(decor.Percentage(decor.WC{W: 7})),
		mpb.BarRemoveOnComplete(),
	)
	if err != nil {
		return nil
	}
	return &Bar{inner: bar}
}

// truncate 按显示宽度截断过长的文件名（CJK 字符按 2 列计），尾部补省略号
func truncate(s string, w int) string {
	if runewidth.StringWidth(s) <= w {
		return s
	}
	cur := 0
	for i, r := range s {
		rw := runewidth.RuneWidth(r)
		if cur+rw > w-2 {
			return s[:i] + "…"
		}
		cur += rw
	}
	return s
}

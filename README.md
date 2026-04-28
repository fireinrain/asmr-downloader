
<div align="right">
  <details>
    <summary >🌐 Language</summary>
    <div>
      <div align="center">
        <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=en">English</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=zh-CN">简体中文</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=zh-TW">繁體中文</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=ja">日本語</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=ko">한국어</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=hi">हिन्दी</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=th">ไทย</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=fr">Français</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=de">Deutsch</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=es">Español</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=it">Italiano</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=ru">Русский</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=pt">Português</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=nl">Nederlands</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=pl">Polski</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=ar">العربية</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=fa">فارسی</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=tr">Türkçe</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=vi">Tiếng Việt</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=id">Bahasa Indonesia</a>
        | <a href="https://openaitx.github.io/view.html?user=fireinrain&project=asmr-downloader&lang=as">অসমীয়া</
      </div>
    </div>
  </details>
</div>


## 📖 项目简介

ASMRoner 是一款 Go 语言命令行工具，用于搜索、下载、同步 asmr.one 音声作品，并提供简易 Web 播放界面。

> 🌐 衍生作品：[asmr.furina.in](https://asmr.furina.in) — 一个简洁干净的在线 ASMR 听音页面

## 🚀 快速开始

```bash
https://github.com/MIKANOoOo/asmr-downloader.git && cd asmroner
go build -o asmroner
./asmroner config   # 交互式初始化配置
```

## 📋 常用命令

```bash
# 搜索
./asmroner search "护士" -c 20
./asmroner search "护士,-中出@duration:1h" -c 50

# 下载
./asmroner download RJ01037721 -d ./downloads
./asmroner download RJ01037721,RJ02000001 -d ./downloads
./asmroner download hot100 -n 10 -d ./downloads

# 搜索 + 下载/导出
./asmroner search download "护士" -d ./downloads -s 20
./asmroner search export "护士" -n 100 -f data.json

# 同步元数据 & 批量下载
./asmroner sync
./asmroner sync download -d ./downloads
./asmroner sync retry -d ./downloads
./asmroner sync report

  # 导出单个作品或指定数量热门榜链接 & 导出到指定目录
./asmroner export RJ01544940 -o ./downloads
./asmroner export hot100 -n 20 -o ./downloads
./asmroner export hot100 -n 10 -o ./downloads
更多内容参考常见问题中的guide

# Web 播放界面
./asmroner listen -p 8080 ./syncdata
```

## 📸 截图

| 配置 | 搜索 |
|:---:|:---:|
| ![配置](dist/config.png) | ![搜索](dist/search.png) |
| **下载** | **同步** |
| ![下载](dist/download.png) | ![同步](dist/sync.png) |
| **同步下载** | **统计** |
| ![同步下载](dist/sync-down.png) | ![统计](dist/sync-report.png) |
| **Web 界面** | **Web 界面 2** |
| ![Web界面](dist/listen.png) | ![Web界面2](dist/listen2.png) |
| **export 界面** | **export 界面 2** |
| ![export界面](dist/export1.png) | ![export界面2](dist/export2.png) |

<details>
<summary><b>✨ 功能特性</b></summary>

- **搜索**：单个/批量 RJID、高级搜索语法、结果导出 CSV/JSON
- **下载**：单个/批量/热门作品下载，自动限流、重试、指数退避
- **同步**：元数据同步、批量下载控制、状态跟踪、失败重试
- **Web 界面**：可视化浏览、浏览器内播放、响应式设计
- **配置**：交互式初始化，支持代理、限流、抖动等高级配置

</details>

<details>
<summary><b>⚙️ 配置文件说明</b></summary>

配置文件路径：`~/.asmroner/config.toml`（TOML 格式）

```toml
[user]
account = "guest"
password = "guest"

[downloader]
api_url = ""                # 留空自动获取最快站点
proxy_url = ""              # 支持 http / socks5
max_workers = 5
max_retries = 3
sync_data_folder = "./syncdata"
sync_wanted_size = "200MB"  # 同步容量限制
prefer_media = "all"        # all | mp3>wav>flac

[limit]
sync_qps = 2
sync_jitter_min = 100       # ms
sync_jitter_max = 500
download_qps = 0.2
download_jitter_min = 2000
download_jitter_max = 5000
```

</details>

<details>
<summary><b>📋 命令选项速查</b></summary>

| 命令 | 选项 | 说明 |
|------|------|------|
| `search` | `-c` | 搜索结果数量（默认 10） |
| `search download` | `-d`, `-s` | 下载目录、下载数量 |
| `search export` | `-f`, `-n` | 导出文件名（.csv/.json）、导出数量 |
| `download` | `-d`, `-n` | 下载目录、hot100 数量 |
| `sync download` | `-d` | 下载目录 |
| `sync retry` | `-d` | 失败文件所在目录 |
| `sync export` | `-s`, `-f` | 状态（failed/success）、导出文件 |
| `listen` | `-p` | 端口（默认 9999） |
| `export` | `-o`, `-n` | 导出目录、hot100数量 |

</details>

<details>
<summary><b>📁 项目结构</b></summary>

```
asmroner/
├── cmd/                # 命令行接口（config/download/search/sync/listen）
├── internal/
│   ├── engine/        # 核心下载引擎（限流、重试、并发控制）
│   ├── logger/        # 结构化日志系统
│   ├── model/         # 数据模型与查询参数解析
│   ├── database/      # SQLite 数据库
│   ├── consts/        # 常量定义
│   └── utils/         # 工具函数
├── webui/             # 内嵌 Web 界面（Tailwind + Plyr）
├── main.go
└── go.mod
```

</details>

<details>
<summary><b>🛠 技术栈</b></summary>

| 组件 | 用途 |
|------|------|
| Cobra + Viper | CLI 框架 + 配置管理 |
| GORM + SQLite | 数据持久化 |
| Resty | HTTP 客户端（支持 HTTP/SOCKS5 代理） |
| Pond | 并发工作池 |
| x/time/rate | 令牌桶限流 |
| Gin | Web 服务 |
| Tailwind + Plyr | 前端界面 + 音频播放 |

</details>

<details>
<summary><b>🔧 常见问题</b></summary>

**配置文件未找到** → 运行 `./asmroner config` 初始化

**下载失败（stream error）** → 程序会自动重试；若仍失败，用 `sync retry` 重试，或查看 `.asmroner-data/download_errors.log`

**Web 界面无法访问** → 确认端口未被占用，尝试 `-p` 指定其他端口

**搜索结果为空** → 检查查询语法，尝试简化条件

**export指令配套的下载方式** → 参考[guide](/dist/guide.pdf) 

</details>

## 🤝 贡献

欢迎提交 Pull Request！Fork → 新建分支 → 提交更改 → 开启 PR。

## 📄 许可证

本项目采用 MIT 许可证，详情请查看 [LICENSE](/LICENSE) 文件。

## 🙏 致谢

- 特别感谢 [go-asmr-spider](https://github.com/DiheChen/go-asmr-spider)
- 感谢所有贡献者和用户！

---

**ASMRoner** — 每天晚上都有不同的妹妹陪你入睡 :)

*最后更新：2026 年 2 月*

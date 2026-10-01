<div align="right">
  <b>English</b> | <a href="README.md">简体中文</a>
</div>

## 📖 Introduction

ASMRoner is a Go command-line tool for searching, downloading, and syncing ASMR works from asmr.one, with a built-in web player interface.

> 🌐 Derivative: [asmr.furina.in](https://asmr.furina.in) — a clean and simple online ASMR listening page

## 🚀 Quick Start

### 1. Clone & Build

```bash
git clone https://github.com/MIKANOoOo/asmr-downloader.git
cd asmroner
go build -o asmroner
```

### 2. Initialize Config

```bash
./asmroner config
```

This launches an interactive setup wizard. Here are the key items explained:

| Item                   | Description                                                                                                                                   |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| **Account / Password** | **Your asmr.one website login credentials**. If you haven't registered, just press Enter to use the default `guest` / `guest` — it works fine |
| API URL                | Leave empty for the default official API (api.asmr-300.com), or set a custom one                                                              |
| Proxy URL              | Press Enter to skip if no proxy needed; supports `http://`, `https://`, `socks5://` (with auth)                                               |
| Sync data folder       | Where downloaded files are saved, default `./syncdata`                                                                                        |
| Sync size limit        | Max total download size for `sync download`, e.g. `200MB`, `2GB`                                                                              |

> 💡 For all other options, you can simply press Enter to use the defaults. You can always run `./asmroner config` again to change them later.

### 3. Start Using

```bash
./asmroner search "nurse" -c 10        # Search works
./asmroner download RJ01037721         # Download a work
./asmroner listen -p 8080 ./syncdata   # Start web player
```

## 📋 Common Commands

```bash
# Show version
./asmroner version

# Search
./asmroner search "nurse" -c 20
./asmroner search "nurse,-creampie@duration:1h" -c 50

# Download
./asmroner download RJ01037721 -d ./downloads
./asmroner download RJ01037721,RJ02000001 -d ./downloads
./asmroner download hot100 -n 10 -d ./downloads
./asmroner download RJ01037721 -f

# List file types (with counts) inside a work's resource directory
./asmroner list RJ01412863

# List all files in a work's resource directory
./asmroner list RJ01412863 -d

# Search + Download/Export
./asmroner search download "nurse" -d ./downloads -s 20
./asmroner search export "nurse" -n 100 -f data.json

# Sync metadata & batch download
./asmroner sync
./asmroner sync download -d ./downloads
./asmroner sync retry -d ./downloads
./asmroner sync report

# Export download links (generates links.txt + Aria2/IDM scripts)
./asmroner export RJ01544940 -o ./downloads
./asmroner export hot100 -n 20 -o ./downloads

# Web player interface
./asmroner listen -p 8080 ./syncdata
```

## 📸 Screenshots

|                Config                |               Search                |
| :----------------------------------: | :---------------------------------: |
|      ![Config](dist/config.png)      |     ![Search](dist/search.png)      |
|             **Download**             |              **Sync**               |
|    ![Download](dist/download.png)    |       ![Sync](dist/sync.png)        |
|          **Sync Download**           |           **Statistics**            |
| ![Sync Download](dist/sync-down.png) | ![Statistics](dist/sync-report.png) |
|              **Web UI**              |            **Web UI 2**             |
|      ![Web UI](dist/listen.png)      |    ![Web UI 2](dist/listen2.png)    |
|              **Export**              |            **Export 2**             |
|     ![Export](dist/export1.png)      |    ![Export 2](dist/export2.png)    |

<details>
<summary><b>✨ Features</b></summary>

- **Search**: Single/batch RJID lookup, advanced search syntax (filter by tag/circle/VA/duration/rating/price/sales/age/language), auto page-merging, CSV/JSON export
- **Download**: Single/batch/hot ranking downloads, automatically skips existing files (`--force` to overwrite), audio format priority (mp3>wav>flac), extension whitelist/blacklist filtering, auto rate-limiting, retry with exponential backoff, Worker Pool concurrency
- **Export**: Export download link lists — generates `links.txt` + `idm_download.bat` + `aria2_download.sh` per work, supports single work and hot ranking batch export
- **Sync**: Full metadata sync (with sync rate statistics), capacity-aware batch download control, status tracking (PENDING/COMPLETED/FAILED), retry failed downloads, CSV/JSON status export
- **Web UI**: Visual browsing, in-browser audio playback, paginated API, auto-open browser, graceful shutdown
- **Config**: Interactive setup, HTTP/SOCKS5 proxy support (with auth), token bucket rate-limiting + random jitter, custom folder naming (`{rjid}` `{date}` `{subtitle}` `{title}`), IDM path config
- **Site Config**: Uses the default official API (api.asmr-300.com), with an optional custom endpoint in config
- **Version Info**: Built-in version, build time, author info (supports ldflags injection at compile time)

</details>

<details>
<summary><b>🔍 Advanced Search Syntax</b></summary>

Full asmr.one search syntax: `keyword,exclusion@filters?pagination`

Strict grammar rules (invalid input fails with an error):

1. The three parts are separated explicitly by `@` and `?`; unneeded parts may be omitted entirely (no `@` when there are no filters)
2. Multiple values within a part are always comma-separated
3. The `-` prefix means exclude / negate
4. Filters must go after `@` — placing a filter before `@` is an error
5. The same filter / pagination param may appear only once

**Syntax grammar** (EBNF):

```ebnf
query        = withKeywords | filtersOnly ;

withKeywords = keywords , [ "@" , filters ] , [ "?" , params ] ;
filtersOnly  = "@" , filters , [ "?" , params ] ;

keywords     = keyword , { "," , keyword } ;
keyword      = [ "-" ] , text ;                    (* "-" prefix = exclude *)

filters      = filter , { "," , filter } ;
filter       = [ "-" ] , key , ":" , value ;       (* "-" prefix = negate *)
             (* key ∈ tag | circle | va | duration | rate | price | sell | age | lang *)

params       = param , { "&" , param } ;
param        = key , "=" , value ;
             (* key ∈ order | sort | subtitle | page | pageSize | includeTranslationWorks *)
```

Constraints beyond the grammar: at least one part must be non-empty (`?params` alone is invalid); no duplicate `key` within `filters` or within `params`; `text`/`value` must not contain the delimiters `,` `@` `?`; allowed `value`s are listed in the tables below.

| Form                             | Meaning                           |
| -------------------------------- | --------------------------------- |
| `nurse`                          | keyword only                      |
| `nurse@duration:1h`              | keyword + filters                 |
| `@va:陽向葵ゅか?subtitle=1`      | filters + pagination only         |
| `nurse?order=dl_count&sort=desc` | keyword + pagination (no filters) |

**Plain keywords** (before `@`, comma-separated):

| Form            | Description                       |
| --------------- | --------------------------------- |
| `nurse`         | single keyword                    |
| `nun,lotion`    | multiple keywords, all must match |
| `nun,-tentacle` | `-` prefix excludes a keyword     |

> 💡 Unspecified pagination params fall back to defaults (order=release / sort=desc / page=1 / pageSize=20).

**Filters** (after `@`, comma-separated, prefix `-` for negation):

| Filter   | Format                        | Description                                      |
| -------- | ----------------------------- | ------------------------------------------------ |
| tag      | `tag:loli/nurse`              | Filter by tag (supports `/` for multiple values) |
| circle   | `circle:CircleName`           | Filter by circle                                 |
| va       | `va:VA Name`                  | Filter by voice actor                            |
| duration | `duration:1h`                 | Duration greater than specified                  |
| rate     | `rate:4.5`                    | Rating greater than specified                    |
| price    | `price:1000` or `-price:2000` | Price filter (negation = less than)              |
| sell     | `sell:500`                    | Sales count greater than specified               |
| age      | `age:adult`                   | Age rating                                       |
| lang     | `lang:JPN` or `-lang:JPN`     | Language filter/exclusion                        |

**Pagination** (after `?`, `&`-separated):

| Param                   | Description              | Values                                                                                          |
| ----------------------- | ------------------------ | ----------------------------------------------------------------------------------------------- |
| order                   | Sort field               | release / dl_count / create_date / rating / price / rate_average_2dp / review_count / id / nsfw |
| sort                    | Sort direction           | desc / asc                                                                                      |
| subtitle                | Subtitle filter          | 0 (all) / 1 (subtitled only)                                                                    |
| page                    | Page number              | positive integer                                                                                |
| pageSize                | Items per page           | positive integer                                                                                |
| includeTranslationWorks | Include translated works | true / false                                                                                    |

**Examples**:

```bash
# 1) Plain / multiple keywords
./asmroner search "nurse" -c 10
./asmroner search "nun,lotion" -c 10

# 2) Keyword + exclusion
./asmroner search "nun,-tentacle" -c 10

# 3) Filters only (keyword omitted)
./asmroner search "@va:陽向葵ゅか" -c 20
./asmroner search "@tag:耳かき,age:adult" -c 20

# 4) Keyword + filters (duration > 1h)
./asmroner search "nurse,-creampie@duration:1h" -c 50

# 5) Filters + sorting / subtitles
./asmroner search "nurse,-creampie@duration:1h?order=dl_count&sort=desc" -c 50
./asmroner search "@va:陽向葵ゅか,age:adult?subtitle=1" -c 20

# 6) Pagination: start from page 2 (-c no larger than pageSize returns that page directly)
./asmroner search "nurse?order=dl_count&sort=desc&page=2&pageSize=50" -c 50

# 7) Negated filters: exclude tag / language / free works
./asmroner search "@tag:creampie,-lang:JPN" -c 20
./asmroner search "@-price:0" -c 20

# 8) Multiple conditions: sales > 500 and rating > 4.0
./asmroner search "@sell:500,rate:4.0" -c 20

# 9) Search by RJID (single / batch)
./asmroner search "RJ01037721"
./asmroner search "RJ01037721,RJ02000001"

# 10) Search + download / export, all syntax above works here too
./asmroner search download "@va:陽向葵ゅか?subtitle=1" -d ./downloads -s 20
./asmroner search export "nurse,-creampie@duration:1h?order=dl_count&sort=desc" -n 100 -f data.json
```

</details>

<details>
<summary><b>⚙️ Config File Reference</b></summary>

Config file path: `.asmroner-data/config.toml` (TOML format)

```toml
[user]
account = "guest"
password = "guest"

[downloader]
api_url = ""                # API endpoint, leave empty for the default official API https://api.asmr-300.com
proxy_url = ""              # Supports http / https / socks5 (with username/password auth)
max_workers = 5             # Concurrent worker count
max_retries = 3             # Max download retries
sync_data_folder = "./syncdata"
sync_wanted_size = "200MB"  # Sync size limit (supports MB/GB/TB/PB)
prefer_media = "all"        # Audio format priority: all | mp3>wav>flac
include_ext = ""            # Extension whitelist, e.g. ".mp3,.png,.jpg" (only download matches, empty = no filter)
exclude_ext = ""            # Extension blacklist, e.g. ".mp4,.webm" (exclude matches, empty = no filter)
                            # Exact suffix matching with nested extension support:
                            # ".vtt" matches all subtitle files
                            # ".mp3.vtt" only matches mp3 subtitle files
include_keyword = ""        # Path keyword whitelist, e.g. "SEあり" (matches dirs + filenames, empty = no filter)
exclude_keyword = ""        # Path keyword blacklist, e.g. "SEなし,no se" (whitelist applied before blacklist)
download_dir = ""           # Default download directory, empty = download uses current dir (-d takes priority)
folder_name_format = ""     # Download folder naming format, placeholders: {rjid} {date} {subtitle} {title}
                            # Leave empty for default: {rjid}-{date}-{subtitle}-{title}
idm_path = ""               # IDM installation path (optional, used by export command for generating scripts)

[limit]
sync_qps = 2                # Sync metadata QPS
sync_jitter_min = 100       # Min random jitter for sync requests (ms)
sync_jitter_max = 500       # Max random jitter for sync requests (ms)
download_qps = 0.2          # Download QPS
download_jitter_min = 2000  # Min random jitter for download requests (ms)
download_jitter_max = 5000  # Max random jitter for download requests (ms)
```

</details>

<details>
<summary><b>📋 Command Reference</b></summary>

| Command           | Options          | Description                                                                                     |
| ----------------- | ---------------- | ----------------------------------------------------------------------------------------------- |
| `version`         | —                | Show version, build time, author info                                                           |
| `config`          | —                | Interactive config init or reset                                                                |
| `search`          | `-c`             | Search result count (default 10, auto page-merge)                                               |
| `search download` | `-d`, `-s`       | Download directory, download count (default 100)                                                |
| `search export`   | `-f`, `-n`       | Export filename (.csv/.json), export count (default 100)                                        |
| `list`            | `-d`             | Show file types (with counts) inside a work's resource directory; `-d/--detail` lists all files |
| `download`        | `-d`, `-n`, `-f` | Download directory, hot100 mode count, force overwrite existing files                           |
| `export`          | `-o`, `-n`       | Output directory, hot100 mode count                                                             |
| `sync`            | —                | Sync metadata only (compares local vs remote, shows sync rate)                                  |
| `sync download`   | `-d`             | Sync then batch download with size limit and status tracking                                    |
| `sync retry`      | `-d`             | Clear old directory and retry failed downloads                                                  |
| `sync export`     | `-s`, `-f`       | Status filter (failed/success), export filename (.csv/.json)                                    |
| `sync report`     | —                | Print download statistics (total/completed/failed/pending)                                      |
| `listen`          | `-p`             | Web player port (default 9999), with paginated API                                              |

</details>

<details>
<summary><b>📁 Project Structure</b></summary>

```
asmroner/
├── main.go             # Entry point, registers version command
├── cmd/                # CLI commands (config/download/export/search/sync/listen)
├── internal/
│   ├── engine/        # Core engine (rate limiting, retry, concurrency, script generation, site detection)
│   ├── logger/        # Structured logging system
│   ├── model/         # Data models, query parameter parsing, config structs
│   ├── database/      # SQLite database (GORM)
│   ├── consts/        # Constants (API paths, regex, User-Agent)
│   └── utils/         # Utilities (file naming, directory size, CSV/JSON export)
├── webui/             # Embedded web UI (Gin + embedded static assets)
├── version.go         # Version info (ldflags injection)
└── go.mod
```

</details>

<details>
<summary><b>🛠 Tech Stack</b></summary>

| Component                      | Purpose                                                   |
| ------------------------------ | --------------------------------------------------------- |
| Cobra + Viper                  | CLI framework + config management (TOML)                  |
| GORM + SQLite                  | Data persistence                                          |
| Resty + golang.org/x/net/proxy | HTTP client (HTTP/HTTPS/SOCKS5 proxy support)             |
| Pond                           | Concurrent worker pool                                    |
| golang.org/x/time/rate         | Token bucket rate limiting + random jitter (SmartLimiter) |
| Gin                            | Web server                                                |
| embed                          | Embedded frontend static assets                           |

</details>

<details>
<summary><b>🔧 FAQ</b></summary>

**Config file not found** → Run `./asmroner config` to initialize

**Download failed (stream error / connection reset, etc.)** → The program will auto-retry with exponential backoff. If still failing, use `./asmroner sync retry -d <dir>` or check `.asmroner-data/download_errors.log`

**Web UI not accessible** → Confirm the port is not in use; try `-p` with a different port

**Empty search results** → Check query syntax and try simplifying; test with `./asmroner search "RJ01037721"` for a single RJID

**How to download after exporting links** → The export command generates `idm_download.bat` (Windows) and `aria2_download.sh` (cross-platform) inside each work's `download_scripts/` directory, along with `links.txt` in each subfolder

**Sync shows "local data logic error"** → The local SQLite database may have duplicate entries; delete `.asmroner-data/asmroner.db` and re-sync

**How to customize download folder names** → Configure `folder_name_format` in `config.toml` with placeholders `{rjid}` `{date}` `{subtitle}` `{title}`, e.g. `{rjid}-{title}`

**How to re-download without skipping existing files** → Use `-f` / `--force` to overwrite, e.g. `./asmroner download RJ01037721 -f`; without it, existing files are skipped automatically

</details>

## 🤝 Contributing

Pull Requests are welcome! Fork → Create a branch → Commit changes → Open a PR.

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](/LICENSE) file for details.

## 🙏 Acknowledgments

- Special thanks to [go-asmr-spider](https://github.com/DiheChen/go-asmr-spider)
- Thanks to all contributors and users!

---

**ASMRoner** — A different girl to keep you company every night :)

_Last updated: October 2026_

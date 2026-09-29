# 游戏档案

统计每一个游戏的游戏时长、进行游玩数据分析，并支持本地游戏存档导入与备份的桌面应用。

当前进度：**阶段 1-6 已完成**。阶段 6 接入 Steam 游戏库与游玩时长同步。

## 技术栈

| 层次 | 选型 |
| --- | --- |
| 桌面框架 | Wails v2 |
| 后端 | Go 1.25+ |
| 前端 | React 18 + TypeScript + Vite + Tailwind CSS v4 |
| 状态管理 | Zustand |
| 数据库 | SQLite（纯 Go 驱动 `glebarez/sqlite` + GORM，无需 CGO） |
| 进程监控 | `gopsutil/v3/process`，每 5 秒轮询 |
| 图表 | ECharts 6（按需注册折线 / 柱状 / 饼图，Canvas 渲染） |
| AI 报告 | OpenAI 兼容 Chat Completions API（默认 DeepSeek） |
| Markdown / 图片 | marked + DOMPurify、html2canvas-pro |
| 界面语言 / 主题 | 中文 / 暗色 |

## 已实现功能

### 阶段 1：Wails 初始化 + SQLite + 游戏 / 分类 / 标签 CRUD

阶段 1 相关功能详见下方数据库与项目结构说明。

### 阶段 2：目录扫描 + 进程监控 + 时长统计

- 游戏库可选目录并递归扫描 `.exe` 文件，默认深度 3 层，可设置扫描深度；过滤卸载程序、安装程序、崩溃报告、启动器等常见非游戏程序。
- 扫描结果先预览，支持勾选、修改游戏名与分类后批量导入；重复的可执行文件路径会跳过。
- 监控默认启动，每 5 秒读取进程；优先匹配可执行文件路径，再回退到进程名。
- 监控开启期间累加 `games.total_seconds`，并写入 `play_sessions`；设置页可暂停 / 继续，暂停和应用退出时会结束进行中的会话。
- 游玩时长按轮询增量持久化；异常退出后会收尾未结束记录，不把应用关闭期间的时间计入统计。
- 前端通过 Wails `tracker:update` 事件刷新活动游戏与累计时长。

### 阶段 3：统计图表 + 按年 / 月 / 周分析

- 新增「统计报告」页：周 / 月 / 年三种周期切换，支持上一周期 / 下一周期 / 回到当前。
- 概览卡片：周期内时长、游玩天数、日均时长、游玩游戏数、游玩次数。
- 图表（ECharts）：
  - 时长趋势：按年为按月分桶，月 / 周为按天分桶，无数据的日期也会补齐零点，保证时间轴连续。
  - 分类时长占比：饼图，使用分类自身颜色，未分类统一为灰色。
  - 标签时长排行：横向条形图，同一游戏的多个标签都会获得该次游玩时长。
  - Top 游戏：周期内时长前 10 名。
- 仪表盘新增今日 / 本周 / 本月时长卡片，并把分类分布引导到统计报告页。
- 统计口径：数据来自 `play_sessions`；跨天会话归属开始日期，与 `games.total_seconds` 一致；应用运行中会自动刷新图表。
- 后端接口支持自定义范围（同时传 `start` / `end`），跨度不超过 62 天按天分桶，否则按月分桶。

### 阶段 4：AI 报告 + 图片导出 + 报告导入导出

- 支持 OpenAI 兼容 Chat Completions API；默认地址 `https://api.deepseek.com`、模型 `deepseek-chat`，也可在设置页更换服务地址与模型。
- API Key 由用户在设置页配置并保存在本机 SQLite `settings` 表；测试连接并生成报告时才会向所配置的服务发送请求及所选周期统计数据。
- 按年 / 月 / 周统计结果生成 Markdown 报告，并保存到本地报告库；支持查看、删除与导出 Markdown 文件。
- 支持导入 `.md`、`.markdown`、`.txt` 和包含标题、周期、正文的 `.json` 报告。
- 统计分析页可导出当前周期图表与汇总卡片为 PNG；报告详情可单独导出 Markdown 或 PNG。
- Markdown 正文经 DOMPurify 净化后显示；图片写入前会检查 PNG 格式、文件大小和尺寸。

### 阶段 5：本地游戏存档 + 备份 + 备份历史管理

- 在游戏详情中为游戏登记现有存档文件或文件夹；登记只保存原始路径，不移动原文件。
- 选择备份目标文件夹与备注后，将存档复制为带游戏名、存档名和时间戳的唯一备份；文件夹层级、文件内容和修改时间会保留，符号链接及特殊文件会被明确拒绝。
- 每个存档显示原始路径、备注、最近备份时间、备份路径、每次备份备注及完整历史。
- 删除存档或备份历史默认只删数据库记录并保留磁盘文件；需在二次确认中勾选后，才会删除对应原始存档或备份文件。
- 删除存档记录会同时移除其备份历史记录，但生成的备份文件仍保留；删除游戏也只移除档案记录，不删除磁盘文件。

### 阶段 6：Steam 平台数据接入

- 设置页可填写 Steam Web API Key 与 17 位 SteamID64，并手动触发游戏库同步。
- 调用 `IPlayerService/GetOwnedGames` 获取游戏名称、AppID 和累计游玩分钟数；Steam 资料库需允许查看游戏详情。
- 先按 AppID 匹配，再按不区分大小写的游戏名匹配本地游戏；未匹配的游戏会作为 Steam 游戏记录导入。
- Steam 累计时长单独存于 `steam_playtime_seconds`，不覆盖进程监控的 `total_seconds`。游戏库区分展示两种时长；仪表盘累计、Top 游戏与分类占比按两种来源的较大值计算，避免重复统计。
- 设置页展示最近一次同步结果或错误；凭据、最近结果与同步错误保存在本机 `settings` 表。

### 游戏封面与展示模式

- 游戏库支持列表 / 卡片模式，模式、卡片列数和封面尺寸写入 `settings` 表。
- 游戏表分别保存 `cover_path` / `icon_path`、来源标记和更新时间；旧数据库通过 GORM 自动迁移。
- 列表优先展示图标，缺失时回退封面；卡片优先展示竖版封面，缺失时回退图标。图片由 Go 读取并转换为本地 data URL 提供给界面。
- 上传图片会复制到用户配置目录下的 `covers/` 或 `icons/` 缓存；支持清除、单个或批量自动获取。
- 自动来源顺序：安装目录的常见图片文件、Steam 商店 / CDN、（可选）SteamGridDB、（可选）RAWG。Steam 同步后会为缺图游戏尝试补充封面和图标。
- 自动获取不覆盖已有图片；用户主动触发单个或批量自动获取时可替换。下载失败使用占位图，不影响游戏库其它操作。
- 设置页可关闭在线搜索并保存 SteamGridDB / RAWG API Key；密钥保存在本地设置表。

## 目录结构

```
.
├─ main.go                     # Wails 入口：窗口配置、启动/退出钩子、绑定 App
├─ app.go                      # 绑定给前端的 App 方法（游戏管理、统计、Steam 等）
├─ wails.json                  # Wails 项目与应用元信息配置
├─ 项目总纲.md                  # 需求来源
├─ internal/
│  ├─ models/models.go         # GORM 数据模型与扫描 DTO
│  ├─ database/database.go     # SQLite 连接、迁移、初始化数据
│  └─ services/                # 业务逻辑层
│     ├─ service.go            # 服务集合
│     ├─ game.go               # 游戏 CRUD 与校验
│     ├─ category.go           # 分类 CRUD 与校验
│     ├─ tag.go                # 标签 CRUD 与校验
│     ├─ scanner.go            # 目录扫描与候选过滤
│     ├─ scanner_service.go    # 扫描结果批量导入
│     ├─ tracker.go            # 进程轮询、会话与时长累计
│     ├─ stats.go              # 年 / 月 / 周统计聚合与仪表盘汇总
│     ├─ report.go             # AI 报告生成、报告存档及导入导出
│     ├─ archive.go            # 游戏存档登记、文件/文件夹备份与历史管理
│     ├─ steam.go              # Steam 游戏库同步、AppID / 名称匹配与游玩时长关联
│     └─ *_test.go             # 扫描、监控、统计与报告的单元测试
├─ build/                      # Wails 构建资源（图标、安装包脚本）
└─ frontend/
   ├─ index.html
   ├─ vite.config.ts           # Vite + React + Tailwind 插件
   └─ src/
      ├─ App.tsx               # 外壳：侧边栏 + 页面切换
      ├─ main.tsx / style.css  # 入口与全局样式（Tailwind）
      ├─ lib/                  # 格式化、图表、Markdown 清理与图片导出辅助
      ├─ stores/appStore.ts    # Zustand 状态与所有 Wails 调用
      ├─ components/           # Sidebar、Modal、游戏表单 / 详情、AI / Steam / 封面设置等
      ├─ pages/                # Dashboard、GameLibrary、StatsPage、CategoriesPage、SettingsPage
      └─ wailsjs/              # Wails 自动生成的绑定（勿手动修改）
```

## 数据库

数据库文件：`os.UserConfigDir()/GameArchive/game_archive.db`

- Windows：`%AppData%\GameArchive\game_archive.db`
- macOS：`~/Library/Application Support/GameArchive/game_archive.db`
- Linux：`~/.config/GameArchive/game_archive.db`

全部业务表在初始化时一次建好，避免后续阶段再做破坏性迁移：

| 表 | 说明 | 当前用途 |
| --- | --- | --- |
| `games` | 游戏主表（名称、exe 路径、进程名、安装目录、封面、分类、本地累计秒数、Steam AppID / 秒数 / 同步时间、最后游玩时间） | 读写 |
| `categories` | 分类（名称唯一、颜色、排序值） | 读写 |
| `tags` | 标签（名称唯一、颜色） | 读写 |
| `game_tags` | 游戏 ↔ 标签 多对多关联表 | 读写 |
| `play_sessions` | 单次游玩记录 | 读写（阶段 2 写入，阶段 3 聚合） |
| `settings` | 键值配置（监控开关、扫描目录、AI 服务设置） | 读写 |
| `reports` | AI / 导入报告 | 读写 |
| `save_archives` | 游戏存档原路径、备份目标、最近备份时间与备注 | 读写 |
| `save_backups` | 每次备份路径、时间与备注 | 读写 |

`total_seconds` 与 `last_played_at` 由阶段 2 的进程监控维护，前端新增游戏时会被强制初始化为 0 / NULL。Steam 的 AppID、游玩秒数和最近同步时间分别保存在 `steam_app_id`、`steam_playtime_seconds` 和 `steam_last_synced_at`；两类时长独立维护。
封面和图标分别使用 `cover_path` / `icon_path`、`cover_source` / `icon_source` 和各自更新时间；游戏库展示模式及图片服务配置存于 `settings`。

## 前端可调用的后端方法

由 `app.go` 导出，生成到 `frontend/wailsjs/go/main/App`：

```
GetAppInfo()                      // 应用信息与计数（不返回错误，便于展示启动异常）

ListGames() / GetGameDetail(id) / AddGame(game) / UpdateGame(game) / DeleteGame(id)
ListCategories() / AddCategory(c) / UpdateCategory(c) / DeleteCategory(id)
ListTags() / AddTag(t) / UpdateTag(t) / DeleteTag(id)

SelectGameDirectory()              // 系统目录选择框
AddGameDirectory(dir) / GetGameDirectory()
ScanGamesInDirectory(dir, depth)   // 返回待预览的 ScannedGame 列表
ImportScannedGames(games)          // 批量导入并返回 imported / skipped 数量
StartMonitor() / StopMonitor()
GetMonitorStatus()

GetStatsByPeriod(periodType, start, end)   // year / month / week，start / end 可空
GetDashboardStats()                        // 总时长、今日 / 本周 / 本月、Top 游戏、分类占比

GetAISettings() / SaveAISettings(config) / TestAIConnection()
GenerateAIReport(request) / ListReports() / GetReport(id) / SaveReport(report) / DeleteReport(id)
ImportReportFromFile() / ExportReportMarkdown(id) / SaveImage(base64PNG, filename)

GetSteamSettings() / SaveSteamSettings(config)
GetSteamSyncStatus() / SyncSteamLibrary()

SelectAndSetCover(gameID, "cover" | "icon") / ClearCover(gameID, target)
AutoFetchCover(gameID) / AutoFetchIcon(gameID)
BatchFetchCovers(gameIDs) / BatchFetchIcons(gameIDs)
GetGameCoverInfo(gameID) / GetGameImageData(gameID, target)
GetCoverSettings() / SaveCoverSettings(config)
GetLibraryDisplaySettings() / SaveLibraryDisplaySettings(settings)

SelectSaveArchiveFile() / SelectSaveArchiveDirectory() / SelectSaveBackupDirectory(defaultDir)
AddSaveArchive(gameID, name, sourcePath, isDir, backupDir, note)
ListSaveArchives(gameID) / BackupSaveArchive(archiveID, targetDir, note) / ListSaveBackups(archiveID)
DeleteSaveArchive(id, deleteOriginal) / DeleteSaveBackup(id, deleteFile)
```

> `AddGame` / `UpdateGame` 额外返回保存后的完整记录，方便前端直接使用；标签只需传入 `{ id }`。
> `tracker:update` 事件会在成功轮询后推送监控状态。扫描仅识别 `.exe`；`.lnk` / `.url` 与 Steam manifest 解析不在本阶段实现。
> `GetStatsByPeriod` 的 `start` / `end` 为空时统计当前周期，同时传入按自定义范围统计；跨天会话归属开始日期。
> AI 服务 API Key 需在桌面应用设置页输入；不要将密钥提交到版本库。生成报告会把对应周期的统计数据发送到已配置的 AI 服务。
> Steam API Key 与 SteamID64 需在设置页输入；手动同步需要 Steam 游戏详情对 Web API 可见。Steam 时长与本地监控时长独立保存。
> 存档登记只保存路径；删除操作默认保留文件，只有确认对话框中勾选删除文件后才会执行物理删除。

## 运行与构建

```bash
# 依赖安装
go mod download
cd frontend && npm install && cd ..

# 开发模式（热重载，自动重新生成 wailsjs 绑定）
wails dev

# 生产构建，产物在 build/bin/
wails build

# 仅重新生成前端绑定
wails generate module

# 运行后端单元测试
go test ./...

# 前端生产构建
cd frontend && npm run build
```

## 验收清单

1. `go test ./...`、`go vet ./...`、`cd frontend && npm run build` 与 `wails build` 均通过。
2. 首次启动后创建 `%AppData%\GameArchive\game_archive.db`，并保留阶段 1-4 的游戏、监控、统计与报告功能。
3. 设置 DeepSeek / OpenAI 兼容服务地址、模型与 API Key，测试连接成功。
4. 选择有时长数据的周 / 月 / 年周期，生成报告并确认报告可在历史列表查看、删除及导出 Markdown / PNG。
5. 导入 Markdown、文本或 JSON 报告，并确认其出现在报告历史中。
6. 在游戏详情分别登记单个存档文件和文件夹，确认原始内容未被移动或修改。
7. 选择目标文件夹执行备份，检查副本内容、命名和备份时间/路径/备注记录；再次备份时应生成另一历史记录。
8. 删除存档或备份记录时不勾选删除文件，确认原始/备份文件仍存在；使用临时测试文件勾选确认后，验证对应文件才被删除。
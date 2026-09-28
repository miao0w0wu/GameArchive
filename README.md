# 游戏档案

统计每一个游戏的游戏时长、进行游玩数据分析，并支持本地游戏存档导入与备份的桌面应用。

当前进度：**阶段 1、阶段 2、阶段 3 已完成**。阶段 3 增加 ECharts 统计图表与按年 / 月 / 周分析；阶段 4-5 尚未实现。

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

未实现（后续阶段）：

- 阶段 4：Ollama / OpenAI 兼容接口生成 AI 报告、图片导出、报告导入导出。
- 阶段 5：本地存档导入、备份与备份历史管理。

## 目录结构

```
.
├─ main.go                     # Wails 入口：窗口配置、启动/退出钩子、绑定 App
├─ app.go                      # 绑定给前端的 App 方法（游戏/分类/标签 CRUD、应用信息）
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
│     └─ *_test.go             # 扫描、监控、统计的单元测试
├─ build/                      # Wails 构建资源（图标、安装包脚本）
└─ frontend/
   ├─ index.html
   ├─ vite.config.ts           # Vite + React + Tailwind 插件
   └─ src/
      ├─ App.tsx               # 外壳：侧边栏 + 页面切换
      ├─ main.tsx / style.css  # 入口与全局样式（Tailwind）
      ├─ lib/                  # format.ts 格式化、ui.ts 复用类名、charts.ts ECharts 注册
      ├─ stores/appStore.ts    # Zustand 状态与所有 Wails 调用
      ├─ components/           # Sidebar、Modal、EChart、ScanGamesModal、游戏表单 / 详情等
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
| `games` | 游戏主表（名称、exe 路径、进程名、安装目录、封面、分类、累计秒数、最后游玩时间） | 读写 |
| `categories` | 分类（名称唯一、颜色、排序值） | 读写 |
| `tags` | 标签（名称唯一、颜色） | 读写 |
| `game_tags` | 游戏 ↔ 标签 多对多关联表 | 读写 |
| `play_sessions` | 单次游玩记录 | 读写（阶段 2 写入，阶段 3 聚合） |
| `settings` | 键值配置（监控开关、扫描目录） | 读写 |
| `reports` | AI / 导入报告 | 仅建表（阶段 4） |
| `save_archives` | 本地存档记录 | 仅建表（阶段 5） |
| `save_backups` | 存档备份历史 | 仅建表（阶段 5） |

`total_seconds` 与 `last_played_at` 由阶段 2 的进程监控维护，前端新增游戏时会被强制初始化为 0 / NULL。

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
```

> `AddGame` / `UpdateGame` 额外返回保存后的完整记录，方便前端直接使用；标签只需传入 `{ id }`。
> `tracker:update` 事件会在成功轮询后推送监控状态。扫描仅识别 `.exe`；`.lnk` / `.url` 与 Steam manifest 解析不在本阶段实现。
> `GetStatsByPeriod` 的 `start` / `end` 为空时统计当前周期，同时传入按自定义范围统计；跨天会话归属开始日期。

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
```

## 验收清单

1. `go build ./...` 与 `go vet ./...` 无错误。
2. `cd frontend && npm run build`（tsc + vite）无类型错误。
3. `wails build` 产出 `build/bin/GameArchive.exe`。
4. 启动应用后生成 `%AppData%\GameArchive\game_archive.db`，且包含上表全部 9 张业务表 + `sqlite_sequence`。
5. 首次运行自动写入 9 个默认分类，标签初始为空。
6. 阶段 1：游戏 / 分类 / 标签可新增、编辑、删除；列表搜索与筛选可用；分类删除后游戏变为未分类。
7. 阶段 2：游戏库选择目录并扫描后可预览候选 `.exe`，可勾选、改名、选分类并导入；重复路径会跳过。
8. 阶段 2：监控启动后运行库中已登记游戏，等待轮询后仪表盘/游戏库时长增加，`play_sessions` 保存开始、结束和秒数。
9. 阶段 2：设置页暂停监控会结束当前会话且不再累计；继续后重新监控；正常退出时会收尾会话。
10. 阶段 3：统计报告页可在周 / 月 / 年之间切换，上一周期 / 下一周期 / 回到当前按钮生效；趋势图按天或按月分桶，分类饼图、标签排行与 Top 游戏随周期变化。
11. 阶段 3：仪表盘今日 / 本周 / 本月时长与统计报告页数据一致；游玩过程中图表会自动刷新。
12. `go test ./...`、`go vet ./...` 与 `cd frontend && npm run build` 均通过；界面为中文暗色主题，错误通过提示或界面状态呈现。
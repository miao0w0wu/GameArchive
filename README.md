# 游戏档案

统计每一个游戏的游戏时长、进行游玩数据分析，并支持本地游戏存档导入与备份的桌面应用。

当前进度：**阶段 1（Wails 项目初始化 + SQLite + 游戏 / 分类 / 标签 CRUD）已完成**，阶段 2-5 尚未实现。

## 技术栈

| 层次 | 选型 |
| --- | --- |
| 桌面框架 | Wails v2 |
| 后端 | Go 1.25+ |
| 前端 | React 18 + TypeScript + Vite + Tailwind CSS v4 |
| 状态管理 | Zustand |
| 数据库 | SQLite（纯 Go 驱动 `glebarez/sqlite` + GORM，无需 CGO） |
| 界面语言 / 主题 | 中文 / 暗色 |

## 阶段 1 范围

已实现：

- Wails 项目骨架、中文窗口标题、暗色主题外壳（侧边栏导航 + 仪表盘 / 游戏库 / 分类与标签 / 设置）。
- SQLite 初始化：数据库放在 `os.UserConfigDir()/GameArchive/game_archive.db`，自动建表、开启 WAL 与外键约束，首次运行写入 9 个常用分类。
- 游戏 CRUD：新增、编辑、删除、列表、详情；可设置分类与多个标签，支持搜索、按分类 / 标签筛选、列表与卡片两种视图。
- 分类 CRUD：名称唯一校验、颜色、排序值；删除分类后相关游戏自动变为「未分类」。
- 标签 CRUD：名称唯一校验、颜色；删除标签时自动清理与游戏的关联。
- 统一的错误提示与中文校验信息（如「游戏名称不能为空」「已存在同名分类」）。

未实现（后续阶段）：

- 阶段 2：手动添加游戏目录、目录扫描、进程监控与时长统计。
- 阶段 3：ECharts 统计图表、按年 / 月 / 周分析。
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
│  ├─ models/models.go         # GORM 数据模型
│  ├─ database/database.go     # SQLite 连接、迁移、初始化数据
│  └─ services/                # 业务逻辑层
│     ├─ service.go            # 服务集合
│     ├─ game.go               # 游戏 CRUD 与校验
│     ├─ category.go           # 分类 CRUD 与校验
│     └─ tag.go                # 标签 CRUD 与校验
├─ build/                      # Wails 构建资源（图标、安装包脚本）
└─ frontend/
   ├─ index.html
   ├─ vite.config.ts           # Vite + React + Tailwind 插件
   └─ src/
      ├─ App.tsx               # 外壳：侧边栏 + 页面切换
      ├─ main.tsx / style.css  # 入口与全局样式（Tailwind）
      ├─ lib/                  # format.ts 格式化、ui.ts 复用类名
      ├─ stores/appStore.ts    # Zustand 状态与所有 Wails 调用
      ├─ components/           # Sidebar、Modal、ConfirmDialog、Toaster、StatCard、表单与详情弹窗、Icon
      ├─ pages/                # Dashboard、GameLibrary、CategoriesPage、SettingsPage
      └─ wailsjs/              # Wails 自动生成的绑定（勿手动修改）
```

## 数据库

数据库文件：`os.UserConfigDir()/GameArchive/game_archive.db`

- Windows：`%AppData%\GameArchive\game_archive.db`
- macOS：`~/Library/Application Support/GameArchive/game_archive.db`
- Linux：`~/.config/GameArchive/game_archive.db`

阶段 1 一次性建好全部业务表，避免后续阶段再做破坏性迁移：

| 表 | 说明 | 阶段 1 用途 |
| --- | --- | --- |
| `games` | 游戏主表（名称、exe 路径、进程名、安装目录、封面、分类、累计秒数、最后游玩时间） | 读写 |
| `categories` | 分类（名称唯一、颜色、排序值） | 读写 |
| `tags` | 标签（名称唯一、颜色） | 读写 |
| `game_tags` | 游戏 ↔ 标签 多对多关联表 | 读写 |
| `play_sessions` | 单次游玩记录 | 仅建表 |
| `settings` | 键值配置 | 仅建表 |
| `reports` | AI / 导入报告 | 仅建表 |
| `save_archives` | 本地存档记录 | 仅建表 |
| `save_backups` | 存档备份历史 | 仅建表 |

`total_seconds` 与 `last_played_at` 由阶段 2 的进程监控维护，前端新增游戏时会被强制初始化为 0 / NULL。

## 前端可调用的后端方法

由 `app.go` 导出，生成到 `frontend/wailsjs/go/main/App`：

```
GetAppInfo()                      // 应用信息与计数（不返回错误，便于展示启动异常）

ListGames() / GetGameDetail(id) / AddGame(game) / UpdateGame(game) / DeleteGame(id)
ListCategories() / AddCategory(c) / UpdateCategory(c) / DeleteCategory(id)
ListTags() / AddTag(t) / UpdateTag(t) / DeleteTag(id)
```

> `AddGame` / `UpdateGame` 额外返回保存后的完整记录，方便前端直接使用；标签只需传入 `{ id }`。

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
```

## 验收清单（阶段 1）

1. `go build ./...` 与 `go vet ./...` 无错误。
2. `cd frontend && npm run build`（tsc + vite）无类型错误。
3. `wails build` 产出 `build/bin/GameArchive.exe`。
4. 启动应用后生成 `%AppData%\GameArchive\game_archive.db`，且包含上表全部 9 张业务表 + `sqlite_sequence`。
5. 首次运行自动写入 9 个默认分类，标签初始为空。
6. 游戏：可新增（名称必填、可空分类、可多标签）、编辑（可清空分类）、删除；列表支持搜索、按分类 / 标签筛选、列表与卡片视图切换；详情弹窗展示 Go 侧计算的累计小时数。
7. 分类 / 标签：可增删改；重名会被拒绝；删除分类后相关游戏变为「未分类」，删除标签后关联被清理。
8. 界面为中文暗色主题，所有错误以浮层提示展示。
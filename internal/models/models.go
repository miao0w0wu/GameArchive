// Package models 定义「游戏档案」的 SQLite 数据模型（GORM）与 Wails DTO。
package models

import "time"

// Game 游戏主表：保存游戏基础信息与累计游玩时长。
type Game struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:255;not null;index" json:"name"`
	// ExePath 可执行文件绝对路径，进程监控优先用它匹配（阶段 2）。
	ExePath string `gorm:"size:1024" json:"exePath"`
	// ProcessName 进程名（含或不含 .exe），作为 exe 路径匹配失败时的兜底。
	ProcessName string `gorm:"size:255;index" json:"processName"`
	InstallDir  string `gorm:"size:1024" json:"installDir"`
	// CoverPath 封面图片的本地路径，阶段 1 仅存储与编辑，不做渲染。
	CoverPath  string    `gorm:"size:1024" json:"coverPath"`
	CategoryID *uint     `gorm:"index" json:"categoryId"`
	Category   *Category `gorm:"foreignKey:CategoryID;constraint:OnDelete:SET NULL" json:"category"`
	Tags       []Tag     `gorm:"many2many:game_tags;" json:"tags"`
	// TotalSeconds 累计游玩秒数，由阶段 2 的进程监控累加。
	TotalSeconds int64      `gorm:"not null;default:0" json:"totalSeconds"`
	LastPlayedAt *time.Time `json:"lastPlayedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// ScannedGame 是目录扫描的预览结果，不会直接映射到数据库。
type ScannedGame struct {
	Name              string `json:"name"`
	ExePath           string `json:"exePath"`
	ProcessName       string `json:"processName"`
	InstallDir        string `json:"installDir"`
	SizeBytes         int64  `json:"sizeBytes"`
	SuggestedCategory string `json:"suggestedCategory"`
	CategoryID        *uint  `json:"categoryId"`
}

// Category 游戏分类。
type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Color     string    `gorm:"size:32" json:"color"`
	SortOrder int       `gorm:"not null;default:0;index" json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Tag 游戏标签，与 Game 为多对多关系（关联表 game_tags）。
type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Color     string    `gorm:"size:32" json:"color"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PlaySession 一次游玩记录，由阶段 2 的进程监控写入。
type PlaySession struct {
	ID      uint       `gorm:"primaryKey" json:"id"`
	GameID  uint       `gorm:"not null;index" json:"gameId"`
	StartAt time.Time  `gorm:"index" json:"startAt"`
	EndAt   *time.Time `json:"endAt"`
	// DurationSeconds 本次游玩时长（秒）。
	DurationSeconds int64 `gorm:"not null;default:0" json:"durationSeconds"`
	// Date 游玩日期，格式 2006-01-02，便于按天聚合。
	Date string `gorm:"size:10;index" json:"date"`
	// Source 数据来源：monitor（进程监控）/ manual（手动补录）。
	Source    string    `gorm:"size:32;not null;default:monitor" json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

// Setting 键值对配置表。
type Setting struct {
	Key       string    `gorm:"primaryKey;size:191" json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Report AI 报告或导入的外部报告。
type Report struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Title       string     `gorm:"size:255;not null" json:"title"`
	PeriodType  string     `gorm:"size:16;index" json:"periodType"`
	PeriodStart *time.Time `json:"periodStart"`
	PeriodEnd   *time.Time `json:"periodEnd"`
	// Content 报告正文（Markdown）。
	Content   string    `gorm:"type:text" json:"content"`
	Source    string    `gorm:"size:32;not null;default:ai" json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

// SaveArchive 本地游戏存档记录。
type SaveArchive struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	GameID uint   `gorm:"not null;index" json:"gameId"`
	Name   string `gorm:"size:255;not null" json:"name"`
	// SourcePath 存档的原始路径（文件或文件夹）。
	SourcePath string `gorm:"size:1024" json:"sourcePath"`
	// BackupDir 该存档默认的备份目标目录。
	BackupDir    string     `gorm:"size:1024" json:"backupDir"`
	LastBackupAt *time.Time `json:"lastBackupAt"`
	Note         string     `gorm:"size:1024" json:"note"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// SaveBackup 一次存档备份的历史记录。
type SaveBackup struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	ArchiveID uint `gorm:"not null;index" json:"archiveId"`
	// BackupPath 备份产物路径（文件夹或文件）。
	BackupPath string    `gorm:"size:1024" json:"backupPath"`
	BackedUpAt time.Time `gorm:"index" json:"backedUpAt"`
	Note       string    `gorm:"size:1024" json:"note"`
	CreatedAt  time.Time `json:"createdAt"`
}

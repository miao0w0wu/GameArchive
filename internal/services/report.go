package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

// AI 配置与报告来源相关的常量。
const (
	settingAIBaseURL = "ai_base_url"
	settingAIAPIKey  = "ai_api_key"
	settingAIModel   = "ai_model"

	// DefaultAIBaseURL DeepSeek 提供 OpenAI 兼容接口。
	DefaultAIBaseURL = "https://api.deepseek.com"
	// DefaultAIModel 默认模型。
	DefaultAIModel = "deepseek-chat"

	// 报告来源：ai=模型生成，import=外部导入，manual=手动新建。
	reportSourceAI     = "ai"
	reportSourceImport = "import"

	// aiRequestTimeout 单次 AI 请求的超时时间。
	aiRequestTimeout = 120 * time.Second
	// aiTestTimeout 测试连接的短超时。
	aiTestTimeout = 20 * time.Second
)

// AIConfig 是 OpenAI 兼容接口的配置，保存在 settings 表中。
//
// API Key 存储在本机数据库中；发起请求时仅发送到用户配置的服务。
type AIConfig struct {
	// BaseURL 接口基础地址，可为服务根地址或包含版本路径的地址。
	BaseURL string `json:"baseUrl"`
	// APIKey 接口密钥。
	APIKey string `json:"apiKey"`
	// Model 模型名称。
	Model string `json:"model"`
}

// AIReportRequest 是生成 AI 报告的请求参数。
type AIReportRequest struct {
	PeriodType string `json:"periodType"`
	Start      string `json:"start"`
	End        string `json:"end"`
	// Title 自定义报告标题，留空时按周期自动生成。
	Title string `json:"title"`
	// Instruction 额外要求，会附加到提示词中。
	Instruction string `json:"instruction"`
}

// ReportService 负责 AI 报告生成、报告存档以及导入导出。
type ReportService struct {
	db     *gorm.DB
	stats  *StatsService
	client *http.Client
}

// NewReportService 创建报告服务。
func NewReportService(db *gorm.DB) *ReportService {
	return &ReportService{
		db:     db,
		stats:  &StatsService{db: db},
		client: &http.Client{Timeout: aiRequestTimeout},
	}
}

// ------------------------------------------------------------- AI 配置

// GetAIConfig 读取 AI 配置，未配置时返回带默认值的空配置。
func (s *ReportService) GetAIConfig() (AIConfig, error) {
	values, err := s.loadSettings(settingAIBaseURL, settingAIAPIKey, settingAIModel)
	if err != nil {
		return AIConfig{}, err
	}
	config := AIConfig{
		BaseURL: values[settingAIBaseURL],
		APIKey:  values[settingAIAPIKey],
		Model:   values[settingAIModel],
	}
	if config.BaseURL == "" {
		config.BaseURL = DefaultAIBaseURL
	}
	if config.Model == "" {
		config.Model = DefaultAIModel
	}
	return config, nil
}

// SaveAIConfig 保存 AI 配置，并对地址与模型做基本校验。
func (s *ReportService) SaveAIConfig(config AIConfig) error {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.Model = strings.TrimSpace(config.Model)

	if config.BaseURL == "" {
		return errors.New("接口地址不能为空")
	}
	parsedURL, err := url.ParseRequestURI(config.BaseURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" ||
		parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return errors.New("接口地址需要以 http:// 或 https:// 开头")
	}
	if config.Model == "" {
		return errors.New("模型名称不能为空")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		pairs := map[string]string{
			settingAIBaseURL: config.BaseURL,
			settingAIAPIKey:  config.APIKey,
			settingAIModel:   config.Model,
		}
		for key, value := range pairs {
			setting := models.Setting{Key: key, Value: value}
			if err := tx.Save(&setting).Error; err != nil {
				return fmt.Errorf("保存 AI 配置失败: %w", err)
			}
		}
		return nil
	})
}

// TestConnection 验证配置是否可用：优先请求 /models，不支持时退回一次最小对话请求。
func (s *ReportService) TestConnection() error {
	config, err := s.GetAIConfig()
	if err != nil {
		return err
	}
	if config.APIKey == "" {
		return errors.New("请先填写 API Key")
	}

	request, err := http.NewRequest(http.MethodGet, config.BaseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("构造测试请求失败: %w", err)
	}
	s.authorize(request, config.APIKey)

	client := &http.Client{Timeout: aiTestTimeout}
	response, err := client.Do(request)
	if err == nil {
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return nil
		}
		// /models 不被支持时（部分兼容服务返回 404），改用最小对话请求验证。
		if response.StatusCode != http.StatusNotFound && response.StatusCode != http.StatusMethodNotAllowed {
			return formatAPIError(response.StatusCode, body)
		}
	}

	_, err = s.chat(config, "ping", "回复 ok 即可")
	if err != nil {
		return err
	}
	return nil
}

// ------------------------------------------------------------- 报告生成

// GenerateAIReport 依据统计结果调用 OpenAI 兼容接口生成 Markdown 报告并保存。
func (s *ReportService) GenerateAIReport(req AIReportRequest) (*models.Report, error) {
	config, err := s.GetAIConfig()
	if err != nil {
		return nil, err
	}
	if config.APIKey == "" {
		return nil, errors.New("尚未配置 API Key，请先到「设置」页填写 AI 配置")
	}

	stats, err := s.stats.GetStatsByPeriod(req.PeriodType, req.Start, req.End)
	if err != nil {
		return nil, err
	}
	if stats.TotalSeconds == 0 {
		return nil, errors.New("所选周期没有游玩记录，暂时无法生成报告")
	}

	payload, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("整理统计数据失败: %w", err)
	}

	content, err := s.chat(config, aiSystemPrompt, buildUserPrompt(stats, string(payload), req.Instruction))
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = defaultReportTitle(stats)
	}
	startAt, endAt := parseRangeBound(stats.Start, stats.End)

	report := models.Report{
		Title:       title,
		PeriodType:  stats.PeriodType,
		PeriodStart: startAt,
		PeriodEnd:   endAt,
		Content:     strings.TrimSpace(content),
		Source:      reportSourceAI,
	}
	return s.Save(report)
}

// ------------------------------------------------------------- 报告 CRUD

// List 返回报告列表（不含正文，便于列表展示），按创建时间倒序。
func (s *ReportService) List() ([]models.Report, error) {
	reports := make([]models.Report, 0)
	err := s.db.
		Select("id, title, period_type, period_start, period_end, source, created_at").
		Order("created_at DESC, id DESC").
		Find(&reports).Error
	if err != nil {
		return nil, fmt.Errorf("查询报告列表失败: %w", err)
	}
	return reports, nil
}

// Get 按 ID 查询报告（含正文）。
func (s *ReportService) Get(id uint) (*models.Report, error) {
	var report models.Report
	err := s.db.First(&report, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询报告失败: %w", err)
	}
	return &report, nil
}

// Save 保存报告（新增或更新），并校验必填项。
func (s *ReportService) Save(report models.Report) (*models.Report, error) {
	report.Title = strings.TrimSpace(report.Title)
	report.Content = strings.TrimSpace(report.Content)
	if report.Title == "" {
		return nil, errors.New("报告标题不能为空")
	}
	if report.Content == "" {
		return nil, errors.New("报告内容不能为空")
	}
	if report.Source == "" {
		report.Source = reportSourceImport
	}

	if report.ID == 0 {
		if err := s.db.Create(&report).Error; err != nil {
			return nil, fmt.Errorf("保存报告失败: %w", err)
		}
	} else {
		fields := map[string]any{
			"title":        report.Title,
			"period_type":  report.PeriodType,
			"period_start": report.PeriodStart,
			"period_end":   report.PeriodEnd,
			"content":      report.Content,
			"source":       report.Source,
		}
		result := s.db.Model(&models.Report{}).Where("id = ?", report.ID).Updates(fields)
		if result.Error != nil {
			return nil, fmt.Errorf("保存报告失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil, ErrNotFound
		}
	}
	return s.Get(report.ID)
}

// Delete 删除报告。
func (s *ReportService) Delete(id uint) error {
	result := s.db.Delete(&models.Report{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除报告失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ------------------------------------------------------------- 导入与导出

// ImportFromFile 从 .json / .md / .txt 文件导入外部报告。
func (s *ReportService) ImportFromFile(path string) (*models.Report, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("未选择文件")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("检查报告文件失败: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("报告路径不是普通文件")
	}
	if info.Size() > 10<<20 {
		return nil, errors.New("报告文件超过 10 MiB 限制")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取报告文件失败: %w", err)
	}

	text := strings.TrimPrefix(string(data), "\uFEFF") // 去掉可能存在的 BOM
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return s.importFromJSON(text, path)
	}
	return s.importFromMarkdown(text, path)
}

// importFromJSON 解析 JSON 报告；支持完整报告对象，也支持只带 title / content 的精简对象。
func (s *ReportService) importFromJSON(text, path string) (*models.Report, error) {
	var payload struct {
		Title       string `json:"title"`
		PeriodType  string `json:"periodType"`
		PeriodStart string `json:"periodStart"`
		PeriodEnd   string `json:"periodEnd"`
		Content     string `json:"content"`
		Markdown    string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, fmt.Errorf("报告 JSON 解析失败: %w", err)
	}

	content := strings.TrimSpace(payload.Content)
	if content == "" {
		content = strings.TrimSpace(payload.Markdown)
	}
	if content == "" {
		return nil, errors.New("报告 JSON 中缺少 content 或 markdown 字段")
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = titleFromFilename(path)
	}
	startAt := parseOptionalTime(payload.PeriodStart)
	endAt := parseOptionalTime(payload.PeriodEnd)

	return s.Save(models.Report{
		Title:       title,
		PeriodType:  normalizePeriod(payload.PeriodType),
		PeriodStart: startAt,
		PeriodEnd:   endAt,
		Content:     content,
		Source:      reportSourceImport,
	})
}

// importFromMarkdown 解析 Markdown / 文本报告，首个一级标题作为标题。
func (s *ReportService) importFromMarkdown(text, path string) (*models.Report, error) {
	content := strings.TrimSpace(text)
	if content == "" {
		return nil, errors.New("报告文件内容为空")
	}

	title := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			break
		}
		if trimmed != "" {
			break
		}
	}
	if title == "" {
		title = titleFromFilename(path)
	}

	return s.Save(models.Report{
		Title:   title,
		Content: content,
		Source:  reportSourceImport,
	})
}

// ExportMarkdown 返回报告正文与建议的导出文件名，落盘由绑定层弹出保存对话框完成。
func (s *ReportService) ExportMarkdown(id uint) (string, string, error) {
	report, err := s.Get(id)
	if err != nil {
		return "", "", err
	}
	content := report.Content
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	filename := fmt.Sprintf(
		"%s_%s.md",
		sanitizeFilename(report.Title),
		time.Now().Format("20060102-150405"),
	)
	return content, filename, nil
}

// ------------------------------------------------------------- 内部实现

// loadSettings 批量读取设置表，未存在的键返回空字符串。
func (s *ReportService) loadSettings(keys ...string) (map[string]string, error) {
	settings := make([]models.Setting, 0, len(keys))
	if err := s.db.Where("key IN ?", keys).Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	values := make(map[string]string, len(settings))
	for _, setting := range settings {
		values[setting.Key] = setting.Value
	}
	return values, nil
}

// authorize 为请求附加 OpenAI 兼容的鉴权头。
func (s *ReportService) authorize(request *http.Request, apiKey string) {
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// chat 调用 OpenAI 兼容的 /chat/completions 接口，返回模型输出的文本。
func (s *ReportService) chat(config AIConfig, systemPrompt, userPrompt string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: config.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.6,
	})
	if err != nil {
		return "", fmt.Errorf("构造请求体失败: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("构造请求失败: %w", err)
	}
	s.authorize(request, config.APIKey)

	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("请求 AI 接口失败: %w", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("读取 AI 接口响应失败: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", formatAPIError(response.StatusCode, raw)
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("解析 AI 接口响应失败: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("AI 接口返回错误: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errors.New("AI 接口未返回内容，请检查模型名称是否正确")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// formatAPIError 把接口错误转换成可读的中文提示。
func formatAPIError(status int, body []byte) error {
	detail := strings.TrimSpace(string(body))
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		detail = parsed.Error.Message
	}
	if len(detail) > 300 {
		detail = detail[:300] + "…"
	}

	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("AI 接口鉴权失败（%d），请检查 API Key：%s", status, detail)
	case http.StatusNotFound:
		return fmt.Errorf("AI 接口地址不存在（404），请确认接口地址包含版本路径：%s", detail)
	case http.StatusTooManyRequests:
		return fmt.Errorf("AI 接口请求过于频繁（429），请稍后重试：%s", detail)
	default:
		return fmt.Errorf("AI 接口返回异常状态 %d：%s", status, detail)
	}
}

// aiSystemPrompt 生成报告时使用的系统提示词。
const aiSystemPrompt = "你是一名游戏游玩数据分析师。请依据用户提供的统计数据撰写中文分析报告，" +
	"使用 Markdown 格式，语言简洁客观，不要编造数据中不存在的信息。"

// buildUserPrompt 依据统计数据组装用户提示词。
func buildUserPrompt(stats StatsResult, payload, instruction string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "统计周期：%s 至 %s（%s）\n\n", stats.Start, stats.End, periodLabel(stats.PeriodType))
	builder.WriteString("统计数据（JSON）：\n```json\n")
	builder.WriteString(payload)
	builder.WriteString("\n```\n\n")
	builder.WriteString("请输出一份 Markdown 报告，包含以下部分：\n")
	builder.WriteString("1. 总体概览：总时长、游玩天数、日均时长、游玩次数；\n")
	builder.WriteString("2. Top 游戏：按时长排序并简要点评；\n")
	builder.WriteString("3. 分类占比：说明时间分配情况；\n")
	builder.WriteString("4. 趋势分析：结合趋势数据描述游玩节奏；\n")
	builder.WriteString("5. 游玩习惯总结；\n")
	builder.WriteString("6. 建议：给出 2-3 条可执行的建议。\n")
	builder.WriteString("报告以一级标题开头，控制在 600 字以内。")

	if extra := strings.TrimSpace(instruction); extra != "" {
		builder.WriteString("\n\n额外要求：")
		builder.WriteString(extra)
	}
	return builder.String()
}

// periodLabel 返回周期的中文名称。
func periodLabel(periodType string) string {
	switch periodType {
	case PeriodYear:
		return "按年"
	case PeriodMonth:
		return "按月"
	default:
		return "按周"
	}
}

// defaultReportTitle 依据统计范围生成默认报告标题。
func defaultReportTitle(stats StatsResult) string {
	start, err := time.ParseInLocation(dateLayout, stats.Start, time.Local)
	if err != nil {
		return fmt.Sprintf("%s ~ %s 游玩报告", stats.Start, stats.End)
	}
	switch stats.PeriodType {
	case PeriodYear:
		return fmt.Sprintf("%d 年游玩报告", start.Year())
	case PeriodMonth:
		return fmt.Sprintf("%d 年 %d 月游玩报告", start.Year(), int(start.Month()))
	default:
		return fmt.Sprintf("%s ~ %s 游玩报告", stats.Start, stats.End)
	}
}

// parseRangeBound 把统计范围的起止日期转换为时间指针。
func parseRangeBound(start, end string) (*time.Time, *time.Time) {
	return parseOptionalTime(start), parseOptionalTime(end)
}

// parseOptionalTime 解析日期字符串，失败时返回 nil。
func parseOptionalTime(value string) *time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if parsed, err := time.ParseInLocation(dateLayout, trimmed, time.Local); err == nil {
		return &parsed
	}
	// 兼容 RFC3339 形式的时间戳
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return &parsed
	}
	return nil
}

// normalizePeriod 把导入数据中的周期字段规范化为 year / month / week。
func normalizePeriod(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PeriodYear:
		return PeriodYear
	case PeriodMonth:
		return PeriodMonth
	case PeriodWeek:
		return PeriodWeek
	default:
		return ""
	}
}

// titleFromFilename 用文件名作为报告标题。
func titleFromFilename(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.TrimSpace(name)
	if name == "" {
		return "导入的报告"
	}
	return name
}

var filenameSanitizer = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]+`)

// sanitizeFilename 清理文件名中的非法字符，保证可写入磁盘。
func sanitizeFilename(name string) string {
	cleaned := filenameSanitizer.ReplaceAllString(strings.TrimSpace(name), "_")
	cleaned = strings.Trim(cleaned, " .")
	if cleaned == "" {
		cleaned = "report"
	}
	if len([]rune(cleaned)) > 60 {
		cleaned = string([]rune(cleaned)[:60])
	}
	return cleaned
}

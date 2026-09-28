package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"GameArchive/internal/models"
)

func TestAIConfigDefaultsAndValidation(t *testing.T) {
	db := newTestDB(t)
	service := NewReportService(db)

	config, err := service.GetAIConfig()
	if err != nil {
		t.Fatalf("GetAIConfig() error = %v", err)
	}
	if config.BaseURL != DefaultAIBaseURL || config.Model != DefaultAIModel {
		t.Fatalf("default config = %#v", config)
	}
	if config.APIKey != "" {
		t.Fatalf("expected empty api key, got %q", config.APIKey)
	}

	invalid := []AIConfig{
		{BaseURL: "", Model: "m", APIKey: "k"},
		{BaseURL: "api.openai.com/v1", Model: "m", APIKey: "k"},
		{BaseURL: "https://user:password@example.com/v1", Model: "m", APIKey: "k"},
		{BaseURL: "https://example.com/v1?token=unexpected", Model: "m", APIKey: "k"},
		{BaseURL: "https://api.openai.com/v1", Model: "  ", APIKey: "k"},
	}
	for _, cfg := range invalid {
		if err := service.SaveAIConfig(cfg); err == nil {
			t.Fatalf("expected validation error for %#v", cfg)
		}
	}

	saved := AIConfig{BaseURL: "https://example.com/v1/", APIKey: "sk-test", Model: "gpt-4o-mini"}
	if err := service.SaveAIConfig(saved); err != nil {
		t.Fatalf("SaveAIConfig() error = %v", err)
	}

	loaded, err := service.GetAIConfig()
	if err != nil {
		t.Fatalf("GetAIConfig() error = %v", err)
	}
	if loaded.BaseURL != "https://example.com/v1" {
		t.Fatalf("base url = %q, want trailing slash trimmed", loaded.BaseURL)
	}
	if loaded.APIKey != "sk-test" || loaded.Model != "gpt-4o-mini" {
		t.Fatalf("loaded config = %#v", loaded)
	}
}

func TestGenerateAIReportCallsOpenAICompatibleAPI(t *testing.T) {
	db := newTestDB(t)

	game := models.Game{Name: "Alpha"}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 21, 10, 0, 0, 0, time.Local)
	createSession(t, db, game.ID, start, 5400)

	type captured struct {
		Auth string
		Body chatRequest
	}
	got := make(chan captured, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var body chatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		got <- captured{Auth: r.Header.Get("Authorization"), Body: body}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "# 周期报告\n\n本周游玩 1.5 小时。"}},
			},
		})
	}))
	defer server.Close()

	service := NewReportService(db)
	if err := service.SaveAIConfig(AIConfig{BaseURL: server.URL + "/v1", APIKey: "sk-local", Model: "gpt-4o-mini"}); err != nil {
		t.Fatal(err)
	}

	report, err := service.GenerateAIReport(AIReportRequest{
		PeriodType:  PeriodWeek,
		Start:       "2026-09-21",
		End:         "2026-09-27",
		Instruction: "请突出本周新游戏",
	})
	if err != nil {
		t.Fatalf("GenerateAIReport() error = %v", err)
	}

	select {
	case request := <-got:
		if request.Auth != "Bearer sk-local" {
			t.Fatalf("authorization header = %q", request.Auth)
		}
		if request.Body.Model != "gpt-4o-mini" {
			t.Fatalf("model = %q", request.Body.Model)
		}
		if len(request.Body.Messages) != 2 {
			t.Fatalf("messages = %#v", request.Body.Messages)
		}
		userPrompt := request.Body.Messages[1].Content
		if !strings.Contains(userPrompt, "\"totalSeconds\": 5400") {
			t.Fatalf("prompt should embed stats json, got: %s", userPrompt)
		}
		if !strings.Contains(userPrompt, "请突出本周新游戏") {
			t.Fatalf("prompt should include the extra instruction, got: %s", userPrompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the AI endpoint was not called")
	}

	if report.Title != "2026-09-21 ~ 2026-09-27 游玩报告" {
		t.Fatalf("title = %q", report.Title)
	}
	if report.Source != reportSourceAI {
		t.Fatalf("source = %q, want %q", report.Source, reportSourceAI)
	}
	if !strings.Contains(report.Content, "本周游玩") {
		t.Fatalf("content = %q", report.Content)
	}
	if report.ID == 0 {
		t.Fatal("report was not persisted")
	}

	stored, err := service.Get(report.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.PeriodStart == nil || !stored.PeriodStart.Equal(time.Date(2026, 9, 21, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("period start = %v", stored.PeriodStart)
	}

	list, err := service.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].Content != "" {
		t.Fatalf("list should contain one report without content: %#v", list)
	}
}

func TestGenerateAIReportErrors(t *testing.T) {
	db := newTestDB(t)
	service := NewReportService(db)

	// 未配置 API Key
	if _, err := service.GenerateAIReport(AIReportRequest{PeriodType: PeriodWeek}); err == nil {
		t.Fatal("expected error when api key is missing")
	}

	if err := service.SaveAIConfig(AIConfig{BaseURL: "https://example.com/v1", APIKey: "sk", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	// 周期内没有游玩记录
	if _, err := service.GenerateAIReport(AIReportRequest{PeriodType: PeriodWeek}); err == nil {
		t.Fatal("expected error when the period has no sessions")
	}

	game := models.Game{Name: "Alpha"}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 21, 10, 0, 0, 0, time.Local)
	createSession(t, db, game.ID, start, 600)

	// 未知周期
	if _, err := service.GenerateAIReport(AIReportRequest{PeriodType: "day", Start: "2026-09-21", End: "2026-09-27"}); err == nil {
		t.Fatal("expected error for an invalid period type")
	}
	// 上游错误状态应转成可读提示
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer failing.Close()

	if err := service.SaveAIConfig(AIConfig{BaseURL: failing.URL, APIKey: "sk", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	_, err := service.GenerateAIReport(AIReportRequest{PeriodType: PeriodWeek, Start: "2026-09-21", End: "2026-09-27"})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "鉴权失败") || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("error = %v, want an auth message with the upstream detail", err)
	}
}

func TestTestConnectionFallsBackToChatWhenModelsIsMissing(t *testing.T) {
	db := newTestDB(t)
	service := NewReportService(db)

	chatCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			http.NotFound(w, r)
			return
		}
		chatCalls++
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	if err := service.SaveAIConfig(AIConfig{BaseURL: server.URL + "/v1", APIKey: "sk", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := service.TestConnection(); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if chatCalls == 0 {
		t.Fatal("expected a fallback chat completion call")
	}
}

func TestTestConnectionReportsAuthFailure(t *testing.T) {
	db := newTestDB(t)
	service := NewReportService(db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer server.Close()

	if err := service.SaveAIConfig(AIConfig{BaseURL: server.URL + "/v1", APIKey: "sk", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := service.TestConnection(); err == nil {
		t.Fatal("expected an auth error")
	}
}

func TestImportAndExportReports(t *testing.T) {
	db := newTestDB(t)
	service := NewReportService(db)
	dir := t.TempDir()

	markdownPath := filepath.Join(dir, "外部报告.md")
	markdown := "# 我的年度回顾\n\n今年一共游玩了 120 小时。"
	if err := os.WriteFile(markdownPath, []byte("\uFEFF"+markdown), 0o600); err != nil {
		t.Fatal(err)
	}

	imported, err := service.ImportFromFile(markdownPath)
	if err != nil {
		t.Fatalf("ImportFromFile(markdown) error = %v", err)
	}
	if imported.Title != "我的年度回顾" {
		t.Fatalf("markdown title = %q, want the first heading", imported.Title)
	}
	if imported.Source != reportSourceImport {
		t.Fatalf("source = %q", imported.Source)
	}
	if strings.HasPrefix(imported.Content, "\uFEFF") {
		t.Fatal("BOM should be stripped")
	}

	jsonPath := filepath.Join(dir, "imported.json")
	payload, err := json.Marshal(map[string]any{
		"title":       "导入的 JSON 报告",
		"periodType":  "month",
		"periodStart": "2026-08-01",
		"content":     "## 概览\n\n八月表现稳定。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	jsonReport, err := service.ImportFromFile(jsonPath)
	if err != nil {
		t.Fatalf("ImportFromFile(json) error = %v", err)
	}
	if jsonReport.Title != "导入的 JSON 报告" || jsonReport.PeriodType != PeriodMonth {
		t.Fatalf("json report = %#v", jsonReport)
	}
	if jsonReport.PeriodStart == nil || jsonReport.PeriodStart.Month() != time.August {
		t.Fatalf("period start = %v", jsonReport.PeriodStart)
	}

	// 无标题的文本文件使用文件名
	plainPath := filepath.Join(dir, "无标题报告.txt")
	if err := os.WriteFile(plainPath, []byte("纯文本内容"), 0o600); err != nil {
		t.Fatal(err)
	}
	plain, err := service.ImportFromFile(plainPath)
	if err != nil {
		t.Fatalf("ImportFromFile(txt) error = %v", err)
	}
	if plain.Title != "无标题报告" {
		t.Fatalf("plain title = %q, want the file name", plain.Title)
	}

	// 缺少正文的 JSON 应报错
	emptyJSONPath := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(emptyJSONPath, []byte(`{"title":"空报告"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ImportFromFile(emptyJSONPath); err == nil {
		t.Fatal("expected an error for json without content")
	}

	// 导出应返回正文并清理文件名中的非法字符
	tricky, err := service.Save(models.Report{Title: `报告 / 2026: "总结"`, Content: "正文"})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	content, filename, err := service.ExportMarkdown(tricky.ID)
	if err != nil {
		t.Fatalf("ExportMarkdown() error = %v", err)
	}
	if !strings.HasSuffix(content, "\n") {
		t.Fatal("exported markdown should end with a newline")
	}
	if strings.ContainsAny(filename, `\/:*?"<>|`) {
		t.Fatalf("filename %q still contains illegal characters", filename)
	}
	if !strings.HasSuffix(filename, ".md") {
		t.Fatalf("filename %q should end with .md", filename)
	}
	if !strings.Contains(filename, "2026") {
		t.Fatalf("filename %q should keep the safe part of the title", filename)
	}
}

func TestReportCRUD(t *testing.T) {
	db := newTestDB(t)
	service := NewReportService(db)

	if _, err := service.Save(models.Report{Title: "  ", Content: "x"}); err == nil {
		t.Fatal("expected error for a blank title")
	}
	if _, err := service.Save(models.Report{Title: "标题", Content: "   "}); err == nil {
		t.Fatal("expected error for blank content")
	}

	created, err := service.Save(models.Report{Title: "手动报告", Content: "# 标题\n正文"})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if created.Source != reportSourceImport {
		t.Fatalf("source = %q, want the import default", created.Source)
	}

	if _, err := service.Get(created.ID + 999); err != ErrNotFound {
		t.Fatalf("Get(unknown) error = %v, want ErrNotFound", err)
	}
	if err := service.Delete(created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := service.Delete(created.ID); err != ErrNotFound {
		t.Fatalf("Delete(again) error = %v, want ErrNotFound", err)
	}
}

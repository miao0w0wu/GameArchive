package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

const (
	coverOnlineSetting  = "cover_online_enabled"
	coverSGDBKeySetting = "cover_sgdb_api_key"
	coverRAWGKeySetting = "cover_rawg_api_key"
	maxImageSize        = 15 << 20
)

// CoverSettings 是在线封面来源的本机配置。
type CoverSettings struct {
	OnlineEnabled  bool   `json:"onlineEnabled"`
	SteamGridDBKey string `json:"steamGridDBKey"`
	RAWGKey        string `json:"rawgKey"`
	IconCacheDir   string `json:"iconCacheDir"`
	CacheDir       string `json:"cacheDir"`
}

// CoverResult 表示某张封面 / 图标的获取结果。
type CoverResult struct {
	GameID uint   `json:"gameId"`
	Target string `json:"target"`
	Path   string `json:"path"`
	Source string `json:"source"`
	Found  bool   `json:"found"`
	Reason string `json:"reason"`
}

// BatchCoverResult 是批量获取图片的统计。
type BatchCoverResult struct {
	Requested int           `json:"requested"`
	Fetched   int           `json:"fetched"`
	Skipped   int           `json:"skipped"`
	Failed    int           `json:"failed"`
	Results   []CoverResult `json:"results"`
}

// GameCoverInfo 返回游戏图片路径、来源、更新时间和缓存路径。
type GameCoverInfo struct {
	GameID         uint       `json:"gameId"`
	CoverPath      string     `json:"coverPath"`
	IconPath       string     `json:"iconPath"`
	CoverSource    string     `json:"coverSource"`
	IconSource     string     `json:"iconSource"`
	CoverUpdatedAt *time.Time `json:"coverUpdatedAt"`
	IconUpdatedAt  *time.Time `json:"iconUpdatedAt"`
	CacheDir       string     `json:"cacheDir"`
}

// LibraryDisplaySettings 是持久化的游戏库展示偏好。
type LibraryDisplaySettings struct {
	ViewMode string `json:"viewMode"`
	Columns  int    `json:"columns"`
	CardSize string `json:"cardSize"`
}

type CoverService struct {
	db         *gorm.DB
	cacheDir   string
	iconsDir   string
	httpClient *http.Client
}

func NewCoverService(db *gorm.DB, dataDir string) *CoverService {
	return &CoverService{
		db:       db,
		cacheDir: filepath.Join(dataDir, "covers"),
		iconsDir: filepath.Join(dataDir, "icons"),
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (s *CoverService) GetSettings() (CoverSettings, error) {
	values, err := s.loadSettings(coverOnlineSetting, coverSGDBKeySetting, coverRAWGKeySetting)
	if err != nil {
		return CoverSettings{}, err
	}
	enabled := values[coverOnlineSetting] != "false"
	return CoverSettings{
		OnlineEnabled:  enabled,
		SteamGridDBKey: values[coverSGDBKeySetting],
		RAWGKey:        values[coverRAWGKeySetting],
		CacheDir:       s.cacheDir,
		IconCacheDir:   s.iconsDir,
	}, nil
}

func (s *CoverService) SaveSettings(config CoverSettings) error {
	if len(config.SteamGridDBKey) > 512 || len(config.RAWGKey) > 512 {
		return errors.New("封面服务 API Key 不能超过 512 个字符")
	}
	online := "false"
	if config.OnlineEnabled {
		online = "true"
	}
	values := map[string]string{
		coverOnlineSetting:  online,
		coverSGDBKeySetting: strings.TrimSpace(config.SteamGridDBKey),
		coverRAWGKeySetting: strings.TrimSpace(config.RAWGKey),
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for key, value := range values {
			if err := tx.Save(&models.Setting{Key: key, Value: value}).Error; err != nil {
				return fmt.Errorf("保存封面设置失败: %w", err)
			}
		}
		return nil
	})
}

func (s *CoverService) GetLibraryDisplaySettings() (LibraryDisplaySettings, error) {
	values, err := s.loadSettings("library_view_mode", "library_columns", "library_card_size")
	if err != nil {
		return LibraryDisplaySettings{}, err
	}
	settings := LibraryDisplaySettings{
		ViewMode: values["library_view_mode"],
		Columns:  3,
		CardSize: values["library_card_size"],
	}
	if settings.ViewMode != "list" && settings.ViewMode != "card" {
		settings.ViewMode = "list"
	}
	if columns, err := strconv.Atoi(values["library_columns"]); err == nil && columns >= 2 && columns <= 6 {
		settings.Columns = columns
	}
	if settings.CardSize != "small" && settings.CardSize != "medium" && settings.CardSize != "large" {
		settings.CardSize = "medium"
	}
	return settings, nil
}

func (s *CoverService) SaveLibraryDisplaySettings(settings LibraryDisplaySettings) error {
	if settings.ViewMode != "list" && settings.ViewMode != "card" {
		return errors.New("展示模式必须为 list 或 card")
	}
	if settings.Columns < 2 || settings.Columns > 6 {
		return errors.New("每行卡片数量必须为 2 至 6")
	}
	if settings.CardSize != "small" && settings.CardSize != "medium" && settings.CardSize != "large" {
		return errors.New("卡片尺寸必须为 small、medium 或 large")
	}
	values := map[string]string{
		"library_view_mode": settings.ViewMode,
		"library_columns":   strconv.Itoa(settings.Columns),
		"library_card_size": settings.CardSize,
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for key, value := range values {
			if err := tx.Save(&models.Setting{Key: key, Value: value}).Error; err != nil {
				return fmt.Errorf("保存游戏库展示设置失败: %w", err)
			}
		}
		return nil
	})
}

func (s *CoverService) SetImage(gameID uint, target, sourcePath string) (string, error) {
	if target != "cover" && target != "icon" {
		return "", errors.New("图片类型必须为 cover 或 icon")
	}
	game, err := s.getGame(gameID)
	if err != nil {
		return "", err
	}
	sourcePath = filepath.Clean(strings.TrimSpace(sourcePath))
	file, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("打开所选图片失败: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("读取所选图片信息失败: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxImageSize {
		return "", fmt.Errorf("图片必须是小于 %d MiB 的普通文件", maxImageSize>>20)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil {
		return "", fmt.Errorf("读取所选图片失败: %w", err)
	}
	if len(data) == 0 || len(data) > maxImageSize {
		return "", fmt.Errorf("图片必须小于 %d MiB", maxImageSize>>20)
	}
	contentType, err := validateImage(data)
	if err != nil {
		return "", errors.New("所选文件不是有效图片")
	}
	cacheDir := s.cacheDir
	if target == "icon" {
		cacheDir = s.iconsDir
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("创建图片缓存目录失败: %w", err)
	}
	ext := imageExtension(contentType, filepath.Ext(sourcePath))
	now := time.Now()
	dst := filepath.Join(cacheDir, fmt.Sprintf("%d_manual_%d%s", gameID, now.UnixNano(), ext))
	if err := writeImageAtomically(dst, data); err != nil {
		return "", err
	}
	fields := imageFields(target, dst, "manual", now)
	if err := s.db.Model(&models.Game{}).Where("id = ?", game.ID).Updates(fields).Error; err != nil {
		return "", fmt.Errorf("保存游戏图片信息失败: %w", err)
	}
	return dst, nil
}

func (s *CoverService) ClearImage(gameID uint, target string) error {
	if target != "cover" && target != "icon" {
		return errors.New("图片类型必须为 cover 或 icon")
	}
	statement := "UPDATE games SET cover_path = '', cover_source = 'none', cover_updated_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?"
	if target == "icon" {
		statement = "UPDATE games SET icon_path = '', icon_source = 'none', icon_updated_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?"
	}
	result := s.db.Exec(statement, gameID)
	if result.Error != nil {
		return fmt.Errorf("清除游戏图片信息失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *CoverService) GetInfo(gameID uint) (GameCoverInfo, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return GameCoverInfo{}, err
	}
	return GameCoverInfo{
		GameID:         game.ID,
		CoverPath:      game.CoverPath,
		IconPath:       game.IconPath,
		CoverSource:    sourceOrNone(game.CoverSource),
		IconSource:     sourceOrNone(game.IconSource),
		CoverUpdatedAt: game.CoverUpdatedAt,
		IconUpdatedAt:  game.IconUpdatedAt,
		CacheDir:       s.cacheDir,
	}, nil
}

func (s *CoverService) GetImageData(gameID uint, target string) (string, error) {
	if target != "cover" && target != "icon" {
		return "", errors.New("图片类型必须为 cover 或 icon")
	}
	game, err := s.getGame(gameID)
	if err != nil {
		return "", err
	}
	paths := []string{game.IconPath, game.CoverPath}
	if target == "cover" {
		paths = []string{game.CoverPath, game.IconPath}
	}
	for _, path := range paths {
		if path == "" {
			continue
		}
		data, err := readImage(path)
		if err == nil {
			contentType, _ := validateImage(data)
			return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
		}
	}
	return "", nil
}

func (s *CoverService) AutoFetch(gameID uint, target string, explicit bool) (CoverResult, error) {
	if target != "cover" && target != "icon" {
		return CoverResult{}, errors.New("图片类型必须为 cover 或 icon")
	}
	game, err := s.getGame(gameID)
	if err != nil {
		return CoverResult{}, err
	}
	currentPath, currentSource := game.CoverPath, game.CoverSource
	if target == "icon" {
		currentPath, currentSource = game.IconPath, game.IconSource
	}
	if !explicit && currentPath != "" {
		return CoverResult{GameID: gameID, Target: target, Path: currentPath, Source: currentSource, Reason: "已有图片，跳过自动覆盖"}, nil
	}
	result := CoverResult{GameID: gameID, Target: target}
	for _, localPath := range findLocalImages(game, target) {
		data, readErr := readImage(localPath)
		if readErr == nil {
			path, cacheErr := s.cacheImage(game.ID, target, "local", data)
			if cacheErr == nil {
				return s.saveAutoImage(result, target, path, "local", true, "")
			}
			result.Reason = cacheErr.Error()
		}
	}
	if target == "icon" && game.ExePath != "" {
		if icon, iconErr := extractPEIcon(game.ExePath); iconErr == nil {
			path, cacheErr := s.cacheImage(game.ID, target, "icon", icon)
			if cacheErr == nil {
				return s.saveAutoImage(result, target, path, "icon", true, "")
			}
			result.Reason = cacheErr.Error()
		}
	}

	config, err := s.GetSettings()
	if err != nil {
		return result, err
	}
	if !config.OnlineEnabled {
		if result.Reason == "" {
			result.Reason = "在线封面搜索已关闭，未找到本地图片"
		}
		return result, nil
	}

	if game.SteamAppID != 0 {
		if path, found := s.fetchSteamArtwork(game, target); found {
			return s.saveAutoImage(result, target, path, "steam", true, "")
		}
	}
	appID := game.SteamAppID
	if appID == 0 {
		appID = s.searchSteamAppID(game.Name)
	}
	if appID != 0 {
		steamGame := *game
		steamGame.SteamAppID = appID
		if path, found := s.fetchSteamArtwork(&steamGame, target); found {
			return s.saveAutoImage(result, target, path, "steam", true, "")
		}
	}
	if config.SteamGridDBKey != "" {
		if path, found := s.fetchSteamGridDB(game.ID, game.Name, target, config.SteamGridDBKey); found {
			return s.saveAutoImage(result, target, path, "steamgriddb", true, "")
		}
	}
	if config.RAWGKey != "" {
		if path, found := s.fetchRAWG(game.ID, game.Name, config.RAWGKey); found {
			return s.saveAutoImage(result, target, path, "rawg", true, "")
		}
	}
	if result.Reason == "" {
		result.Reason = "未找到可用图片"
	}
	return result, nil
}

func (s *CoverService) BatchFetch(gameIDs []uint, target string) (BatchCoverResult, error) {
	return s.batchFetch(gameIDs, target, true)
}

func (s *CoverService) BatchFetchMissing(gameIDs []uint, target string) (BatchCoverResult, error) {
	return s.batchFetch(gameIDs, target, false)
}

func (s *CoverService) batchFetch(gameIDs []uint, target string, explicit bool) (BatchCoverResult, error) {
	if target != "cover" && target != "icon" {
		return BatchCoverResult{}, errors.New("图片类型必须为 cover 或 icon")
	}
	const maxBatchSize = 100
	if len(gameIDs) > maxBatchSize {
		return BatchCoverResult{}, fmt.Errorf("单次批量获取最多支持 %d 个游戏", maxBatchSize)
	}
	result := BatchCoverResult{Requested: len(gameIDs), Results: make([]CoverResult, len(gameIDs))}
	type job struct {
		index  int
		gameID uint
	}
	jobs := make(chan job)
	errorsFound := make(chan error, len(gameIDs))
	var workers sync.WaitGroup
	workerCount := 4
	if len(gameIDs) < workerCount {
		workerCount = len(gameIDs)
	}
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				fetched, err := s.AutoFetch(item.gameID, target, explicit)
				if err != nil {
					fetched = CoverResult{GameID: item.gameID, Target: target, Reason: err.Error()}
					errorsFound <- err
				}
				result.Results[item.index] = fetched
			}
		}()
	}
	for index, gameID := range gameIDs {
		jobs <- job{index: index, gameID: gameID}
	}
	close(jobs)
	workers.Wait()
	close(errorsFound)
	if err := <-errorsFound; err != nil {
		return result, fmt.Errorf("批量获取游戏图片失败: %w", err)
	}
	for _, item := range result.Results {
		switch {
		case item.Found:
			result.Fetched++
		case item.Reason == "已有图片，跳过自动覆盖":
			result.Skipped++
		default:
			result.Failed++
		}
	}
	return result, nil
}

func (s *CoverService) fetchSteamArtwork(game *models.Game, target string) (string, bool) {
	if game.SteamAppID == 0 {
		return "", false
	}
	appID := strconv.FormatUint(game.SteamAppID, 10)
	imageURL := "https://cdn.akamai.steamstatic.com/steam/apps/" + appID + "/logo.png"
	if target == "cover" {
		imageURL = "https://cdn.akamai.steamstatic.com/steam/apps/" + appID + "/library_600x900.jpg"
	}
	data, _, ok := s.downloadImage(imageURL, nil)
	if ok {
		path, err := s.cacheImage(game.ID, target, "steam", data)
		if err == nil {
			return path, true
		}
	}
	detailsURL := "https://store.steampowered.com/api/appdetails?appids=" + appID + "&l=english"
	var details map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			HeaderImage  string `json:"header_image"`
			CapsuleImage string `json:"capsule_image"`
		} `json:"data"`
	}
	if !s.getJSON(detailsURL, nil, &details) {
		return "", false
	}
	item := details[appID]
	candidate := item.Data.HeaderImage
	if target == "cover" && item.Data.CapsuleImage != "" {
		candidate = item.Data.CapsuleImage
	}
	if !item.Success || candidate == "" {
		return "", false
	}
	data, _, ok = s.downloadImage(candidate, nil)
	if !ok {
		return "", false
	}
	path, err := s.cacheImage(game.ID, target, "steam", data)
	return path, err == nil
}

func (s *CoverService) searchSteamAppID(name string) uint64 {
	endpoint := "https://store.steampowered.com/api/storesearch/?l=english&cc=US&term=" + url.QueryEscape(name)
	var response struct {
		Items []struct {
			ID   uint64 `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if !s.getJSON(endpoint, nil, &response) || len(response.Items) == 0 {
		return 0
	}
	if normalizeSteamName(response.Items[0].Name) != normalizeSteamName(name) {
		return 0
	}
	return response.Items[0].ID
}

func (s *CoverService) fetchSteamGridDB(gameID uint, name, target, apiKey string) (string, bool) {
	headers := http.Header{"Authorization": []string{"Bearer " + apiKey}}
	searchURL := "https://www.steamgriddb.com/api/v2/search/autocomplete/" + url.PathEscape(name)
	var search struct {
		Data []struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	if !s.getJSON(searchURL, headers, &search) || len(search.Data) == 0 {
		return "", false
	}
	endpoint := fmt.Sprintf("https://www.steamgriddb.com/api/v2/%s/game/%d", map[string]string{"cover": "grids", "icon": "icons"}[target], search.Data[0].ID)
	var response struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if !s.getJSON(endpoint, headers, &response) || len(response.Data) == 0 {
		return "", false
	}
	data, _, ok := s.downloadImage(response.Data[0].URL, nil)
	if !ok {
		return "", false
	}
	path, err := s.cacheImage(gameID, target, "steamgriddb", data)
	return path, err == nil
}

func (s *CoverService) fetchRAWG(gameID uint, name, apiKey string) (string, bool) {
	endpoint := "https://api.rawg.io/api/games?key=" + url.QueryEscape(apiKey) + "&search=" + url.QueryEscape(name) + "&page_size=5"
	var response struct {
		Results []struct {
			Name            string `json:"name"`
			BackgroundImage string `json:"background_image"`
		} `json:"results"`
	}
	if !s.getJSON(endpoint, nil, &response) {
		return "", false
	}
	for _, item := range response.Results {
		if item.BackgroundImage == "" || normalizeSteamName(item.Name) != normalizeSteamName(name) {
			continue
		}
		data, _, ok := s.downloadImage(item.BackgroundImage, nil)
		if !ok {
			return "", false
		}
		path, err := s.cacheImage(gameID, "cover", "rawg", data)
		return path, err == nil
	}
	return "", false
}

func (s *CoverService) downloadImage(imageURL string, headers http.Header) ([]byte, string, bool) {
	parsed, err := url.Parse(imageURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, "", false
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", false
	}
	if headers != nil {
		request.Header = headers.Clone()
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, "", false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maxImageSize {
		return nil, "", false
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxImageSize+1))
	if err != nil || len(data) == 0 || len(data) > maxImageSize {
		return nil, "", false
	}
	contentType, err := validateImage(data)
	if err != nil {
		return nil, "", false
	}
	return data, contentType, true
}

func (s *CoverService) getJSON(endpoint string, headers http.Header, target any) bool {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	request.Header.Set("Accept", "application/json")
	if headers != nil {
		for key, values := range headers {
			for _, value := range values {
				request.Header.Add(key, value)
			}
		}
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(target); err != nil {
		return false
	}
	return true
}

func (s *CoverService) cacheImage(gameID uint, target, source string, data []byte) (string, error) {
	dir := s.cacheDir
	if target == "icon" {
		dir = s.iconsDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建图片缓存目录失败: %w", err)
	}
	contentType := http.DetectContentType(data)
	path := filepath.Join(dir, fmt.Sprintf("%d_%s%s", gameID, source, imageExtension(contentType, "")))
	if err := writeImageAtomically(path, data); err != nil {
		return "", err
	}
	return path, nil
}

func (s *CoverService) saveAutoImage(result CoverResult, target, path, source string, found bool, reason string) (CoverResult, error) {
	result.Path = path
	result.Source = source
	result.Found = found
	result.Reason = reason
	if found {
		fields := imageFields(target, path, source, time.Now())
		if err := s.db.Model(&models.Game{}).Where("id = ?", result.GameID).Updates(fields).Error; err != nil {
			return result, fmt.Errorf("更新游戏图片信息失败: %w", err)
		}
	}
	return result, nil
}

func (s *CoverService) loadSettings(keys ...string) (map[string]string, error) {
	settings := make([]models.Setting, 0, len(keys))
	if err := s.db.Where("key IN ?", keys).Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("读取图片设置失败: %w", err)
	}
	values := make(map[string]string, len(settings))
	for _, setting := range settings {
		values[setting.Key] = setting.Value
	}
	return values, nil
}

func (s *CoverService) getGame(gameID uint) (*models.Game, error) {
	var game models.Game
	err := s.db.First(&game, gameID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询游戏图片信息失败: %w", err)
	}
	return &game, nil
}

func findLocalImages(game *models.Game, target string) []string {
	dir := game.InstallDir
	if dir == "" && game.ExePath != "" {
		dir = filepath.Dir(game.ExePath)
	}
	if dir == "" {
		return nil
	}
	names := []string{"cover.jpg", "cover.jpeg", "folder.jpg", "folder.png", "header.jpg", "header.png", "capsule.jpg", "cover.png"}
	if target == "icon" {
		names = []string{"logo.png", "icon.png", "icon.jpg", "icon.ico", "game.ico", "folder.jpg"}
	}
	var paths []string
	for _, name := range names {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() <= maxImageSize {
			paths = append(paths, path)
		}
	}
	return paths
}

func imageFields(target, path, source string, updatedAt time.Time) map[string]any {
	fields := map[string]any{}
	if target == "cover" {
		fields["cover_path"] = path
		fields["cover_source"] = source
		fields["cover_updated_at"] = nullableTime(updatedAt)
	} else {
		fields["icon_path"] = path
		fields["icon_source"] = source
		fields["icon_updated_at"] = nullableTime(updatedAt)
	}
	return fields
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func imageExtension(contentType, fallback string) string {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	switch mediaType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico"
	}
	ext := strings.ToLower(fallback)
	if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" || ext == ".ico" {
		if ext == ".jpeg" {
			return ".jpg"
		}
		return ext
	}
	return ".img"
}

func writeImageAtomically(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".cover-*")
	if err != nil {
		return fmt.Errorf("创建图片缓存临时文件失败: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("写入图片缓存失败: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("关闭图片缓存失败: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("保存图片缓存失败: %w", err)
	}
	return nil
}

func readImage(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxImageSize {
		return nil, errors.New("图片无效或超过大小限制")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil {
		return nil, err
	}
	if _, err := validateImage(data); err != nil {
		return nil, err
	}
	return data, nil
}

func validateImage(data []byte) (string, error) {
	if len(data) >= 22 && data[0] == 0 && data[1] == 0 && data[2] == 1 && data[3] == 0 {
		count := int(data[4]) | int(data[5])<<8
		if count == 0 || len(data) < 6+count*16 {
			return "", errors.New("ICO 图像目录无效")
		}
		for i := 0; i < count; i++ {
			entry := data[6+i*16 : 6+(i+1)*16]
			size := int(entry[8]) | int(entry[9])<<8 | int(entry[10])<<16 | int(entry[11])<<24
			offset := int(entry[12]) | int(entry[13])<<8 | int(entry[14])<<16 | int(entry[15])<<24
			if size > 0 && offset >= 0 && offset <= len(data) && size <= len(data)-offset {
				payload := data[offset : offset+size]
				if _, _, err := image.DecodeConfig(bytes.NewReader(payload)); err == nil {
					return "image/x-icon", nil
				}
				if len(payload) >= 40 && payload[0] >= 40 {
					return "image/x-icon", nil
				}
			}
		}
		return "", errors.New("ICO 图像内容无效")
	}
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return "", errors.New("文件不是图像")
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		return "", err
	}
	return contentType, nil
}

func sourceOrNone(source string) string {
	if source == "" {
		return "none"
	}
	return source
}

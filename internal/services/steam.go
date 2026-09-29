package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

const (
	steamAPIKeySetting      = "steam_api_key"
	steamIDSetting          = "steam_id"
	steamLastResultSetting  = "steam_last_sync_result"
	steamLastAttemptSetting = "steam_last_sync_attempt"
	steamLastErrorSetting   = "steam_last_sync_error"
	steamAPIEndpoint        = "https://api.steampowered.com/IPlayerService/GetOwnedGames/v1/"
)

// SteamConfig 是用户提供的 Steam Web API 凭据。
type SteamConfig struct {
	APIKey  string `json:"apiKey"`
	SteamID string `json:"steamId"`
}

// SteamSyncResult 描述一次 Steam 游戏库同步的处理结果。
type SteamSyncResult struct {
	FetchedGames         int       `json:"fetchedGames"`
	MatchedByAppID       int       `json:"matchedByAppId"`
	MatchedByName        int       `json:"matchedByName"`
	ImportedGames        int       `json:"importedGames"`
	TotalPlaytimeSeconds int64     `json:"totalPlaytimeSeconds"`
	SyncedAt             time.Time `json:"syncedAt"`
}

// SteamSyncStatus 保存最近一次同步结果或错误，供设置页恢复展示。
type SteamSyncStatus struct {
	LastAttemptAt string           `json:"lastAttemptAt"`
	LastError     string           `json:"lastError"`
	LastResult    *SteamSyncResult `json:"lastResult"`
}

type steamOwnedGame struct {
	AppID           uint64 `json:"appid"`
	Name            string `json:"name"`
	PlaytimeForever uint64 `json:"playtime_forever"`
}

type steamOwnedGamesResponse struct {
	Response struct {
		GameCount int              `json:"game_count"`
		Games     []steamOwnedGame `json:"games"`
	} `json:"response"`
}

// SteamService 负责读取 Steam 游戏库并将游玩时长关联到本地游戏。
type SteamService struct {
	db         *gorm.DB
	httpClient *http.Client
	endpoint   string
	syncMu     sync.Mutex
}

// NewSteamService 创建 Steam 接入服务。
func NewSteamService(db *gorm.DB) *SteamService {
	return &SteamService{
		db:       db,
		endpoint: steamAPIEndpoint,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// GetConfig 返回本机保存的 Steam Web API 配置。
func (s *SteamService) GetConfig() (SteamConfig, error) {
	values, err := s.loadSettings(steamAPIKeySetting, steamIDSetting)
	if err != nil {
		return SteamConfig{}, err
	}
	return SteamConfig{
		APIKey:  values[steamAPIKeySetting],
		SteamID: values[steamIDSetting],
	}, nil
}

// SaveConfig 保存 Steam Web API 配置；API Key 与 SteamID 必须同时填写或同时清空。
func (s *SteamService) SaveConfig(config SteamConfig) error {
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.SteamID = strings.TrimSpace(config.SteamID)
	if (config.APIKey == "") != (config.SteamID == "") {
		return errors.New("Steam API Key 和 SteamID 必须同时填写")
	}
	if len(config.APIKey) > 256 {
		return errors.New("Steam API Key 不能超过 256 字节")
	}
	if config.SteamID != "" && !validSteamID(config.SteamID) {
		return errors.New("SteamID 必须是 17 位数字的 SteamID64")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for key, value := range map[string]string{
			steamAPIKeySetting: config.APIKey,
			steamIDSetting:     config.SteamID,
		} {
			setting := models.Setting{Key: key, Value: value}
			if err := tx.Save(&setting).Error; err != nil {
				return fmt.Errorf("保存 Steam 配置失败: %w", err)
			}
		}
		if err := tx.Delete(&models.Setting{}, "key = ?", steamLastErrorSetting).Error; err != nil {
			return fmt.Errorf("清除 Steam 同步错误状态失败: %w", err)
		}
		return nil
	})
}

// GetSyncStatus 返回最近一次同步的结果或错误。
func (s *SteamService) GetSyncStatus() (SteamSyncStatus, error) {
	values, err := s.loadSettings(steamLastAttemptSetting, steamLastErrorSetting, steamLastResultSetting)
	if err != nil {
		return SteamSyncStatus{}, err
	}
	status := SteamSyncStatus{
		LastAttemptAt: values[steamLastAttemptSetting],
		LastError:     values[steamLastErrorSetting],
	}
	if raw := values[steamLastResultSetting]; raw != "" {
		var result SteamSyncResult
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return SteamSyncStatus{}, fmt.Errorf("解析 Steam 同步结果失败: %w", err)
		}
		status.LastResult = &result
	}
	return status, nil
}

// SyncLibrary 从 Steam 获取游戏库并更新本地匹配记录。
func (s *SteamService) SyncLibrary(ctx context.Context) (SteamSyncResult, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	config, err := s.GetConfig()
	if err != nil {
		return SteamSyncResult{}, err
	}
	if config.APIKey == "" || config.SteamID == "" {
		return SteamSyncResult{}, errors.New("请先在设置中填写 Steam API Key 和 SteamID")
	}

	attemptAt := time.Now().UTC()
	games, err := s.fetchOwnedGames(ctx, config)
	if err != nil {
		if saveErr := s.saveSyncError(attemptAt, err); saveErr != nil {
			return SteamSyncResult{}, fmt.Errorf("%w（保存同步错误状态失败: %v）", err, saveErr)
		}
		return SteamSyncResult{}, err
	}

	result := SteamSyncResult{
		FetchedGames: len(games),
		SyncedAt:     attemptAt,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		localGames := make([]models.Game, 0)
		if err := tx.Find(&localGames).Error; err != nil {
			return fmt.Errorf("读取本地游戏失败: %w", err)
		}
		byAppID := make(map[uint64]int, len(localGames))
		byName := make(map[string][]int, len(localGames))
		for i := range localGames {
			if localGames[i].SteamAppID != 0 {
				byAppID[localGames[i].SteamAppID] = i
			}
			name := normalizeSteamName(localGames[i].Name)
			if name != "" {
				byName[name] = append(byName[name], i)
			}
		}

		for _, steamGame := range games {
			if steamGame.AppID == 0 || strings.TrimSpace(steamGame.Name) == "" {
				continue
			}
			playtimeSeconds := int64(steamGame.PlaytimeForever * 60)
			gameIndex, found := byAppID[steamGame.AppID]
			if found {
				result.MatchedByAppID++
				game := &localGames[gameIndex]
				game.SteamPlaytimeSeconds = playtimeSeconds
				game.SteamLastSyncedAt = &attemptAt
				if err := tx.Model(&models.Game{}).Where("id = ?", game.ID).Updates(map[string]any{
					"steam_playtime_seconds": playtimeSeconds,
					"steam_last_synced_at":   attemptAt,
				}).Error; err != nil {
					return fmt.Errorf("更新 Steam 游戏时长失败（%s）: %w", steamGame.Name, err)
				}
				result.TotalPlaytimeSeconds += playtimeSeconds
				continue
			}

			matches := byName[normalizeSteamName(steamGame.Name)]
			if len(matches) == 1 {
				gameIndex = matches[0]
				game := &localGames[gameIndex]
				game.SteamAppID = steamGame.AppID
				game.SteamPlaytimeSeconds = playtimeSeconds
				game.SteamLastSyncedAt = &attemptAt
				if err := tx.Model(&models.Game{}).Where("id = ?", game.ID).Updates(map[string]any{
					"steam_app_id":           steamGame.AppID,
					"steam_playtime_seconds": playtimeSeconds,
					"steam_last_synced_at":   attemptAt,
				}).Error; err != nil {
					return fmt.Errorf("关联本地游戏失败（%s）: %w", steamGame.Name, err)
				}
				byAppID[steamGame.AppID] = gameIndex
				result.MatchedByName++
			} else {
				game := models.Game{
					Name:                 strings.TrimSpace(steamGame.Name),
					SteamAppID:           steamGame.AppID,
					SteamPlaytimeSeconds: playtimeSeconds,
					SteamLastSyncedAt:    &attemptAt,
				}
				if err := tx.Create(&game).Error; err != nil {
					return fmt.Errorf("导入 Steam 游戏失败（%s）: %w", steamGame.Name, err)
				}
				gameIndex = len(localGames)
				localGames = append(localGames, game)
				byAppID[steamGame.AppID] = gameIndex
				byName[normalizeSteamName(game.Name)] = append(byName[normalizeSteamName(game.Name)], gameIndex)
				result.ImportedGames++
			}
			result.TotalPlaytimeSeconds += playtimeSeconds
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("编码 Steam 同步结果失败: %w", err)
		}
		for key, value := range map[string]string{
			steamLastAttemptSetting: attemptAt.Format(time.RFC3339),
			steamLastErrorSetting:   "",
			steamLastResultSetting:  string(resultJSON),
		} {
			if err := tx.Save(&models.Setting{Key: key, Value: value}).Error; err != nil {
				return fmt.Errorf("保存 Steam 同步结果失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		if saveErr := s.saveSyncError(attemptAt, err); saveErr != nil {
			return SteamSyncResult{}, fmt.Errorf("%w（保存同步错误状态失败: %v）", err, saveErr)
		}
		return SteamSyncResult{}, err
	}
	return result, nil
}

func (s *SteamService) fetchOwnedGames(ctx context.Context, config SteamConfig) ([]steamOwnedGame, error) {
	requestURL, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, fmt.Errorf("Steam API 地址无效: %w", err)
	}
	query := requestURL.Query()
	query.Set("key", config.APIKey)
	query.Set("steamid", config.SteamID)
	query.Set("include_appinfo", "1")
	query.Set("include_played_free_games", "1")
	requestURL.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Steam API 请求失败: %w", err)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("请求 Steam 游戏库失败: %s", redactSteamSecret(err.Error(), config.APIKey))
	}
	defer response.Body.Close()

	const maxResponseBytes = 32 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取 Steam API 响应失败: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, errors.New("Steam API 响应超过 32 MiB 限制")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf(
			"Steam API 返回 HTTP %d: %s",
			response.StatusCode,
			redactSteamSecret(strings.TrimSpace(string(body)), config.APIKey),
		)
	}

	var payload steamOwnedGamesResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析 Steam 游戏库响应失败: %w", err)
	}
	if payload.Response.Games == nil && payload.Response.GameCount > 0 {
		return nil, errors.New("Steam API 未返回游戏列表")
	}
	if payload.Response.Games == nil {
		return nil, errors.New("Steam 未返回游戏库；请确认资料库隐私设置允许查看游戏详情")
	}
	return payload.Response.Games, nil
}

func (s *SteamService) saveSyncError(attemptAt time.Time, cause error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		values := map[string]string{
			steamLastAttemptSetting: attemptAt.Format(time.RFC3339),
			steamLastErrorSetting:   cause.Error(),
		}
		for key, value := range values {
			if err := tx.Save(&models.Setting{Key: key, Value: value}).Error; err != nil {
				return fmt.Errorf("保存 Steam 同步状态失败: %w", err)
			}
		}
		return nil
	})
}

func (s *SteamService) loadSettings(keys ...string) (map[string]string, error) {
	settings := make([]models.Setting, 0, len(keys))
	if err := s.db.Where("key IN ?", keys).Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("读取 Steam 配置失败: %w", err)
	}
	values := make(map[string]string, len(settings))
	for _, setting := range settings {
		values[setting.Key] = setting.Value
	}
	return values, nil
}

func validSteamID(value string) bool {
	if len(value) != 17 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	if _, err := strconv.ParseUint(value, 10, 64); err != nil {
		return false
	}
	return true
}

func normalizeSteamName(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return unicode.ToLower(r)
	}, strings.Join(strings.Fields(name), " "))
}

func redactSteamSecret(message, apiKey string) string {
	if apiKey == "" {
		return message
	}
	message = strings.ReplaceAll(message, apiKey, "[REDACTED]")
	escapedKey := url.QueryEscape(apiKey)
	if escapedKey != apiKey {
		message = strings.ReplaceAll(message, escapedKey, "[REDACTED]")
	}
	return message
}

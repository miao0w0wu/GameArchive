package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

const (
	steamAppDetailsEndpoint = "https://store.steampowered.com/api/appdetails"
	genreRequestInterval    = 1500 * time.Millisecond
)

// BatchGenreResult 是一批 Steam 类型同步的结果统计。
type BatchGenreResult struct {
	Requested int `json:"requested"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
}

type steamAppDetailsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Genres []struct {
			ID          json.RawMessage `json:"id"`
			Description string          `json:"description"`
		} `json:"genres"`
	} `json:"data"`
}

type steamGenreRequestError struct {
	cause error
}

func (e *steamGenreRequestError) Error() string {
	return e.cause.Error()
}

func (e *steamGenreRequestError) Unwrap() error {
	return e.cause
}

// GenreService 获取并保存 Steam 官方游戏类型，不处理标签。
type GenreService struct {
	db         *gorm.DB
	httpClient *http.Client
	endpoint   string

	requestMu     sync.Mutex
	lastRequestAt time.Time
	backoffUntil  time.Time
	failureCount  int
}

// NewGenreService 创建 Steam 类型服务。
func NewGenreService(db *gorm.DB) *GenreService {
	return &GenreService{
		db:       db,
		endpoint: steamAppDetailsEndpoint,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// FetchGameGenres 获取并替换游戏的 Steam 类型；接口失败时记录日志并保留已有数据。
func (s *GenreService) FetchGameGenres(gameID uint) ([]models.Genre, error) {
	genres, skipped, err := s.fetchGameGenres(gameID)
	if err != nil {
		var requestErr *steamGenreRequestError
		if errors.As(err, &requestErr) {
			log.Printf("获取游戏 Steam 类型失败（游戏 ID %d）: %v", gameID, requestErr)
			return []models.Genre{}, nil
		}
		return []models.Genre{}, err
	}
	if skipped {
		return []models.Genre{}, nil
	}
	return genres, nil
}

// BatchFetchGameGenres 按顺序同步多个游戏；网络失败会记录并继续处理。
func (s *GenreService) BatchFetchGameGenres(gameIDs []uint) (BatchGenreResult, error) {
	result := BatchGenreResult{Requested: len(gameIDs)}
	for _, gameID := range gameIDs {
		_, skipped, err := s.fetchGameGenres(gameID)
		switch {
		case skipped:
			result.Skipped++
		case err != nil:
			result.Failed++
			log.Printf("获取游戏 Steam 类型失败（游戏 ID %d）: %v", gameID, err)
		default:
			result.Succeeded++
		}
	}
	return result, nil
}

// GetGameGenres 查询游戏的类型，包括 Steam 同步和其他来源的类型关联。
func (s *GenreService) GetGameGenres(gameID uint) ([]models.Genre, error) {
	if _, err := s.getGame(gameID); err != nil {
		return nil, err
	}
	genres := make([]models.Genre, 0)
	err := s.db.Table("genres").
		Distinct("genres.*").
		Joins("JOIN game_genres ON game_genres.genre_id = genres.id").
		Where("game_genres.game_id = ?", gameID).
		Order("genres.name").
		Find(&genres).Error
	if err != nil {
		return nil, fmt.Errorf("查询游戏类型失败: %w", err)
	}
	return genres, nil
}

func (s *GenreService) fetchGameGenres(gameID uint) ([]models.Genre, bool, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return nil, false, err
	}
	if game.SteamAppID == 0 {
		log.Printf("跳过 Steam 类型同步：游戏 %q（ID %d）未关联 Steam AppID", game.Name, game.ID)
		return nil, true, nil
	}

	steamGenres, err := s.fetchSteamGenres(game.SteamAppID)
	if err != nil {
		return nil, false, &steamGenreRequestError{
			cause: fmt.Errorf("游戏 %q（AppID %d）: %w", game.Name, game.SteamAppID, err),
		}
	}
	genres, err := s.replaceSteamGenres(game.ID, steamGenres)
	if err != nil {
		return nil, false, fmt.Errorf("保存游戏 Steam 类型失败（游戏 ID %d）: %w", game.ID, err)
	}
	return genres, false, nil
}

func (s *GenreService) getGame(gameID uint) (*models.Game, error) {
	if gameID == 0 {
		return nil, errors.New("缺少游戏 ID")
	}
	var game models.Game
	if err := s.db.First(&game, gameID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("查询游戏失败: %w", err)
	}
	return &game, nil
}

func (s *GenreService) fetchSteamGenres(appID uint64) ([]steamGenre, error) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	s.waitForRequest()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	requestURL, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, fmt.Errorf("解析 Steam appdetails 地址失败: %w", err)
	}
	query := requestURL.Query()
	query.Set("appids", strconv.FormatUint(appID, 10))
	query.Set("l", "schinese")
	requestURL.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		s.recordRequestFailure(false)
		return nil, fmt.Errorf("创建 Steam appdetails 请求失败: %w", err)
	}
	s.lastRequestAt = time.Now()
	response, err := s.httpClient.Do(request)
	if err != nil {
		s.recordRequestFailure(false)
		return nil, fmt.Errorf("请求 Steam appdetails 失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		s.recordRequestFailure(response.StatusCode == http.StatusTooManyRequests)
		return nil, fmt.Errorf("Steam appdetails 返回 HTTP %d", response.StatusCode)
	}

	const maxResponseBytes = 4 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		s.recordRequestFailure(false)
		return nil, fmt.Errorf("读取 Steam appdetails 响应失败: %w", err)
	}
	if len(body) > maxResponseBytes {
		s.recordRequestFailure(false)
		return nil, errors.New("Steam appdetails 响应超过 4 MiB 限制")
	}
	var payload map[string]steamAppDetailsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		s.recordRequestFailure(false)
		return nil, fmt.Errorf("解析 Steam appdetails 响应失败: %w", err)
	}
	s.recordRequestSuccess()

	appData, ok := payload[strconv.FormatUint(appID, 10)]
	if !ok || !appData.Success {
		return []steamGenre{}, nil
	}
	genres := make([]steamGenre, 0, len(appData.Data.Genres))
	seenNames := make(map[string]struct{}, len(appData.Data.Genres))
	for _, item := range appData.Data.Genres {
		name := strings.TrimSpace(item.Description)
		genreID := parseSteamGenreID(item.ID)
		if name == "" || genreID == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := seenNames[key]; exists {
			continue
		}
		seenNames[key] = struct{}{}
		genres = append(genres, steamGenre{SteamGenreID: genreID, Name: name})
	}
	return genres, nil
}

type steamGenre struct {
	SteamGenreID string
	Name         string
}

func parseSteamGenreID(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return ""
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return ""
		}
		value = strings.TrimSpace(decoded)
	}
	if value == "" || len(value) > 32 {
		return ""
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return ""
		}
	}
	return value
}

func (s *GenreService) replaceSteamGenres(gameID uint, steamGenres []steamGenre) ([]models.Genre, error) {
	saved := make([]models.Genre, 0, len(steamGenres))
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range steamGenres {
			var genre models.Genre
			err := tx.Where("steam_genre_id = ?", item.SteamGenreID).First(&genre).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				genre = models.Genre{SteamGenreID: item.SteamGenreID, Name: item.Name}
				if err := tx.Create(&genre).Error; err != nil {
					return fmt.Errorf("新增 Steam 类型 %q 失败: %w", item.Name, err)
				}
			} else if err != nil {
				return fmt.Errorf("查询 Steam 类型 %q 失败: %w", item.Name, err)
			} else if genre.Name != item.Name {
				if err := tx.Model(&genre).Update("name", item.Name).Error; err != nil {
					return fmt.Errorf("更新 Steam 类型 %q 失败: %w", item.Name, err)
				}
				genre.Name = item.Name
			}
			saved = append(saved, genre)
		}

		if err := tx.Where("game_id = ? AND source = ?", gameID, "steam").Delete(&models.GameGenre{}).Error; err != nil {
			return fmt.Errorf("清理旧 Steam 类型关联失败: %w", err)
		}
		for _, genre := range saved {
			association := models.GameGenre{GameID: gameID, GenreID: genre.ID, Source: "steam"}
			if err := tx.Create(&association).Error; err != nil {
				return fmt.Errorf("写入 Steam 类型关联失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *GenreService) waitForRequest() {
	// 在服务内串行请求，并同时遵守最低间隔和失败退避时间。
	now := time.Now()
	wait := time.Duration(0)
	if !s.lastRequestAt.IsZero() {
		if remaining := genreRequestInterval - now.Sub(s.lastRequestAt); remaining > wait {
			wait = remaining
		}
	}
	if remaining := s.backoffUntil.Sub(now); remaining > wait {
		wait = remaining
	}
	if wait > 0 {
		time.Sleep(wait)
	}
}

func (s *GenreService) recordRequestFailure(rateLimited bool) {
	s.failureCount++
	delay := 3 * time.Second
	if rateLimited {
		delay = 15 * time.Second
	}
	for i := 1; i < s.failureCount && delay < 5*time.Minute; i++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	s.backoffUntil = time.Now().Add(delay)
}

func (s *GenreService) recordRequestSuccess() {
	s.failureCount = 0
	s.backoffUntil = time.Time{}
}

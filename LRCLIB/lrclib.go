package lrclib

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"
)

// LyricsResponse 定义 LRCLIB 返回的歌词 JSON 数据结构
type LyricsResponse struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"` // LRC 格式的时间戳歌词
}
type RequestInfo struct {
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int    `json:"duration"`
}
type lyricsRequest struct {
	client *http.Client
	rInfo  *RequestInfo
}

func NewLyricsRequest(client *http.Client, rInfo *RequestInfo) *lyricsRequest {
	return &lyricsRequest{
		client: client,
		rInfo:  rInfo,
	}
}

// GetLyricsWithRetry 带退阶重试（Exponential Backoff）的歌词请求函数
func (l *lyricsRequest) GetLyricsWithRetry() (*LyricsResponse, error) {
	// 参数：
	//   - trackName: 曲名（必填）
	//   - artistName: 艺术家/歌手名（必填）
	//   - albumName: 专辑名（可选，传空字符串则忽略）
	//   - duration: 歌曲时长，单位秒（可选，若不传可填 0）
	var (
		trackName  = l.rInfo.Title
		artistName = l.rInfo.Artist
		albumName  = l.rInfo.Album
		duration   = l.rInfo.Duration
	)
	baseURL := "https://lrclib.net/api/get"

	params := url.Values{}
	params.Set("track_name", trackName)
	params.Set("artist_name", artistName)
	if albumName != "" {
		params.Set("album_name", albumName)
	}
	if duration > 0 {
		params.Set("duration", fmt.Sprintf("%d", duration))
	}

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	client := l.client

	maxRetries := 5
	baseDelay := 1 * time.Second

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("创建请求失败: %w", err)
		}

		// 设置合规的 User-Agent
		req.Header.Set("User-Agent", "music-downloader (https://github.com/IanHan-cn/music-downloader)")
		req.Header.Set("Lrclib-App-Id", os.Getenv("LRCLIB_APP_ID"))
		req.Header.Set("Lrclib-Api-Key", os.Getenv("LRCLIB_APP_KEY"))

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			// 网络错误同样触发退阶重试
		} else {
			defer resp.Body.Close()

			// 如果遇到 503 或 429（请求过多），进行退阶重试
			if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
				// 优先尝试读取 Retry-After 头部（此处简化处理，主要依靠退阶算法）
				resp.Body.Close()
				lastErr = fmt.Errorf("服务器超载或限流 (HTTP %d)", resp.StatusCode)
			} else if resp.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("未找到相关歌词 (404)")
			} else if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return nil, fmt.Errorf("API 返回错误状态码: %d", resp.StatusCode)
			} else {
				// 请求成功，解析 JSON
				var lyrics LyricsResponse
				if err := json.NewDecoder(resp.Body).Decode(&lyrics); err != nil {
					return nil, fmt.Errorf("解析 JSON 失败: %w", err)
				}
				return &lyrics, nil
			}
		}

		// 如果已经是最后一次尝试，则跳出循环返回错误
		if attempt == maxRetries {
			break
		}

		// 计算退阶延迟时间：baseDelay * 2^attempt，并加入随机抖动防止惊群效应
		multiplier := math.Pow(2, float64(attempt))
		sleepDuration := time.Duration(float64(baseDelay) * multiplier)

		// 加上 0~500ms 的随机抖动
		jitter := time.Duration(rand.Intn(500)) * time.Millisecond
		totalSleep := sleepDuration + jitter

		fmt.Printf("请求遇到问题: %v。正在进行第 %d 次重试，等待 %v...\n", lastErr, attempt+1, totalSleep)
		time.Sleep(totalSleep)
	}

	return nil, fmt.Errorf("达到最大重试次数，请求失败: %w", lastErr)
}

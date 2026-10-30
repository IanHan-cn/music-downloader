package lrclib

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"
)

// 退避重试相关参数：整体等待时间受 maxTotalWait 限制，避免高并发时大量 goroutine 长时间阻塞
const (
	maxRetries     = 3
	baseDelay      = 200 * time.Millisecond
	maxSingleDelay = 1 * time.Second
	maxTotalWait   = 3 * time.Second
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
// ctx 取消或超时会立即中断重试等待
func (l *lyricsRequest) GetLyricsWithRetry(ctx context.Context) (*LyricsResponse, error) {
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

	var lastErr error
	var waited time.Duration

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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

		// 已经是最后一次尝试或已耗尽等待预算，则跳出循环返回错误
		remaining := maxTotalWait - waited
		if attempt == maxRetries || remaining <= 0 {
			break
		}

		// 计算退阶延迟时间：baseDelay * 2^attempt，单次上限 maxSingleDelay
		sleepDuration := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
		if sleepDuration > maxSingleDelay {
			sleepDuration = maxSingleDelay
		}

		// 加入 0~100ms 的随机抖动，防止惊群效应
		jitter := time.Duration(rand.Intn(100)) * time.Millisecond
		totalSleep := sleepDuration + jitter
		if totalSleep > remaining {
			totalSleep = remaining
		}

		slog.Info("歌词请求遇到问题，准备重试",
			"error", lastErr, "attempt", attempt+1, "wait", totalSleep.String())

		timer := time.NewTimer(totalSleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("重试等待被中断: %w", ctx.Err())
		case <-timer.C:
		}
		waited += totalSleep
	}

	return nil, fmt.Errorf("达到最大重试次数，请求失败: %w", lastErr)
}

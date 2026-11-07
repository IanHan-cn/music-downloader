package music

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	lrclib "github.com/IanHan-cn/music-downloader/LRCLIB"
)

const maxFileSize = 1024 * 1024 * 100

func (p *Processor) Process(ctx context.Context, f FileInfo) (string, error) {
	outputPath := f.Title + "-" + f.Artist + ".flac"
	flacData, err := downloadFile(ctx, downloadClient, f.FileURL)
	if err != nil {
		return "", fmt.Errorf("FileURL 下载失败: %w", err)
	}

	var lyrics string
	if os.Getenv("SCRAPE_LYRICS") == "true" {
		lyrics, _ = downloadLyrics(ctx, f)
	}

	flacFile, err := parseFlacFile(bytes.NewReader(flacData), f, lyrics)
	if err != nil {
		return "", err
	}
	flacObject := flacFile.Marshal()

	coverURL := strings.TrimSpace(f.CoverURL)
	coverURL = strings.Split(coverURL, "?")[0]
	if coverURL != "" {
		slog.Debug("开始下载封面...")
		imgData, err := downloadFile(ctx, downloadClient, coverURL)
		if err != nil {
			slog.Warn("CoverURL 下载失败", "error", err, "TrackName", f.Title, "ArtistName", f.Artist, "AlbumName", f.Album)
		}
		if imgData != nil {
			slog.Debug("下载封面成功！")
			flacFileWithCover, err := addCoverToFlacFile(flacFile, imgData)
			if err != nil {
				return "", err
			}
			flacObject = flacFileWithCover.Marshal()
		}
	}

	downloadURL, err := p.uploader.Upload(context.Background(), outputPath, flacObject)
	if err != nil {
		return "", err
	}
	// 安排删除临时文件
	if err := Task(outputPath); err != nil {
		return "", err
	}
	slog.Info("FLAC 标签与封面修改成功！")
	return downloadURL, nil
}

func downloadLyrics(ctx context.Context, fInfo FileInfo) (string, error) {
	info := lrclib.RequestInfo{
		Album:    fInfo.Album,
		Artist:   fInfo.Artist,
		Duration: fInfo.Duration,
		Title:    fInfo.Title,
	}
	lrcQ := lrclib.NewLyricsRequest(apiClient, &info)
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	lrc, err := lrcQ.GetLyricsWithRetry(ctx)
	if err != nil {
		slog.Error(" http请求获取歌词失败", "error", err)
		return "", nil
	}
	if lrc.SyncedLyrics != "" {
		return lrc.SyncedLyrics, nil
	}
	if lrc.PlainLyrics != "" {
		return lrc.PlainLyrics, nil
	}
	if lrc.Instrumental {
		return "纯音乐", nil
	}
	slog.Info("未找到歌词", "Album", fInfo.Album, "Artist", fInfo.Artist, "Title", fInfo.Title)
	return "", nil
}

func downloadFile(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	slog.Debug("开始下载文件...", "url", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("请求失败或超时", "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w 状态码: %d", ErrResponseCode, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("读取响应体失败", "error", err)
		return nil, err
	}
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("文件体积过大，最大允许为 %d字节,收到%d字节的文件。", maxFileSize, len(data))
	}
	return data, nil
}

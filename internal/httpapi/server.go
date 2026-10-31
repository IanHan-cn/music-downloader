package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/IanHan-cn/music-downloader/internal/music"
)

type Processor interface {
	Process(ctx context.Context, file music.FileInfo) (string, error)
}

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

var unsafeCharsRe = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]|\.\.`)

func NewHandler(processor Processor, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, response{Code: 200, Message: "ok"})
	})
	mux.HandleFunc("POST /modify/flac", AuthMiddleware(token, func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var file music.FileInfo
		if err := json.NewDecoder(r.Body).Decode(&file); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeJSON(w, http.StatusRequestEntityTooLarge, response{Code: 413, Message: "Request body too large"})
				return
			}
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "Invalid request body"})
			return
		}

		file.Title = sanitizeFileName(file.Title)
		file.Artist = sanitizeFileName(file.Artist)
		file.Album = sanitizeFileName(file.Album)

		url, err := processor.Process(r.Context(), file)
		if err != nil {
			slog.Error("处理请求时出错", "error", err)
			if errors.Is(err, music.ErrResponseCode) {
				writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: fmt.Sprintf("请求错误: %s", err)})
				return
			}
			writeJSON(w, http.StatusInternalServerError, response{Code: 500, Message: "Internal Server Error "})
			return
		}

		writeJSON(w, http.StatusAccepted, response{
			Code:    202,
			Message: "FLAC file modified successfully",
			Data:    map[string]string{"file_url": url},
		})
	}))
	return mux
}

func AuthMiddleware(expectedToken string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if strings.TrimSpace(expectedToken) != "" && token != "Bearer "+expectedToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	safeName := unsafeCharsRe.ReplaceAllString(name, "_")
	safeName = strings.Trim(safeName, " .")
	if safeName == "" {
		return "unknown"
	}
	return safeName
}

func writeJSON(w http.ResponseWriter, status int, value response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("写入 JSON 响应失败", "error", err)
	}
}

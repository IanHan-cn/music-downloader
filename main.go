package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/IanHan-cn/music-downloader/internal/httpapi"
	"github.com/IanHan-cn/music-downloader/internal/music"
	"github.com/IanHan-cn/music-downloader/r2"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("加载 .env 失败", "error", err)
	}
	initLog()

	s3Client, err := r2.NewS3Client()
	if err != nil {
		os.Exit(1)
	}

	store := r2.NewStore(s3Client, os.Getenv("R2BucketName"))
	processor := music.NewProcessor(store)
	server := &http.Server{
		Addr:              ":8080",
		Handler:           httpapi.NewHandler(processor, os.Getenv("HEADERS_TOKEN")),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	slog.Info("启动服务...")
	slog.Info("Server is running on http://localhost:8080...")
	if err := server.ListenAndServe(); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}

func initLog() {
	logLevel := slog.LevelInfo
	if os.Getenv("DEBUG") == "true" {
		logLevel = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true, // 开启源码文件和行号记录

	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}

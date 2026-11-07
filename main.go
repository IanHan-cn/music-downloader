package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	// 任务客户端与进程同生命周期，只创建一次，退出时统一关闭 Redis 连接池。
	scheduler := music.NewTaskScheduler()
	processor := music.NewProcessor(store, scheduler)
	server := &http.Server{
		Addr:              ":8080",
		Handler:           httpapi.NewHandler(processor, os.Getenv("HEADERS_TOKEN")),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// 服务错误与退出信号在主流程汇合，避免关闭逻辑与信号处理并发执行。
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("启动服务...")
	slog.Info("Server is running on http://localhost:8080...")
	// 只有在途请求全部结束后才能关闭任务客户端，否则处理中的请求会用到已关闭的连接池。
	shutdownCompleted := true
	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed to start", "error", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := server.Shutdown(shutdownCtx)
		cancel()
		if err != nil {
			slog.Error("关闭 HTTP 服务失败", "error", err)
			shutdownCompleted = false
		}
		<-serverErr
	}
	if shutdownCompleted {
		if err := scheduler.Close(); err != nil {
			slog.Warn("关闭任务客户端失败", "error", err)
		}
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

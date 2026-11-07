package main

import (
	"log/slog"
	"os"

	"github.com/IanHan-cn/music-downloader/r2"
	"github.com/IanHan-cn/music-downloader/task"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("加载 .env 失败", "error", err)
	}
	redisConnOpt := asynq.RedisClientOpt{
		Addr:     os.Getenv("REDIS_URL"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       1,
	}

	s3Client, err := r2.NewS3Client()
	if err != nil {
		slog.Error("创建 S3 客户端失败", "error", err)
		os.Exit(1)
	}
	store := r2.NewStore(s3Client, os.Getenv("R2BucketName"))

	srv := asynq.NewServer(
		redisConnOpt,
		asynq.Config{Concurrency: 10},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(task.TypeS3DeleteByKey, task.NewDeleteHandler(store).Handle)

	// Run 内部监听退出信号并优雅关闭，会一并释放 Server 独占的 Redis 连接池。
	if err := srv.Run(mux); err != nil {
		slog.Error("任务服务运行失败", "error", err)
		os.Exit(1)
	}
}

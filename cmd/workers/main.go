package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/IanHan-cn/music-downloader/task"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("加载 .env 失败", "error", err)
	}
	redisConnOpt := asynq.RedisClientOpt{
		Addr: os.Getenv("REDIS_URL"), // Redis 服务器地址
		// Omit if no password is required
		Password: os.Getenv("REDIS_PASSWORD"),
		// Use a dedicated db number for asynq.
		// By default, Redis offers 16 databases (0..15)
		DB: 1,
	}
	srv := asynq.NewServer(
		redisConnOpt,
		asynq.Config{Concurrency: 10},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(task.TypeS3DeleteByKey, task.HandleDeleteTask)

	if err := srv.Run(mux); err != nil {
		log.Fatal(err)
	}
}

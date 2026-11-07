package music

import (
	"log/slog"
	"os"
	"time"

	"github.com/IanHan-cn/music-downloader/task"
	"github.com/hibiken/asynq"
)

type DeleteTaskPayload struct {
	// S3 存储桶中的对象键
	Key string
}

func Task(key string) error {
	redisConnOpt := asynq.RedisClientOpt{
		Addr: os.Getenv("REDIS_URL"), // Redis 服务器地址
		// Omit if no password is required
		Password: os.Getenv("REDIS_PASSWORD"),
		// Use a dedicated db number for asynq.
		// By default, Redis offers 16 databases (0..15)
		DB: 1,
	}
	client := asynq.NewClient(redisConnOpt)
	// todo: client 需要关闭么？
	task, err := task.NewDeleteTask(key)
	if err != nil {
		return err
	}

	lifeTime, err := time.ParseDuration(os.Getenv("PRESIGN_LIFETIME"))
	if err != nil {
		lifeTime = 60 * time.Second
	}
	info, err := client.Enqueue(task, asynq.ProcessIn(lifeTime))
	if err != nil {
		slog.Error("创建任务失败", "error", err)
		return err
	}
	slog.Debug("创建任务成功", "id", info.ID)
	return nil
}

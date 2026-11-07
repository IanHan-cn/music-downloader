package music

import (
	"log/slog"
	"os"
	"time"

	"github.com/IanHan-cn/music-downloader/task"
	"github.com/hibiken/asynq"
)

// TaskScheduler 持有进程级复用的 asynq 客户端，避免每次请求都新建 Redis 连接池。
type TaskScheduler struct {
	client *asynq.Client
}

// NewTaskScheduler 创建任务调度器，调用方需在进程退出前调用 Close 释放连接。
func NewTaskScheduler() *TaskScheduler {
	redisConnOpt := asynq.RedisClientOpt{
		Addr:     os.Getenv("REDIS_URL"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       1,
	}
	return &TaskScheduler{client: asynq.NewClient(redisConnOpt)}
}

// ScheduleDelete 在下载链接过期后安排删除临时文件。
func (s *TaskScheduler) ScheduleDelete(key string) error {
	t, err := task.NewDeleteTask(key)
	if err != nil {
		return err
	}

	lifeTime, err := time.ParseDuration(os.Getenv("PRESIGN_LIFETIME"))
	if err != nil {
		lifeTime = 60 * time.Second
	}
	info, err := s.client.Enqueue(t, asynq.ProcessIn(lifeTime))
	if err != nil {
		slog.Error("创建任务失败", "error", err)
		return err
	}
	slog.Debug("创建任务成功", "id", info.ID)
	return nil
}

// Close 关闭客户端独占的 Redis 连接池。
func (s *TaskScheduler) Close() error {
	return s.client.Close()
}

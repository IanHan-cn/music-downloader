package task

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/IanHan-cn/music-downloader/r2"
	"github.com/hibiken/asynq"
)

// A list of task types.
const (
	TypeS3DeleteByKey = "s3:delete"
)

type DeleteTaskPayload struct {
	// S3 存储桶中的对象键
	Key string
}

func NewDeleteTask(key string) (*asynq.Task, error) {
	payload, err := json.Marshal(DeleteTaskPayload{
		Key: key,
	})
	if err != nil {
		slog.Error("序列化任务失败", "error", err)
		return nil, err
	}
	return asynq.NewTask(TypeS3DeleteByKey, payload), nil
}
func HandleDeleteTask(ctx context.Context, t *asynq.Task) error {
	var p DeleteTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		slog.Error("反序列化任务失败", "error", err)
		return err
	}

	s3Client, err := r2.NewS3Client()
	if err != nil {
		slog.Error("创建 S3 客户端失败", "error", err)
		return err
	}
	store := r2.NewStore(s3Client, os.Getenv("R2BucketName"))
	if err := store.Delete(ctx, p.Key); err != nil {
		slog.Error("删除文件失败", "error", err)
		return err
	}
	slog.Info("删除文件成功", "key", p.Key)
	return nil
}

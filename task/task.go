package task

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/IanHan-cn/music-downloader/r2"
	"github.com/hibiken/asynq"
)

// 任务类型列表
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

// DeleteHandler 复用进程级的 S3 客户端，避免每个任务都重建 HTTP 连接池。
type DeleteHandler struct {
	store *r2.Store
}

func NewDeleteHandler(store *r2.Store) *DeleteHandler {
	return &DeleteHandler{store: store}
}

func (h *DeleteHandler) Handle(ctx context.Context, t *asynq.Task) error {
	var p DeleteTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		slog.Error("反序列化任务失败", "error", err)
		return err
	}

	if err := h.store.Delete(ctx, p.Key); err != nil {
		slog.Error("删除文件失败", "error", err)
		return err
	}
	slog.Info("删除文件成功", "key", p.Key)
	return nil
}

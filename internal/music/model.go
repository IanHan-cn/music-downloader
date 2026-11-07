package music

import (
	"context"
	"errors"
)

type FileInfo struct {
	FileURL  string `json:"file_url"`
	CoverURL string `json:"cover_url"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int    `json:"duration"`
}

var ErrResponseCode = errors.New("请求返回异常")

type Uploader interface {
	Upload(ctx context.Context, key string, data []byte) (string, error)
}

// DeleteScheduler 负责安排临时文件的异步删除。
type DeleteScheduler interface {
	ScheduleDelete(key string) error
}

type Processor struct {
	uploader  Uploader
	scheduler DeleteScheduler
}

func NewProcessor(uploader Uploader, scheduler DeleteScheduler) *Processor {
	return &Processor{uploader: uploader, scheduler: scheduler}
}

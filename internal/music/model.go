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

type Processor struct {
	uploader Uploader
}

func NewProcessor(uploader Uploader) *Processor {
	return &Processor{uploader: uploader}
}

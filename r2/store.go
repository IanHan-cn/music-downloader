package r2

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Store struct {
	client *s3.Client
	bucket string
}

func NewStore(client *s3.Client, bucket string) *Store {
	return &Store{client: client, bucket: bucket}
}

func (store *Store) Upload(ctx context.Context, key string, data []byte) (string, error) {
	bucket := store.bucket

	bucketBasics := NewBucketBasics(store.client)
	if err := bucketBasics.UploadLargeObject(ctx, bucket, key, data); err != nil {
		return "", err
	}

	presigner := Presigner{PresignClient: s3.NewPresignClient(store.client)}
	// 从环境变量中获取链存活时间
	durationStr := os.Getenv("PRESIGN_LIFETIME")
	lifetime, err := time.ParseDuration(durationStr)
	if err != nil {
		lifetime = 60 * time.Second // 默认值
	}
	downloadURL, err := presigner.GetObject(ctx, bucket, key, int64(lifetime.Seconds()))
	if err != nil {
		return "", err
	}
	return downloadURL.URL, nil
}
func (store *Store) Delete(ctx context.Context, key string) error {
	bucket := store.bucket
	bucketBasics := NewBucketBasics(store.client)
	keys := []string{key}
	return bucketBasics.DeleteObjects(ctx, bucket, keys)
}

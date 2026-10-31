package r2

import (
	"context"

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
	downloadURL, err := presigner.GetObject(ctx, bucket, key, 60*10)
	if err != nil {
		return "", err
	}
	return downloadURL.URL, nil
}

package r2

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func NewS3Client() (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("auto"),
		config.WithBaseEndpoint(os.Getenv("R2_ENDPOINT")),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv("R2_ACCESS_KEY_ID"),
			os.Getenv("R2_SECRET_ACCESS_KEY"),
			"",
		)),
	)
	if err != nil {
		slog.Error("初始化 AWS SDK for Go v2 失败", "error", err)
		return nil, err
	}

	cfg.HTTPClient = awshttp.NewBuildableClient().
		WithTimeout(120 * time.Second).
		WithDialerOptions(func(d *net.Dialer) {
			d.Timeout = 10 * time.Second
			d.KeepAlive = 30 * time.Second
		}).
		WithTransportOptions(func(tr *http.Transport) {
			tr.MaxIdleConns = 1000
			tr.MaxIdleConnsPerHost = 100
			tr.MaxConnsPerHost = 100
			tr.TLSHandshakeTimeout = 10 * time.Second
			tr.ResponseHeaderTimeout = 30 * time.Second
		})

	return s3.NewFromConfig(cfg), nil
}

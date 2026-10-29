package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/IanHan-cn/music-downloader/r2"
	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
	"github.com/joho/godotenv"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type FileInfo struct {
	FileURL  string `json:"file_url"`
	CoverURL string `json:"cover_url"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
}

// 定义返回的 JSON 数据结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

var (
	ErrResponseCode = errors.New("请求返回异常")
)

// 定义不安全字符的正则表达式：
// 1. 匹配 Windows/Linux 非法文件名字符: < > : " / \ | ? * 以及控制字符 \x00-\x1F
// 2. 匹配路径穿越符: ..
var unsafeCharsRe = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]|\.\.`)

func sanitizeFileName(name string) string {
	// 将所有非法字符和路径穿越符替换为下划线
	name = strings.TrimSpace(name) // 去除首尾空格
	safeName := unsafeCharsRe.ReplaceAllString(name, "_")

	// 去除首尾的空格和点（某些系统对结尾的点或空格敏感）
	safeName = strings.Trim(safeName, " .")

	// 如果清洗后为空（例如输入全是特殊符号），给一个默认兜底名称
	if safeName == "" {
		return "unknown"
	}

	return safeName
}
func initLog() {
	// 1. 创建 Handler 选项并设置 Level 为 Debug
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	// 2. 创建自定义的 logger（这里以 TextHandler 为例，也可以用 JSONHandler）
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))

	// 3. （可选）将其设置为全局默认 Logger
	slog.SetDefault(logger)
}

func initR2() (*s3.Client, error) {

	// 初始化 AWS SDK for Go v2 连接 Cloudflare R2
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

	s3Client := s3.NewFromConfig(cfg)
	return s3Client, nil
}

func main() {
	godotenv.Load()
	initLog()
	r2Client, err := initR2()
	if err != nil {
		os.Exit(1)
	}

	// 1. 创建一个多路复用器（路由器）
	mux := http.NewServeMux()
	// 2. 注册 POST 接口（接收 JSON 参数）
	mux.HandleFunc("POST /modify/flac", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// 解析请求体中的 JSON
		var f FileInfo
		err := json.NewDecoder(r.Body).Decode(&f)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{Code: 400, Message: "Invalid request body"})
			return
		}
		f.Title = sanitizeFileName(f.Title)
		f.Artist = sanitizeFileName(f.Artist)
		f.Album = sanitizeFileName(f.Album)

		if url, err := run(f, r2Client); err != nil {
			slog.Error("处理请求时出错", "error", err)
			if errors.Is(err, ErrResponseCode) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(Response{Code: 400, Message: fmt.Sprintf("请求错误: %s", err)})

			} else {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(Response{Code: 500, Message: "Internal Server Error "})

			}
			return
		} else {
			// 返回成功响应
			resp := Response{
				Code:    202,
				Message: "FLAC file modified successfully",
				Data: map[string]string{
					"file_url": *url,
				},
			}
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(resp)
		}
	})

	// 3. 启动服务，监听 8080 端口
	slog.Info("启动服务...")
	slog.Info("Server is running on http://localhost:8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		slog.Error("Server failed to start", "error", err)
	}

	// if err := run(); err != nil {
	// 	slog.Error("运行程序时出错", "error", err)
	// 	os.Exit(1)
	// }
}

func run(f FileInfo, r2Client *s3.Client) (*string, error) {
	outputPath := f.Title + "-" + f.Artist + ".flac"
	// 全局复用 client，避免频繁创建和关闭连接
	client := &http.Client{
		Timeout: 30 * time.Second, // 30 秒超时
	}
	flacFile, err := downloadFile(client, f.FileURL)
	if err != nil {
		return nil, fmt.Errorf("FileURL 下载失败: %w", err)
	}

	flacFileParsed, err := parseFlacFile(bytes.NewReader(flacFile), f)
	if err != nil {
		return nil, err
	}
	imgData, err := downloadFile(client, f.CoverURL)
	if err != nil {
		return nil, fmt.Errorf("CoverURL 下载失败: %w", err)
	}

	flacFileWithCover, err := addCoverToFlacFile(flacFileParsed, imgData)
	if err != nil {
		return nil, err
	}
	// 保存到 R2
	flacObject := flacFileWithCover.Marshal()
	var downloadUrl *string
	if downloadUrl, err = putToR2(flacObject, outputPath, r2Client); err != nil {
		return nil, err
	}

	slog.Info("FLAC 标签与封面修改成功！")
	return downloadUrl, nil
}
func putToR2(flacObject []byte, outputPath string, r2Client *s3.Client) (*string, error) {
	r2Bucket := r2.NewBucketBasics(r2Client)

	if err := r2Bucket.UploadLargeObject(
		context.Background(),
		os.Getenv("R2BucketName"),
		outputPath,
		flacObject,
	); err != nil {
		slog.Error("上传文件失败", "error", err)
		return nil, err
	}
	preSignClient := s3.NewPresignClient(r2Client)
	var p r2.Presigner
	p.PresignClient = preSignClient

	downloadUrl, err := p.GetObject(
		context.Background(),
		os.Getenv("R2BucketName"),
		outputPath,
		60*10, // 10分钟
	)
	return &downloadUrl.URL, err
}
func downloadFile(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		slog.Error("请求失败或超时", "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("%w 状态码: %d", ErrResponseCode, resp.StatusCode)
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("读取响应体失败", "error", err)
		return nil, err
	}
	// 文件体积过小的话，直接返回
	if len(data) < 1024 {
		return nil, fmt.Errorf("文件体积过小，可能链接已过期。")
	}

	return data, nil
}
func parseFlacFile(file io.Reader, f FileInfo) (*flac.File, error) {
	// 1. 解析 FLAC 文件
	flacFile, err := flac.ParseBytes(file)
	if err != nil {
		slog.Error("解析 FLAC 文件失败", "error", err)
		return nil, err
	}

	// 2. 修改或添加文本标签（Vorbis Comments）
	var cmt *flacvorbis.MetaDataBlockVorbisComment
	var cmtIdx = -1

	for idx, meta := range flacFile.Meta {
		if meta.Type == flac.VorbisComment {
			cmt, err = flacvorbis.ParseFromMetaDataBlock(*meta)
			if err != nil {
				slog.Error("解析 Vorbis 标签失败", "error", err)
				return nil, err
			}
			cmtIdx = idx
			break
		}
	}

	if cmt == nil {
		cmt = flacvorbis.New()
	}

	// 设置标签字段
	_ = cmt.Add(flacvorbis.FIELD_TITLE, f.Title)
	_ = cmt.Add(flacvorbis.FIELD_ARTIST, f.Artist)
	_ = cmt.Add(flacvorbis.FIELD_ALBUM, f.Album)

	cmtMeta := cmt.Marshal()
	if cmtIdx >= 0 {
		flacFile.Meta[cmtIdx] = &cmtMeta
	} else {
		flacFile.Meta = append([]*flac.MetaDataBlock{&cmtMeta}, flacFile.Meta...)
	}
	return flacFile, nil
}

func addCoverToFlacFile(flacFile *flac.File, imgData []byte) (*flac.File, error) {

	picMeta, err := flacpicture.NewFromImageData(
		flacpicture.PictureTypeFrontCover,
		"Cover",
		imgData,
		"image/jpeg",
	)
	if err != nil {
		slog.Error("创建封面块失败", "error", err)
		return nil, err
	}

	// 【修改点】：直接调用 picMeta.Marshal() 得到 MetaDataBlock，然后取地址放入切片
	pictureMetaBlock := picMeta.Marshal()
	flacFile.Meta = append(flacFile.Meta, &pictureMetaBlock)
	return flacFile, nil
}

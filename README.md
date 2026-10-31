# music-downloader

一个 HTTP 服务：接收歌曲文件和元数据，更新 FLAC 标签、可选写入歌词与封面，然后上传到 Cloudflare R2，并返回限时下载链接。

## 环境要求

- Go 1.27.1 或更高版本（以 `go.mod` 的 Go 版本为准）
- 可访问的 Cloudflare R2 兼容 S3 存储桶及其凭证

## 配置

程序启动时会尝试从当前工作目录加载 `.env` 文件。未找到 `.env` 时会记录警告，并继续使用进程环境变量。

| 环境变量 | 必需 | 说明 |
| --- | --- | --- |
| `R2_ENDPOINT` | 是 | R2 S3 API 端点，例如 `https://<account-id>.r2.cloudflarestorage.com` |
| `R2_ACCESS_KEY_ID` | 是 | R2 API 令牌的 Access Key ID |
| `R2_SECRET_ACCESS_KEY` | 是 | 对应的 Secret Access Key |
| `R2BucketName` | 是 | 上传目标存储桶名称（变量名区分大小写） |
| `HEADERS_TOKEN` | 建议 | API 鉴权令牌。设置后，请求 `/modify/flac` 必须携带 `Authorization: Bearer <令牌>`；留空时不启用鉴权 |
| `SCRAPE_LYRICS` | 否 | 设置为 `true` 时尝试从 LRCLIB 获取歌词；其他值或未设置时跳过 |
| `LRCLIB_APP_ID` | 否 | 发送给 LRCLIB 的 `Lrclib-App-Id` 请求头 |
| `LRCLIB_APP_KEY` | 否 | 发送给 LRCLIB 的 `Lrclib-Api-Key` 请求头 |
| `DEBUG` | 否 | 设置为 `true` 时启用 Debug 日志；默认使用 Info 日志 |

R2 配置是服务启动和上传所必需的。请勿将包含真实访问密钥的 `.env` 文件提交到版本控制。生产环境应设置 `HEADERS_TOKEN`；令牌留空会关闭接口鉴权。

示例 `.env`：

```dotenv
R2_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
R2_ACCESS_KEY_ID=<your-access-key-id>
R2_SECRET_ACCESS_KEY=<your-secret-access-key>
R2BucketName=<your-bucket-name>

HEADERS_TOKEN=<choose-a-long-random-token>
SCRAPE_LYRICS=false
# LRCLIB_APP_ID=<optional-app-id>
# LRCLIB_APP_KEY=<optional-api-key>
DEBUG=false
```

## 本地运行

```bash
go run .
```

服务监听 `:8080`。`/health` 健康检查接口不需要鉴权。

## API

### `GET /health`

用于检查服务是否可用，成功时返回 HTTP 200：

```json
{"code":200,"message":"ok"}
```

### `POST /modify/flac`

请求体为 JSON，最大 1 MiB。`file_url`、`title`、`artist` 和 `album` 应提供有效值；`cover_url` 可留空，`duration` 为歌曲时长（秒）。

```json
{
  "file_url": "https://example.com/song.flac",
  "cover_url": "https://example.com/cover.jpg",
  "title": "Song Title",
  "artist": "Artist One, Artist Two",
  "album": "Album Name",
  "duration": 210
}
```

多个艺术家可用英文逗号分隔，写入 FLAC 时会分别添加为 `ARTIST` 标签。下载的音频和封面文件最大为 100 MiB；封面下载失败时仍会继续处理音频。启用歌词抓取后，服务会尝试添加 LRCLIB 返回的同步歌词、纯文本歌词或纯音乐标记。

如果设置了 `HEADERS_TOKEN`，请求必须带上对应的 Bearer Token：

```bash
curl -X POST http://localhost:8080/modify/flac \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  --data '{
    "file_url":"https://example.com/song.flac",
    "cover_url":"",
    "title":"Song Title",
    "artist":"Artist One, Artist Two",
    "album":"Album Name",
    "duration":210
  }'
```

处理成功时返回 HTTP 202，响应中的 `data.file_url` 是上传对象的预签名下载链接，有效期为 10 分钟。请求参数错误返回 HTTP 400；处理失败返回 HTTP 500。

## Docker

在项目根目录构建镜像：

```bash
docker build -t music-downloader .
```

运行时通过环境变量传入 R2 配置。建议使用安全的密钥管理方式，而不是把密钥写入镜像：

```bash
docker run --rm \
  -p 8080:8080 \
  -e R2_ENDPOINT='https://<account-id>.r2.cloudflarestorage.com' \
  -e R2_ACCESS_KEY_ID='<your-access-key-id>' \
  -e R2_SECRET_ACCESS_KEY='<your-secret-access-key>' \
  -e R2BucketName='<your-bucket-name>' \
  -e HEADERS_TOKEN='<your-token>' \
  music-downloader
```

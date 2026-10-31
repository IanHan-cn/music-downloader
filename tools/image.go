package tools

import (
	"fmt"
	"net/http"
)

// DetectImageMime 通过魔数检测图片真实 MIME 类型
func DetectImageMime(imgData []byte) (string, error) {
	if len(imgData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	// http.DetectContentType 最多只需要前 512 个字节
	detectLen := 512
	if len(imgData) < detectLen {
		detectLen = len(imgData)
	}

	mime := http.DetectContentType(imgData[:detectLen])

	// 验证是否为支持的图片格式
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
		return mime, nil
	default:
		return "", fmt.Errorf("unsupported or unknown image format: %s", mime)
	}
}

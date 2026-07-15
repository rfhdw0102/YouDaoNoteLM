package markitdown

import (
	"context"
	"io"
)

// Client 文件解析客户端接口
type Client interface {
	Convert(filePath string) (string, error)
	ConvertReader(filename string, reader io.Reader) (string, error)
	ConvertFromURL(url string) (string, error)

	// 带 context 的方法，支持超时控制
	ConvertReaderWithContext(ctx context.Context, filename string, reader io.Reader) (string, error)
	ConvertFromURLWithContext(ctx context.Context, url string) (string, error)
}

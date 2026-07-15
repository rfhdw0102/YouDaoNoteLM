package ppt

// 演示文稿导出文件的二进制结果。
type ExportResult struct {
	Filename    string
	ContentType string
	Data        []byte
}

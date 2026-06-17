package utils

import (
	"fmt"
	"os"
	"testing"
)

func TestConvertFileToASRFormat(t *testing.T) {
	inputPath := `C:\xwechat_files\wxid_wiac4zqxj04y22_6721\msg\file\2026-05\标准录音 10.mp3`

	// 检查文件是否存在
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("测试音频文件不存在: %s", inputPath)
	}

	fInfo, _ := os.Stat(inputPath)
	t.Logf("输入文件: %s (%.2f MB)", inputPath, float64(fInfo.Size())/1024/1024)

	// 测试转换
	outputPath, err := ConvertFileToASRFormat(inputPath)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}
	defer os.Remove(outputPath) // 测试后清理

	t.Logf("输出文件: %s", outputPath)

	// 验证输出文件存在
	outInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("输出文件不存在: %v", err)
	}
	t.Logf("输出大小: %.2f MB", float64(outInfo.Size())/1024/1024)

	// 验证是 WAV 格式
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("读取输出文件失败: %v", err)
	}

	// WAV 文件头: RIFF....WAVE
	if len(data) < 12 {
		t.Fatal("输出文件太小，不是有效的 WAV")
	}
	if string(data[0:4]) != "RIFF" {
		t.Fatalf("输出文件不是 RIFF 格式: %s", string(data[0:4]))
	}
	if string(data[8:12]) != "WAVE" {
		t.Fatalf("输出文件不是 WAVE 格式: %s", string(data[8:12]))
	}

	fmt.Println("✅ WAV 格式验证通过")
	fmt.Println("✅ 音频转换测试通过")
}

func TestConvertBytesToASRFormat(t *testing.T) {
	inputPath := `C:\xwechat_files\wxid_wiac4zqxj04y22_6721\msg\file\2026-05\标准录音 10.mp3`

	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("测试音频文件不存在: %s", inputPath)
	}

	// 读取文件
	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	t.Logf("读取文件成功: %d bytes", len(data))

	// 转换
	wavData, err := ConvertBytesToASRFormat(data, ".mp3")
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}
	t.Logf("转换成功: %d bytes", len(wavData))

	// 验证 WAV 头
	if len(wavData) < 12 {
		t.Fatal("输出太小")
	}
	if string(wavData[0:4]) != "RIFF" || string(wavData[8:12]) != "WAVE" {
		t.Fatal("输出不是有效的 WAV 格式")
	}

	fmt.Println("✅ Bytes 转换测试通过")
}

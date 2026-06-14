package utils

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/hajimehoshi/go-mp3"
)

const (
	// ASRTargetSampleRate 阿里云 ASR 要求的采样率
	ASRTargetSampleRate = 16000
	// ASRChannels 声道数（单声道）
	ASRChannels = 1
	// ASRBitDepth 位深度
	ASRBitDepth = 16
)

// ConvertFileToASRFormat 将音频文件转换为阿里云 ASR 兼容格式（16kHz 单声道 WAV）
// 输入: 文件路径
// 输出: 转换后的 WAV 文件路径
func ConvertFileToASRFormat(inputPath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))

	// 读取并解码音频
	pcmData, sampleRate, channels, err := decodeAudioFile(inputPath, ext)
	if err != nil {
		return "", fmt.Errorf("解码音频失败: %w", err)
	}

	// 如果已经是目标格式，直接写入 WAV
	if sampleRate == ASRTargetSampleRate && channels == ASRChannels {
		return writeWAV(inputPath, pcmData, sampleRate, channels)
	}

	// 转换为单声道
	if channels > 1 {
		pcmData = convertToMono(pcmData, channels)
	}

	// 重采样到目标采样率
	if sampleRate != ASRTargetSampleRate {
		pcmData = resample(pcmData, sampleRate, ASRTargetSampleRate)
	}

	// 写入 WAV 文件
	return writeWAV(inputPath, pcmData, ASRTargetSampleRate, ASRChannels)
}

// ConvertBytesToASRFormat 将音频字节数据转换为 ASR 兼容格式
func ConvertBytesToASRFormat(data []byte, originalExt string) ([]byte, error) {
	ext := strings.ToLower(originalExt)

	// 解码
	pcmData, sampleRate, channels, err := decodeAudioBytes(data, ext)
	if err != nil {
		return nil, fmt.Errorf("解码音频失败: %w", err)
	}

	// 转换单声道
	if channels > 1 {
		pcmData = convertToMono(pcmData, channels)
	}

	// 重采样
	if sampleRate != ASRTargetSampleRate {
		pcmData = resample(pcmData, sampleRate, ASRTargetSampleRate)
	}

	// 编码为 WAV 字节
	return encodeToWAVBytes(pcmData, ASRTargetSampleRate, ASRChannels)
}

// decodeAudioFile 根据扩展名解码音频文件
func decodeAudioFile(path, ext string) ([]int, int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, err
	}
	return decodeAudioBytes(data, ext)
}

// decodeAudioBytes 根据格式解码音频字节
func decodeAudioBytes(data []byte, ext string) ([]int, int, int, error) {
	switch ext {
	case ".mp3":
		return decodeMP3(data)
	case ".wav":
		return decodeWAV(data)
	default:
		return nil, 0, 0, fmt.Errorf("不支持的音频格式: %s", ext)
	}
}

// decodeMP3 解码 MP3 数据
func decodeMP3(data []byte) ([]int, int, int, error) {
	decoder, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("MP3 解码器创建失败: %w", err)
	}

	sampleRate := decoder.SampleRate()

	// 读取所有 PCM 数据（16bit signed little-endian）
	var pcmBytes []byte
	buf := make([]byte, 4096)
	for {
		n, err := decoder.Read(buf)
		if n > 0 {
			pcmBytes = append(pcmBytes, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, 0, fmt.Errorf("MP3 读取失败: %w", err)
		}
	}

	// MP3 解码输出是 16bit stereo (2 channels)
	channels := 2
	// 转换为 int 切片
	pcmData := make([]int, len(pcmBytes)/2)
	for i := 0; i < len(pcmData); i++ {
		// little-endian 16bit signed
		sample := int16(pcmBytes[i*2]) | int16(pcmBytes[i*2+1])<<8
		pcmData[i] = int(sample)
	}

	return pcmData, sampleRate, channels, nil
}

// decodeWAV 解码 WAV 数据
func decodeWAV(data []byte) ([]int, int, int, error) {
	decoder := wav.NewDecoder(bytes.NewReader(data))
	if !decoder.IsValidFile() {
		return nil, 0, 0, fmt.Errorf("无效的 WAV 文件")
	}

	buf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: int(decoder.NumChans),
			SampleRate:  int(decoder.SampleRate),
		},
	}
	// 读取所有采样
	for {
		chunk, err := decoder.FullPCMBuffer()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, 0, fmt.Errorf("WAV 读取失败: %w", err)
		}
		buf.Data = append(buf.Data, chunk.Data...)
		break // FullPCMBuffer 一次读完
	}

	if len(buf.Data) == 0 {
		return nil, 0, 0, fmt.Errorf("WAV 文件无音频数据")
	}

	return buf.Data, int(decoder.SampleRate), int(decoder.NumChans), nil
}

// convertToMono 多声道转单声道（取平均值）
func convertToMono(data []int, channels int) []int {
	if channels == 1 {
		return data
	}

	monoLen := len(data) / channels
	mono := make([]int, monoLen)
	for i := 0; i < monoLen; i++ {
		sum := 0
		for ch := 0; ch < channels; ch++ {
			sum += data[i*channels+ch]
		}
		mono[i] = sum / channels
	}
	return mono
}

// resample 线性插值重采样
func resample(data []int, fromRate, toRate int) []int {
	if fromRate == toRate {
		return data
	}

	ratio := float64(fromRate) / float64(toRate)
	newLen := int(float64(len(data)) / ratio)
	result := make([]int, newLen)

	for i := 0; i < newLen; i++ {
		srcPos := float64(i) * ratio
		srcIdx := int(srcPos)
		frac := srcPos - float64(srcIdx)

		if srcIdx+1 < len(data) {
			// 线性插值
			result[i] = int(math.Round(float64(data[srcIdx])*(1-frac) + float64(data[srcIdx+1])*frac))
		} else if srcIdx < len(data) {
			result[i] = data[srcIdx]
		}
	}

	return result
}

// writeWAV 将 PCM 数据写入 WAV 文件
func writeWAV(originalPath string, data []int, sampleRate, channels int) (string, error) {
	ext := filepath.Ext(originalPath)
	outputPath := originalPath[:len(originalPath)-len(ext)] + "_16k.wav"

	f, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer f.Close()

	encoder := wav.NewEncoder(f, sampleRate, ASRBitDepth, channels, 1)

	// 转换为 audio.IntBuffer
	buf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: channels,
			SampleRate:  sampleRate,
		},
		Data: data,
	}

	if err := encoder.Write(buf); err != nil {
		return "", fmt.Errorf("写入 WAV 失败: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("关闭 WAV 编码器失败: %w", err)
	}

	return outputPath, nil
}

// encodeToWAVBytes 将 PCM 数据编码为 WAV 字节
func encodeToWAVBytes(data []int, sampleRate, channels int) ([]byte, error) {
	// WAV 编码器需要 Seek 支持，使用临时文件
	tmpFile, err := os.CreateTemp("", "asr-wav-*.wav")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	encoder := wav.NewEncoder(tmpFile, sampleRate, ASRBitDepth, channels, 1)

	intBuf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: channels,
			SampleRate:  sampleRate,
		},
		Data: data,
	}

	if err := encoder.Write(intBuf); err != nil {
		return nil, fmt.Errorf("编码 WAV 失败: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("关闭 WAV 编码器失败: %w", err)
	}

	// 读取文件内容
	result, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("读取 WAV 文件失败: %w", err)
	}

	return result, nil
}

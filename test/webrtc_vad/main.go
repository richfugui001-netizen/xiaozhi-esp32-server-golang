package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"

	"xiaozhi-esp32-server-golang/internal/domain/audio"
	"xiaozhi-esp32-server-golang/internal/domain/vad/webrtc_vad"
)

func genFloat32Empty(sampleRate int, durationMs int, channels int, count int) [][]float32 {
	// 计算样本数
	numSamples := int(float64(sampleRate) * float64(durationMs) / 1000.0)
	// 创建静音缓冲区
	var buf bytes.Buffer
	// 32位浮点静音值为0.0
	for i := 0; i < numSamples*channels; i++ {
		binary.Write(&buf, binary.LittleEndian, float32(0.0))
	}
	//将数据转换为float32
	float32Data := make([]float32, numSamples*channels)
	for i := 0; i < numSamples*channels; i++ {
		float32Data[i] = float32(buf.Bytes()[i])
	}
	result := make([][]float32, 0)
	for i := 0; i < count; i++ {
		result = append(result, float32Data)
	}
	return result
}

func genOpusFloat32Empty(sampleRate int, durationMs int, channels int, count int) [][]float32 {
	// 计算样本数
	numSamples := int(float64(sampleRate) * float64(durationMs) / 1000.0)
	// 创建静音缓冲区
	var buf bytes.Buffer
	// 32位浮点静音值为0.0
	for i := 0; i < numSamples*channels; i++ {
		binary.Write(&buf, binary.LittleEndian, float32(0.0))
	}
	//将数据转换为float32
	float32Data := make([]float32, numSamples*channels)
	for i := 0; i < numSamples*channels; i++ {
		float32Data[i] = float32(buf.Bytes()[i])
	}

	audioProcesser, err := audio.GetAudioProcesser(sampleRate, channels, 20)
	if err != nil {
		fmt.Printf("获取解码器失败: %v", err)
		return nil
	}

	pcmFrame := make([]float32, numSamples)

	opusFrame := make([]byte, 50)
	n, err := audioProcesser.DecoderFloat32(opusFrame, pcmFrame)
	if err != nil {
		fmt.Printf("解码失败: %v", err)
		return nil
	}

	result := make([][]float32, 0)
	for i := 0; i < count; i++ {
		tmp := make([]float32, n)
		copy(tmp, pcmFrame[:n])
		result = append(result, tmp)
	}
	return result
}

func main() {
	// 检查命令行参数
	if len(os.Args) != 2 {
		log.Fatalf("用法: %s <wav文件路径>", os.Args[0])
	}

	wavFilePath := os.Args[1]

	// 读取WAV文件
	wavFile, err := os.Open(wavFilePath)
	if err != nil {
		log.Fatalf("无法打开WAV文件: %v", err)
	}
	defer wavFile.Close()

	// 读取整个文件内容
	wavData, err := io.ReadAll(wavFile)
	if err != nil {
		log.Fatalf("无法读取WAV文件: %v", err)
	}

	fmt.Printf("成功读取WAV文件: %s (%d 字节)\n", wavFilePath, len(wavData))

	// 调用 Wav2Pcm 函数转换WAV数据为PCM数据
	// 使用WebRTC VAD支持的标准参数：16000Hz采样率，单声道
	sampleRate := 16000
	channels := 1

	pcmFloat32, pcmBytes, err := Wav2Pcm(wavData, sampleRate, channels)
	if err != nil {
		log.Fatalf("WAV转PCM失败: %v", err)
	}

	_ = pcmFloat32
	_ = pcmBytes

	fmt.Printf("成功转换为PCM数据，共 %d 帧\n", len(pcmFloat32))

	// 创建WebRTC VAD实例
	vadImpl, err := webrtc_vad.NewWebRTCVADWithConfig(sampleRate, 2) // 模式3：高敏感度
	if err != nil {
		log.Fatalf("创建WebRTC VAD失败: %v", err)
	}
	defer vadImpl.Close()

	fmt.Println("WebRTC VAD创建成功，开始测试...")

	// 直接测试VAD是否能正常工作
	if len(pcmFloat32) == 0 {
		log.Fatalf("没有PCM数据可供处理")
	}

	fmt.Println("开始进行语音活动检测...")

	// 对每一帧PCM数据进行VAD检测
	speechFrames := 0
	totalFrames := len(pcmFloat32)

	detectVoice := func(voiceFloat32 [][]float32) {
		for i, pcmFrame := range voiceFloat32 {
			//进行pcmFrame做md5
			byteData := float32ToByte(pcmFrame)
			md5 := md5.Sum(byteData)
			// 进行VAD检测
			isVoice, err := vadImpl.IsVADExt(pcmFrame, sampleRate, 320)
			if err != nil {
				log.Printf("第%d帧VAD检测失败: %v", i+1, err)
				// 如果是第一帧就失败，说明VAD未正确初始化
				if i == 0 {
					log.Fatalf("VAD初始化失败，请检查WebRTC VAD配置")
				}
				continue
			}

			if isVoice {
				speechFrames++
				fmt.Printf("第%d帧: 检测到语音活动, md5: %x\n", i+1, md5)
			} else {
				fmt.Printf("第%d帧: 无语音活动, md5: %x\n", i+1, md5)
			}
		}
		// 输出统计结果
		speechPercentage := float64(speechFrames) / float64(totalFrames) * 100
		fmt.Printf("\n=== VAD检测结果统计 ===\n")
		fmt.Printf("总帧数: %d\n", totalFrames)
		fmt.Printf("语音帧数: %d\n", speechFrames)
		fmt.Printf("语音活动比例: %.2f%%\n", speechPercentage)

		if speechFrames > 0 {
			fmt.Println("结论: 检测到语音活动")
		} else {
			fmt.Println("结论: 未检测到语音活动")
		}
	}

	pcmData, err := ioutil.ReadFile("/tmp/temp.pcm")
	if err != nil {
		log.Fatalf("无法读取PCM文件: %v", err)
	}
	//将pcmData转换为float32，按20ms分帧（320个样本/帧）
	frameSize := 320                 // 16000Hz * 0.02s = 320 samples per 20ms frame
	totalSamples := len(pcmData) / 4 // 每个float32占4字节
	pcmFloat32 = make([][]float32, 0)

	for frameStart := 0; frameStart < totalSamples; frameStart += frameSize {
		frameEnd := frameStart + frameSize
		if frameEnd > totalSamples {
			frameEnd = totalSamples // 处理最后一帧可能不足320样本的情况
		}

		frame := make([]float32, frameEnd-frameStart)
		for i := frameStart; i < frameEnd; i++ {
			byteOffset := i * 4
			frame[i-frameStart] = math.Float32frombits(binary.LittleEndian.Uint32(pcmData[byteOffset : byteOffset+4]))
		}
		pcmFloat32 = append(pcmFloat32, frame)
	}

	//emptyFrame := make([]float32, 50)
	//pcmFloat32 = [][]float32{emptyFrame}
	//pcmFloat32 = genFloat32Empty(sampleRate, 20, channels, 30)
	detectVoice(pcmFloat32)
}

func float32ToByte(pcmFrame []float32) []byte {
	byteData := make([]byte, len(pcmFrame)*4)
	for i, sample := range pcmFrame {
		binary.LittleEndian.PutUint32(byteData[i*4:], math.Float32bits(sample))
	}
	return byteData
}

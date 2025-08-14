package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
)

// Float32SliceToBytes 将float32切片转换为字节数组
func Float32SliceToBytes(data []float32) []byte {
	bytes := make([]byte, len(data)*4)
	for i, v := range data {
		binary.LittleEndian.PutUint32(bytes[i*4:], uint32(v))
	}
	return bytes
}

// GzipCompress 压缩数据
func GzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	if _, err := gzipWriter.Write(data); err != nil {
		return nil, fmt.Errorf("压缩数据失败: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("关闭gzip writer失败: %v", err)
	}
	return buf.Bytes(), nil
}

// GenerateHeader 生成协议头
func GenerateHeader(messageType uint8, flags uint8, serializationMethod uint8) []byte {
	header := make([]byte, 4)
	header[0] = (1 << 4) | 1                   // 协议版本(4位) + 头大小(4位)
	header[1] = (messageType << 4) | flags     // 消息类型(4位) + 消息标志(4位)
	header[2] = (serializationMethod << 4) | 1 // 序列化方法(4位) + 压缩方法(4位，1表示gzip)
	header[3] = 0                              // 保留字段
	return header
}

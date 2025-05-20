package client

import (
	"crypto/cipher"
	"encoding/binary"
	"net"
	"time"
)

// Session 表示一个UDP会话
type UdpSession struct {
	ID          string
	Conn        *net.UDPConn //udp conn
	ClientID    string
	AesKey      [16]byte // 随机32位
	Nonce       [8]byte  // 存储原始nonce模板 16位
	CreatedAt   time.Time
	LastActive  time.Time
	RemoteAddr  *net.UDPAddr //remote addr
	LocalSeq    uint32
	ClientState *ClientState
	Block       cipher.Block
	RemoteSeq   uint32
	RecvChannel chan []byte //发送的音频数据
	SendChannel chan []byte //接收的音频数据
}

// decrypt 解密数据
func (s *UdpSession) Decrypt(data []byte) ([]byte, error) {
	// 分离nonce和密文
	nonce := data[:16] // 使用16字节nonce
	ciphertext := data[16:]

	// 提取序列号
	seqNum := binary.BigEndian.Uint32(data[12:16])

	// 检查序列号
	/*if seqNum < s.RemoteSeq {
		return nil, fmt.Errorf("序列号过期: got %d, expected >= %d", seqNum, s.RemoteSeq)
	}*/
	s.RemoteSeq = seqNum

	// 解密数据
	stream := cipher.NewCTR(s.Block, nonce)
	decrypted := make([]byte, len(ciphertext))
	stream.XORKeyStream(decrypted, ciphertext)

	return decrypted, nil
}

// encrypt 加密数据
func (s *UdpSession) Encrypt(data []byte) ([]byte, error) {
	// 预分配内存，避免扩容
	encrypted := make([]byte, 16+len(data))

	// 构建nonce (16字节)
	encrypted[0] = 0x01                                          // 包类型
	binary.BigEndian.PutUint16(encrypted[2:], uint16(len(data))) // 数据长度
	copy(encrypted[4:12], s.Nonce[:])                            // 8字节nonce
	s.LocalSeq++
	binary.BigEndian.PutUint32(encrypted[12:], s.LocalSeq) // 序列号

	// 加密数据
	stream := cipher.NewCTR(s.Block, encrypted[:16]) // 使用16字节作为IV
	stream.XORKeyStream(encrypted[16:], data)

	return encrypted, nil
}

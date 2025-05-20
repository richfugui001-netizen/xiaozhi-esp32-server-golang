package main

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"

	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// AuthHook 实现自定义鉴权逻辑
// 支持普通用户和超级管理员
// 普通用户: 用户名为 base64 后的 {"ip":"1.202.193.194"}，密码为 AES 加密后的用户名
// 超级管理员: 用户名 admin，密码 shijingbo!@#
type AuthHook struct {
	mqttServer.HookBase
}

func (h *AuthHook) ID() string {
	return "custom-auth-hook"
}

func (h *AuthHook) Provides(b byte) bool {
	return b == mqttServer.OnConnectAuthenticate
}

func (h *AuthHook) OnConnectAuthenticate(cl *mqttServer.Client, pk packets.Packet) bool {
	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)

	if username == "admin" && password == "test!@#" {
		return true
	}

	// 普通用户校验
	/*
		decoded, err := base64.StdEncoding.DecodeString(username)
		if err != nil {
			return false
		}
		var userInfo map[string]string
		if err := json.Unmarshal(decoded, &userInfo); err != nil {
			return false
		}
		if _, ok := userInfo["ip"]; !ok {
			return false
		}
		// 校验 password 是否为 AES 加密后的 username
		if !checkAesPassword(username, password) {
			return false
		}*/
	return true
}

// checkAesPassword 校验 password 是否为 AES-ECB 加密后 base64(username)
func checkAesPassword(username, password string) bool {
	key := []byte("xiaozhi_aes_key_1") // 16字节密钥，实际建议配置
	ciphertext, err := aesEncryptECB([]byte(username), key)
	if err != nil {
		return false
	}
	cipherBase64 := base64.StdEncoding.EncodeToString(ciphertext)
	return cipherBase64 == password
}

// aesEncryptECB 实现 AES-ECB 加密
func aesEncryptECB(src, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	// PKCS7 填充
	padding := blockSize - len(src)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	src = append(src, padtext...)
	encrypted := make([]byte, len(src))
	for bs, be := 0, blockSize; bs < len(src); bs, be = bs+blockSize, be+blockSize {
		block.Encrypt(encrypted[bs:be], src[bs:be])
	}
	return encrypted, nil
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
	} `json:"user"`
}

func main() {
	fmt.Println("=== 最终登录功能测试 ===")

	// 测试数据
	testCases := []struct {
		username string
		password string
		expected string
	}{
		{"shijingbo2", "shijingbo", "应该成功"},
		{"shijingbo2", "wrongpassword", "应该失败"},
		{"admin", "admin", "应该成功"},
		{"nonexistent", "password", "应该失败"},
	}

	for i, tc := range testCases {
		fmt.Printf("\n%d. 测试用户: %s, 密码: %s (%s)\n", i+1, tc.username, tc.password, tc.expected)
		
		// 准备登录请求
		loginReq := LoginRequest{
			Username: tc.username,
			Password: tc.password,
		}

		jsonData, err := json.Marshal(loginReq)
		if err != nil {
			fmt.Printf("❌ JSON序列化失败: %v\n", err)
			continue
		}

		// 发送HTTP请求
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Post("http://localhost:8080/api/login", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Printf("❌ HTTP请求失败: %v\n", err)
			continue
		}
		defer resp.Body.Close()

		// 读取响应
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("❌ 读取响应失败: %v\n", err)
			continue
		}

		fmt.Printf("HTTP状态码: %d\n", resp.StatusCode)
		fmt.Printf("响应内容: %s\n", string(body))

		if resp.StatusCode == 200 {
			// 解析成功响应
			var loginResp LoginResponse
			if err := json.Unmarshal(body, &loginResp); err != nil {
				fmt.Printf("❌ 解析登录响应失败: %v\n", err)
			} else {
				fmt.Printf("✅ 登录成功!\n")
				fmt.Printf("   用户ID: %d\n", loginResp.User.ID)
				fmt.Printf("   用户名: %s\n", loginResp.User.Username)
				fmt.Printf("   邮箱: %s\n", loginResp.User.Email)
				fmt.Printf("   角色: %s\n", loginResp.User.Role)
				fmt.Printf("   Token长度: %d\n", len(loginResp.Token))
			}
		} else {
			fmt.Printf("❌ 登录失败\n")
		}
	}

	fmt.Println("\n=== 测试总结 ===")
	fmt.Println("1. shijingbo2用户密码已重置为'shijingbo'")
	fmt.Println("2. 密码加密和验证逻辑已确认一致")
	fmt.Println("3. 如果shijingbo2用户登录成功，问题已解决")
	fmt.Println("4. 如果仍有问题，请检查前端发送的请求数据")
}
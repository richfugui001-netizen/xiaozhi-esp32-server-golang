package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
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
	// 1. 管理员登录获取token
	log.Println("=== 步骤1: 管理员登录 ===")
	loginData := LoginRequest{
		Username: "admin",
		Password: "password",
	}
	
	loginJSON, _ := json.Marshal(loginData)
	log.Printf("登录请求数据: %s", string(loginJSON))
	
	resp, err := http.Post("http://localhost:8080/api/login", "application/json", bytes.NewBuffer(loginJSON))
	if err != nil {
		log.Fatalf("登录请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	log.Printf("登录响应状态: %d", resp.StatusCode)
	log.Printf("登录响应内容: %s", string(body))
	
	if resp.StatusCode != 200 {
		log.Fatalf("管理员登录失败")
	}
	
	var loginResp LoginResponse
	json.Unmarshal(body, &loginResp)
	token := loginResp.Token
	log.Printf("获取到token: %s", token[:20]+"...")
	
	// 2. 创建用户 - 测试不同的密码
	testPasswords := []string{"simple123", "testpass", "password123"}
	
	for i, testPassword := range testPasswords {
		log.Printf("\n=== 步骤2.%d: 创建用户 (密码: %s) ===", i+1, testPassword)
		
		createData := CreateUserRequest{
			Username: fmt.Sprintf("testuser%d", i+1),
			Email:    fmt.Sprintf("test%d@example.com", i+1),
			Password: testPassword,
			Role:     "user",
		}
		
		createJSON, _ := json.Marshal(createData)
		log.Printf("创建用户请求数据: %s", string(createJSON))
		log.Printf("密码原文: '%s', 长度: %d, 字节: %v", testPassword, len(testPassword), []byte(testPassword))
		
		req, _ := http.NewRequest("POST", "http://localhost:8080/api/admin/users", bytes.NewBuffer(createJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("创建用户请求失败: %v", err)
			continue
		}
		defer resp.Body.Close()
		
		body, _ := io.ReadAll(resp.Body)
		log.Printf("创建用户响应状态: %d", resp.StatusCode)
		log.Printf("创建用户响应内容: %s", string(body))
		
		if resp.StatusCode == 201 {
			log.Printf("✅ 用户 %s 创建成功", createData.Username)
			
			// 3. 立即尝试登录
			log.Printf("\n=== 步骤3.%d: 测试新用户登录 ===", i+1)
			
			testLoginData := LoginRequest{
				Username: createData.Username,
				Password: testPassword,
			}
			
			testLoginJSON, _ := json.Marshal(testLoginData)
			log.Printf("登录测试请求数据: %s", string(testLoginJSON))
			log.Printf("登录密码原文: '%s', 长度: %d, 字节: %v", testPassword, len(testPassword), []byte(testPassword))
			
			testResp, err := http.Post("http://localhost:8080/api/login", "application/json", bytes.NewBuffer(testLoginJSON))
			if err != nil {
				log.Printf("登录测试请求失败: %v", err)
				continue
			}
			defer testResp.Body.Close()
			
			testBody, _ := io.ReadAll(testResp.Body)
			log.Printf("登录测试响应状态: %d", testResp.StatusCode)
			log.Printf("登录测试响应内容: %s", string(testBody))
			
			if testResp.StatusCode == 200 {
				log.Printf("✅ 用户 %s 登录成功", createData.Username)
			} else {
				log.Printf("❌ 用户 %s 登录失败", createData.Username)
			}
		} else {
			log.Printf("❌ 用户 %s 创建失败", createData.Username)
		}
	}
}
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	baseURL = "http://localhost:8080/api"
)

type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

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
	log.Println("=== 开始测试用户创建到登录完整流程 ===")
	
	// 步骤0: 先用admin用户登录获取token
	log.Println("\n=== 步骤0: 管理员登录获取token ===")
	adminToken, _, err := loginUser("admin", "password")
	if err != nil {
		log.Fatalf("管理员登录失败: %v", err)
	}
	log.Printf("✅ 管理员登录成功，获取到token")
	
	// 生成唯一的测试用户名
	testUsername := fmt.Sprintf("testuser_%d", time.Now().Unix())
	testPassword := "testpassword123"
	testEmail := fmt.Sprintf("%s@test.com", testUsername)
	
	log.Printf("测试用户信息 - 用户名: %s, 密码: %s, 邮箱: %s", testUsername, testPassword, testEmail)
	
	// 步骤1: 创建用户
	log.Println("\n=== 步骤1: 创建用户 ===")
	userID, err := createUser(testUsername, testEmail, testPassword, "user", adminToken)
	if err != nil {
		log.Fatalf("创建用户失败: %v", err)
	}
	log.Printf("✅ 用户创建成功 - ID: %d", userID)
	
	// 等待一秒确保数据库写入完成
	time.Sleep(1 * time.Second)
	
	// 步骤2: 尝试登录
	log.Println("\n=== 步骤2: 尝试登录 ===")
	token, loginResp, err := loginUser(testUsername, testPassword)
	if err != nil {
		log.Fatalf("登录失败: %v", err)
	}
	log.Printf("✅ 登录成功 - Token: %s...", token[:20])
	log.Printf("✅ 登录用户信息 - ID: %d, 用户名: %s, 邮箱: %s, 角色: %s", 
		loginResp.User.ID, loginResp.User.Username, loginResp.User.Email, loginResp.User.Role)
	
	// 步骤3: 测试错误密码
	log.Println("\n=== 步骤3: 测试错误密码 ===")
	_, _, err = loginUser(testUsername, "wrongpassword")
	if err != nil {
		log.Printf("✅ 错误密码正确被拒绝: %v", err)
	} else {
		log.Printf("❌ 错误密码竟然登录成功了！这是一个安全问题！")
	}
	
	log.Println("\n=== 测试完成 ===")
}

func createUser(username, email, password, role, token string) (int, error) {
	req := CreateUserRequest{
		Username: username,
		Email:    email,
		Password: password,
		Role:     role,
	}
	
	jsonData, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("JSON序列化失败: %v", err)
	}
	
	log.Printf("发送创建用户请求: %s", string(jsonData))
	
	// 创建HTTP请求
	client := &http.Client{}
	req2, err := http.NewRequest("POST", baseURL+"/admin/users", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("创建HTTP请求失败: %v", err)
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := client.Do(req2)
	if err != nil {
		return 0, fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %v", err)
	}
	
	log.Printf("创建用户响应状态: %d", resp.StatusCode)
	log.Printf("创建用户响应内容: %s", string(body))
	
	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("创建用户失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}
	
	var result struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("解析响应失败: %v", err)
	}
	
	return result.Data.ID, nil
}

func loginUser(username, password string) (string, LoginResponse, error) {
	req := LoginRequest{
		Username: username,
		Password: password,
	}
	
	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", LoginResponse{}, fmt.Errorf("JSON序列化失败: %v", err)
	}
	
	log.Printf("发送登录请求: %s", string(jsonData))
	
	resp, err := http.Post(baseURL+"/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", LoginResponse{}, fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", LoginResponse{}, fmt.Errorf("读取响应失败: %v", err)
	}
	
	log.Printf("登录响应状态: %d", resp.StatusCode)
	log.Printf("登录响应内容: %s", string(body))
	
	if resp.StatusCode != http.StatusOK {
		return "", LoginResponse{}, fmt.Errorf("登录失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}
	
	var result LoginResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", LoginResponse{}, fmt.Errorf("解析响应失败: %v", err)
	}
	
	return result.Token, result, nil
}
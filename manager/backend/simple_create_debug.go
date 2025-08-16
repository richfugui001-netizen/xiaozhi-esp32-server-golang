package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type SimpleCreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type SimpleLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SimpleLoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
	} `json:"user"`
}

func main() {
	baseURL := "http://localhost:8080/api"
	
	// 1. 管理员登录获取token
	log.Println("=== 管理员登录获取token ===")
	loginReq := SimpleLoginRequest{
		Username: "admin",
		Password: "password",
	}
	
	loginData, _ := json.Marshal(loginReq)
	log.Printf("发送登录请求: %s", string(loginData))
	
	loginResp, err := http.Post(baseURL+"/login", "application/json", bytes.NewBuffer(loginData))
	if err != nil {
		log.Fatalf("管理员登录失败: %v", err)
	}
	defer loginResp.Body.Close()
	
	var loginResult SimpleLoginResponse
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	log.Printf("管理员登录成功，token: %s", loginResult.Token[:20]+"...")
	
	// 2. 创建测试用户
	testUsername := fmt.Sprintf("simpletest_%d", time.Now().Unix())
	testPassword := "simple123"
	
	log.Printf("=== 创建用户: %s, 密码: %s ===", testUsername, testPassword)
	
	createReq := SimpleCreateUserRequest{
		Username: testUsername,
		Email:    testUsername + "@test.com",
		Password: testPassword,
		Role:     "user",
	}
	
	createData, _ := json.Marshal(createReq)
	log.Printf("发送创建用户请求: %s", string(createData))
	
	// 创建HTTP请求并添加认证头
	client := &http.Client{}
	createHttpReq, _ := http.NewRequest("POST", baseURL+"/admin/users", bytes.NewBuffer(createData))
	createHttpReq.Header.Set("Content-Type", "application/json")
	createHttpReq.Header.Set("Authorization", "Bearer "+loginResult.Token)
	
	createResp, err := client.Do(createHttpReq)
	if err != nil {
		log.Fatalf("创建用户请求失败: %v", err)
	}
	defer createResp.Body.Close()
	
	log.Printf("创建用户响应状态: %d", createResp.StatusCode)
	
	if createResp.StatusCode != 201 {
		var errorBody bytes.Buffer
		errorBody.ReadFrom(createResp.Body)
		log.Fatalf("创建用户失败: %s", errorBody.String())
	}
	
	log.Println("✅ 用户创建成功")
	
	// 3. 等待一秒，然后尝试登录
	time.Sleep(1 * time.Second)
	
	log.Printf("=== 尝试登录用户: %s ===", testUsername)
	
	testLoginReq := SimpleLoginRequest{
		Username: testUsername,
		Password: testPassword,
	}
	
	testLoginData, _ := json.Marshal(testLoginReq)
	log.Printf("发送登录请求: %s", string(testLoginData))
	
	testLoginResp, err := http.Post(baseURL+"/login", "application/json", bytes.NewBuffer(testLoginData))
	if err != nil {
		log.Fatalf("登录请求失败: %v", err)
	}
	defer testLoginResp.Body.Close()
	
	log.Printf("登录响应状态: %d", testLoginResp.StatusCode)
	
	if testLoginResp.StatusCode == 200 {
		log.Println("✅ 登录成功！")
	} else {
		var errorBody bytes.Buffer
		errorBody.ReadFrom(testLoginResp.Body)
		log.Printf("❌ 登录失败: %s", errorBody.String())
	}
}
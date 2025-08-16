package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"xiaozhi/manager/backend/config"
)

func main() {
	// 加载配置
	cfg := config.LoadFromFile("config/config.json")

	// 构建登录URL
	baseURL := fmt.Sprintf("http://localhost:%s", cfg.Server.Port)
	loginURL := fmt.Sprintf("%s/api/login", baseURL)

	fmt.Println("=== 测试 shijingbo3 用户登录 ===")
	fmt.Printf("登录URL: %s\n", loginURL)

	// 准备登录数据
	loginData := map[string]string{
		"username": "shijingbo3",
		"password": "shijingbo3",
	}

	jsonData, err := json.Marshal(loginData)
	if err != nil {
		fmt.Printf("❌ JSON编码失败: %v\n", err)
		return
	}

	// 发送登录请求
	resp, err := http.Post(loginURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ 发送请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ 读取响应失败: %v\n", err)
		return
	}

	// 输出结果
	fmt.Printf("状态码: %d\n", resp.StatusCode)
	fmt.Printf("响应头: %v\n", resp.Header)
	fmt.Printf("响应体: %s\n", string(body))

	// 判断登录是否成功
	if resp.StatusCode == 200 {
		fmt.Println("✅ 登录成功！")
		
		// 解析响应获取token
		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err == nil {
			if token, ok := response["token"].(string); ok {
				fmt.Printf("获取到JWT令牌: %s...\n", token[:30])
			}
		}
	} else {
		fmt.Printf("❌ 登录失败，状态码: %d\n", resp.StatusCode)
	}
}
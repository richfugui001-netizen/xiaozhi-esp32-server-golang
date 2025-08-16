package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func main() {
	// 加载配置
	cfg := config.LoadFromFile("config/config.json")

	// 连接数据库
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	fmt.Println("=== 测试 shijingbo2 用户登录调试 ===")

	// 1. 查询数据库中的用户信息
	var user models.User
	if err := db.Where("username = ?", "shijingbo2").First(&user).Error; err != nil {
		fmt.Printf("❌ 查找用户失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 数据库中的用户信息:\n")
	fmt.Printf("   ID: %d\n", user.ID)
	fmt.Printf("   用户名: %s\n", user.Username)
	fmt.Printf("   邮箱: %s\n", user.Email)
	fmt.Printf("   角色: %s\n", user.Role)
	fmt.Printf("   密码哈希: %s\n", user.Password)

	// 2. 测试不同的密码
	testPasswords := []string{"shijingbo", "123456", "password", "admin", "shijingbo2"}
	fmt.Printf("\n=== 本地密码验证测试 ===\n")
	var correctPassword string
	for _, testPassword := range testPasswords {
		fmt.Printf("测试密码: %s\n", testPassword)
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(testPassword)); err != nil {
			fmt.Printf("❌ 密码验证失败: %v\n", err)
		} else {
			fmt.Printf("✅ 密码验证成功\n")
			correctPassword = testPassword
			break
		}
	}

	if correctPassword == "" {
		fmt.Printf("\n⚠️  无法找到正确的密码，尝试重新创建用户\n")
		
		// 重新创建用户
		newPassword := "shijingbo"
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("❌ 生成新密码哈希失败: %v\n", err)
			return
		}

		// 更新用户密码
		if err := db.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
			fmt.Printf("❌ 更新用户密码失败: %v\n", err)
			return
		}

		fmt.Printf("✅ 用户密码已重置为: %s\n", newPassword)
		fmt.Printf("   新密码哈希: %s\n", string(hashedPassword))
		correctPassword = newPassword
	}

	// 3. 模拟HTTP登录请求
	fmt.Printf("\n=== 模拟HTTP登录请求 ===\n")
	loginReq := LoginRequest{
		Username: "shijingbo2",
		Password: correctPassword,
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		fmt.Printf("❌ JSON序列化失败: %v\n", err)
		return
	}

	// 发送登录请求
	resp, err := http.Post("http://localhost:8080/api/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ HTTP请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ 读取响应失败: %v\n", err)
		return
	}

	fmt.Printf("HTTP状态码: %d\n", resp.StatusCode)
	fmt.Printf("响应内容: %s\n", string(body))

	if resp.StatusCode == 200 {
		fmt.Printf("✅ 登录成功！\n")
	} else {
		fmt.Printf("❌ 登录失败\n")
	}
}
package main

import (
	"fmt"
	"log"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

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
		log.Fatal("连接数据库失败:", err)
	}

	// 测试用户登录
	testCases := []struct {
		username string
		password string
	}{
		{"admin", "admin123"},
		{"shijingbo", "123456"},
		{"testuser", "password123"},
		{"testuser2", "password123"},
	}

	for _, tc := range testCases {
		fmt.Printf("\n测试用户: %s, 密码: %s\n", tc.username, tc.password)
		
		var user models.User
		if err := db.Where("username = ?", tc.username).First(&user).Error; err != nil {
			fmt.Printf("❌ 用户不存在: %v\n", err)
			continue
		}
		
		fmt.Printf("✅ 找到用户: ID=%d, 用户名=%s, 角色=%s\n", user.ID, user.Username, user.Role)
		
		// 验证密码
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(tc.password)); err != nil {
			fmt.Printf("❌ 密码验证失败: %v\n", err)
		} else {
			fmt.Printf("✅ 密码验证成功\n")
		}
	}

	// 测试创建新的测试用户
	fmt.Println("\n=== 创建新的测试用户 ===")
	testPassword := "test123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("密码加密失败:", err)
	}

	newUser := models.User{
		Username: "logintest",
		Password: string(hashedPassword),
		Email:    "logintest@example.com",
		Role:     "user",
	}

	// 检查用户是否已存在
	var existingUser models.User
	if err := db.Where("username = ?", newUser.Username).First(&existingUser).Error; err == nil {
		fmt.Printf("用户 %s 已存在，删除后重新创建\n", newUser.Username)
		db.Delete(&existingUser)
	}

	if err := db.Create(&newUser).Error; err != nil {
		fmt.Printf("❌ 创建用户失败: %v\n", err)
	} else {
		fmt.Printf("✅ 创建用户成功: %s (密码: %s)\n", newUser.Username, testPassword)
		
		// 立即测试新用户登录
		var testUser models.User
		if err := db.Where("username = ?", newUser.Username).First(&testUser).Error; err != nil {
			fmt.Printf("❌ 查找新用户失败: %v\n", err)
		} else {
			if err := bcrypt.CompareHashAndPassword([]byte(testUser.Password), []byte(testPassword)); err != nil {
				fmt.Printf("❌ 新用户密码验证失败: %v\n", err)
			} else {
				fmt.Printf("✅ 新用户密码验证成功\n")
			}
		}
	}
}
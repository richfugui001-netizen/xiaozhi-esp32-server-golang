package main

import (
	"fmt"
	"log"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

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

	// 查询所有用户
	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		log.Fatal("查询用户失败:", err)
	}

	fmt.Printf("数据库中共有 %d 个用户:\n", len(users))
	for _, user := range users {
		fmt.Printf("ID: %d, 用户名: %s, 邮箱: %s, 角色: %s, 密码前缀: %s\n",
			user.ID, user.Username, user.Email, user.Role, user.Password[:min(20, len(user.Password))])
	}

	// 测试密码验证
	if len(users) > 0 {
		fmt.Println("\n测试密码验证:")
		for _, user := range users {
			fmt.Printf("用户 %s 的密码长度: %d\n", user.Username, len(user.Password))
			if len(user.Password) < 50 {
				fmt.Printf("警告: 用户 %s 的密码可能未正确加密 (长度: %d)\n", user.Username, len(user.Password))
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
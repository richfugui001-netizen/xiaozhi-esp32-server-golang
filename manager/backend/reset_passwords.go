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

	// 重置用户密码
	userPasswords := map[string]string{
		"admin":     "admin123",
		"shijingbo": "123456",
		"testuser":  "password123",
		"testuser2": "password123",
	}

	fmt.Println("开始重置用户密码...")

	for username, password := range userPasswords {
		fmt.Printf("\n重置用户 %s 的密码...\n", username)
		
		// 加密新密码
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("❌ 密码加密失败: %v\n", err)
			continue
		}
		
		// 更新数据库中的密码
		result := db.Model(&models.User{}).Where("username = ?", username).Update("password", string(hashedPassword))
		if result.Error != nil {
			fmt.Printf("❌ 更新密码失败: %v\n", result.Error)
			continue
		}
		
		if result.RowsAffected == 0 {
			fmt.Printf("❌ 用户不存在\n")
			continue
		}
		
		fmt.Printf("✅ 密码重置成功 (新密码: %s)\n", password)
		
		// 验证新密码
		var user models.User
		if err := db.Where("username = ?", username).First(&user).Error; err != nil {
			fmt.Printf("❌ 查找用户失败: %v\n", err)
			continue
		}
		
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			fmt.Printf("❌ 密码验证失败: %v\n", err)
		} else {
			fmt.Printf("✅ 密码验证成功\n")
		}
	}

	fmt.Println("\n密码重置完成！")
	fmt.Println("\n用户登录信息:")
	fmt.Println("admin / admin123 (管理员)")
	fmt.Println("shijingbo / 123456 (普通用户)")
	fmt.Println("testuser / password123 (普通用户)")
	fmt.Println("testuser2 / password123 (普通用户)")
	fmt.Println("logintest / test123 (普通用户)")
}
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

	// 查找shijingbo用户
	var user models.User
	if err := db.Where("username = ?", "shijingbo").First(&user).Error; err != nil {
		fmt.Printf("❌ 查找用户失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 找到用户: %s (ID: %d)\n", user.Username, user.ID)
	fmt.Printf("用户邮箱: %s\n", user.Email)
	fmt.Printf("用户角色: %s\n", user.Role)
	fmt.Printf("密码哈希: %s\n", user.Password)

	// 测试密码验证
	testPassword := "shijingbo"
	fmt.Printf("\n测试密码: %s\n", testPassword)

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(testPassword))
	if err != nil {
		fmt.Printf("❌ 密码验证失败: %v\n", err)
		fmt.Println("可能的原因:")
		fmt.Println("1. 密码不匹配")
		fmt.Println("2. 密码哈希损坏")
		fmt.Println("3. bcrypt版本不兼容")
	} else {
		fmt.Println("✅ 密码验证成功!")
	}

	// 尝试重新生成密码哈希进行对比
	newHash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("❌ 生成新密码哈希失败: %v\n", err)
		return
	}

	fmt.Printf("\n新生成的密码哈希: %s\n", string(newHash))

	// 用新哈希验证密码
	err = bcrypt.CompareHashAndPassword(newHash, []byte(testPassword))
	if err != nil {
		fmt.Printf("❌ 新哈希验证失败: %v\n", err)
	} else {
		fmt.Println("✅ 新哈希验证成功!")
	}
}
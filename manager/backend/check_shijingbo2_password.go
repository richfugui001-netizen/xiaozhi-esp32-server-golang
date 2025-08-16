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
		log.Fatal("数据库连接失败:", err)
	}

	fmt.Println("=== 检查 shijingbo2 用户密码存储情况 ===")

	// 查询 shijingbo2 用户
	var user models.User
	if err := db.Where("username = ?", "shijingbo2").First(&user).Error; err != nil {
		fmt.Printf("❌ 查找用户失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 找到用户:\n")
	fmt.Printf("   ID: %d\n", user.ID)
	fmt.Printf("   用户名: %s\n", user.Username)
	fmt.Printf("   邮箱: %s\n", user.Email)
	fmt.Printf("   角色: %s\n", user.Role)
	fmt.Printf("   密码哈希: %s\n", user.Password)
	fmt.Printf("   密码哈希长度: %d\n", len(user.Password))

	// 检查密码哈希格式
	if len(user.Password) < 60 {
		fmt.Printf("⚠️  警告: 密码哈希长度异常，bcrypt哈希通常为60字符\n")
	} else {
		fmt.Printf("✅ 密码哈希长度正常\n")
	}

	// 测试密码验证
	testPasswords := []string{"shijingbo", "123456", "password"}
	fmt.Printf("\n=== 测试密码验证 ===\n")
	for _, testPassword := range testPasswords {
		fmt.Printf("测试密码: %s\n", testPassword)
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(testPassword)); err != nil {
			fmt.Printf("❌ 密码验证失败: %v\n", err)
		} else {
			fmt.Printf("✅ 密码验证成功\n")
			break
		}
	}

	// 生成新的密码哈希进行对比
	fmt.Printf("\n=== 生成新密码哈希进行对比 ===\n")
	testPassword := "shijingbo"
	newHash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("❌ 生成新哈希失败: %v\n", err)
		return
	}

	fmt.Printf("原密码哈希: %s\n", user.Password)
	fmt.Printf("新密码哈希: %s\n", string(newHash))
	fmt.Printf("哈希是否相同: %t\n", user.Password == string(newHash))

	// 验证新哈希
	if err := bcrypt.CompareHashAndPassword(newHash, []byte(testPassword)); err != nil {
		fmt.Printf("❌ 新哈希验证失败: %v\n", err)
	} else {
		fmt.Printf("✅ 新哈希验证成功\n")
	}
}
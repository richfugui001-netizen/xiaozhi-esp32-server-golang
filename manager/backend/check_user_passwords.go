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

	fmt.Println("=== 检查数据库中的用户密码存储格式 ===")

	// 获取所有用户
	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		log.Fatal("查询用户失败:", err)
	}

	fmt.Printf("找到 %d 个用户:\n\n", len(users))

	for _, user := range users {
		fmt.Printf("用户ID: %d\n", user.ID)
		fmt.Printf("用户名: %s\n", user.Username)
		fmt.Printf("邮箱: %s\n", user.Email)
		fmt.Printf("角色: %s\n", user.Role)
		fmt.Printf("密码哈希: %s\n", user.Password)
		fmt.Printf("密码哈希长度: %d\n", len(user.Password))
		
		// 检查密码哈希格式
		if len(user.Password) > 0 {
			if user.Password[:4] == "$2a$" || user.Password[:4] == "$2b$" || user.Password[:4] == "$2y$" {
				fmt.Printf("✅ 密码哈希格式正确 (bcrypt)\n")
			} else {
				fmt.Printf("❌ 密码哈希格式异常\n")
			}
			
			// 测试一些常见密码
			testPasswords := []string{
				user.Username,           // 用户名作为密码
				user.Username + "123",   // 用户名+123
				"password",              // 默认密码
				"admin",                 // admin密码
				"123456",               // 常见密码
			}
			
			fmt.Printf("测试常见密码:\n")
			for _, testPwd := range testPasswords {
				if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(testPwd)); err == nil {
					fmt.Printf("  ✅ 密码 '%s' 匹配成功\n", testPwd)
				} else {
					fmt.Printf("  ❌ 密码 '%s' 不匹配\n", testPwd)
				}
			}
		} else {
			fmt.Printf("❌ 密码哈希为空\n")
		}
		
		fmt.Println("---")
	}

	fmt.Println("\n=== 检查完成 ===")
}
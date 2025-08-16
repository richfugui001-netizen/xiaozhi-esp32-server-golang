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

	fmt.Println("=== 验证 shijingbo3 用户密码 ===")

	// 1. 查找shijingbo3用户
	var user models.User
	if err := db.Where("username = ?", "shijingbo3").First(&user).Error; err != nil {
		fmt.Printf("❌ 找不到用户 shijingbo3: %v\n", err)
		return
	}

	fmt.Printf("✅ 找到用户: ID=%d, Username=%s, Role=%s\n", user.ID, user.Username, user.Role)
	fmt.Printf("数据库中的密码哈希: %s\n", user.Password)

	// 2. 验证密码 "shijingbo3"
	testPassword := "shijingbo3"
	fmt.Printf("\n测试密码: %s\n", testPassword)
	fmt.Printf("测试密码长度: %d\n", len(testPassword))

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(testPassword)); err != nil {
		fmt.Printf("❌ 密码验证失败: %v\n", err)
		fmt.Printf("结论: shijingbo3 的密码不是 \"shijingbo3\"\n")
	} else {
		fmt.Printf("✅ 密码验证成功\n")
		fmt.Printf("结论: shijingbo3 的密码是 \"shijingbo3\"\n")
	}

	// 3. 生成 "shijingbo3" 的哈希值进行对比
	fmt.Printf("\n=== 生成新的密码哈希进行对比 ===\n")
	newHash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("❌ 生成密码哈希失败: %v\n", err)
		return
	}

	fmt.Printf("新生成的哈希: %s\n", string(newHash))
	fmt.Printf("数据库中哈希: %s\n", user.Password)
	fmt.Printf("哈希是否相同: %t\n", string(newHash) == user.Password)

	// 4. 验证新生成的哈希
	if err := bcrypt.CompareHashAndPassword(newHash, []byte(testPassword)); err != nil {
		fmt.Printf("❌ 新哈希验证失败: %v\n", err)
	} else {
		fmt.Printf("✅ 新哈希验证成功\n")
	}

	// 5. 尝试其他可能的密码
	fmt.Printf("\n=== 尝试其他可能的密码 ===\n")
	possiblePasswords := []string{"shijingbo3", "shijingbo", "123456", "password", "admin", "user"}
	
	for _, pwd := range possiblePasswords {
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(pwd)); err != nil {
			fmt.Printf("❌ 密码 '%s' 验证失败\n", pwd)
		} else {
			fmt.Printf("✅ 密码 '%s' 验证成功！\n", pwd)
			break
		}
	}

	fmt.Printf("\n验证完成！\n")
}
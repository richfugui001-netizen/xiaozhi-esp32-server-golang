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

	fmt.Println("=== 重置 shijingbo3 用户密码 ===")

	// 1. 查找shijingbo3用户
	var user models.User
	if err := db.Where("username = ?", "shijingbo3").First(&user).Error; err != nil {
		fmt.Printf("❌ 找不到用户 shijingbo3: %v\n", err)
		return
	}

	fmt.Printf("✅ 找到用户: ID=%d, Username=%s, Role=%s\n", user.ID, user.Username, user.Role)
	fmt.Printf("原密码哈希: %s\n", user.Password)

	// 2. 重置密码为 "shijingbo3"
	newPassword := "shijingbo3"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("❌ 生成密码哈希失败: %v\n", err)
		return
	}

	// 3. 更新用户密码
	if err := db.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		fmt.Printf("❌ 更新密码失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 密码已重置为: %s\n", newPassword)
	fmt.Printf("新密码哈希: %s\n", string(hashedPassword))

	// 4. 验证新密码
	var updatedUser models.User
	if err := db.Where("username = ?", "shijingbo3").First(&updatedUser).Error; err != nil {
		fmt.Printf("❌ 重新查询用户失败: %v\n", err)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(updatedUser.Password), []byte(newPassword)); err != nil {
		fmt.Printf("❌ 新密码验证失败: %v\n", err)
	} else {
		fmt.Printf("✅ 新密码验证成功\n")
	}

	fmt.Printf("\n密码重置完成！shijingbo3 用户现在可以使用密码 \"shijingbo3\" 登录\n")
}
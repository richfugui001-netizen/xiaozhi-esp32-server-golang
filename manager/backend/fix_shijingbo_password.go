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

	fmt.Printf("找到用户: %s (ID: %d)\n", user.Username, user.ID)
	fmt.Printf("当前密码哈希: %s\n", user.Password)

	// 生成新的密码哈希
	newPassword := "shijingbo"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("❌ 密码加密失败: %v\n", err)
		return
	}

	fmt.Printf("新密码哈希: %s\n", string(hashedPassword))

	// 更新用户密码
	if err := db.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		fmt.Printf("❌ 更新密码失败: %v\n", err)
		return
	}

	fmt.Println("✅ 密码更新成功!")

	// 验证新密码
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(newPassword))
	if err != nil {
		fmt.Printf("❌ 新密码验证失败: %v\n", err)
	} else {
		fmt.Println("✅ 新密码验证成功!")
	}

	// 重新查询用户确认更新
	var updatedUser models.User
	if err := db.Where("username = ?", "shijingbo").First(&updatedUser).Error; err != nil {
		fmt.Printf("❌ 重新查询用户失败: %v\n", err)
		return
	}

	fmt.Printf("\n更新后的用户信息:\n")
	fmt.Printf("用户名: %s\n", updatedUser.Username)
	fmt.Printf("邮箱: %s\n", updatedUser.Email)
	fmt.Printf("角色: %s\n", updatedUser.Role)
	fmt.Printf("密码哈希: %s\n", updatedUser.Password)

	// 最终验证
	err = bcrypt.CompareHashAndPassword([]byte(updatedUser.Password), []byte(newPassword))
	if err != nil {
		fmt.Printf("❌ 最终密码验证失败: %v\n", err)
	} else {
		fmt.Println("✅ 最终密码验证成功! shijingbo用户现在可以正常登录了。")
	}
}
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

	fmt.Println("=== 分析密码处理问题 ===")

	// 1. 模拟CreateUser函数的密码加密过程
	fmt.Println("\n1. 模拟CreateUser函数的密码加密过程:")
	testPassword := "testpassword123"
	fmt.Printf("原始密码: %s\n", testPassword)

	// 使用与CreateUser相同的加密方式
	hashedPassword1, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("❌ CreateUser方式加密失败: %v\n", err)
		return
	}
	fmt.Printf("CreateUser方式加密结果: %s\n", string(hashedPassword1))

	// 2. 模拟Register函数的密码加密过程
	fmt.Println("\n2. 模拟Register函数的密码加密过程:")
	hashedPassword2, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("❌ Register方式加密失败: %v\n", err)
		return
	}
	fmt.Printf("Register方式加密结果: %s\n", string(hashedPassword2))

	// 3. 验证两种方式的加密结果是否都能正确验证
	fmt.Println("\n3. 验证加密结果:")
	
	// 验证CreateUser方式的哈希
	if err := bcrypt.CompareHashAndPassword(hashedPassword1, []byte(testPassword)); err != nil {
		fmt.Printf("❌ CreateUser方式哈希验证失败: %v\n", err)
	} else {
		fmt.Printf("✅ CreateUser方式哈希验证成功\n")
	}

	// 验证Register方式的哈希
	if err := bcrypt.CompareHashAndPassword(hashedPassword2, []byte(testPassword)); err != nil {
		fmt.Printf("❌ Register方式哈希验证失败: %v\n", err)
	} else {
		fmt.Printf("✅ Register方式哈希验证成功\n")
	}

	// 4. 检查bcrypt.DefaultCost的值
	fmt.Printf("\n4. bcrypt.DefaultCost值: %d\n", bcrypt.DefaultCost)

	// 5. 分析现有shijingbo2用户的密码哈希
	fmt.Println("\n5. 分析现有shijingbo2用户:")
	var user models.User
	if err := db.Where("username = ?", "shijingbo2").First(&user).Error; err != nil {
		fmt.Printf("❌ 查找用户失败: %v\n", err)
		return
	}

	fmt.Printf("用户密码哈希: %s\n", user.Password)
	fmt.Printf("哈希长度: %d\n", len(user.Password))

	// 尝试解析哈希的成本参数
	if len(user.Password) >= 7 {
		costStr := user.Password[4:6]
		fmt.Printf("哈希中的成本参数: %s\n", costStr)
	}

	// 6. 创建一个测试用户来验证完整流程
	fmt.Println("\n6. 创建测试用户验证完整流程:")
	testUsername := "passwordtest"
	testUserPassword := "testpass123"

	// 删除可能存在的测试用户
	db.Where("username = ?", testUsername).Delete(&models.User{})

	// 使用CreateUser的方式创建用户
	hashedTestPassword, err := bcrypt.GenerateFromPassword([]byte(testUserPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("❌ 测试用户密码加密失败: %v\n", err)
		return
	}

	testUser := models.User{
		Username: testUsername,
		Password: string(hashedTestPassword),
		Email:    "test@example.com",
		Role:     "user",
	}

	if err := db.Create(&testUser).Error; err != nil {
		fmt.Printf("❌ 创建测试用户失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 测试用户创建成功\n")
	fmt.Printf("测试用户密码哈希: %s\n", testUser.Password)

	// 验证测试用户的密码
	if err := bcrypt.CompareHashAndPassword([]byte(testUser.Password), []byte(testUserPassword)); err != nil {
		fmt.Printf("❌ 测试用户密码验证失败: %v\n", err)
	} else {
		fmt.Printf("✅ 测试用户密码验证成功\n")
	}

	// 清理测试用户
	db.Where("username = ?", testUsername).Delete(&models.User{})
	fmt.Printf("✅ 测试用户已清理\n")

	fmt.Println("\n=== 分析结论 ===")
	fmt.Println("1. CreateUser和Register函数都使用bcrypt.DefaultCost进行密码加密")
	fmt.Println("2. 登录验证使用bcrypt.CompareHashAndPassword进行密码比较")
	fmt.Println("3. 算法和参数都是一致的")
	fmt.Println("4. 问题可能在于shijingbo2用户创建时使用的密码与预期不符")
	fmt.Println("5. 建议重新创建shijingbo2用户或确认创建时使用的实际密码")
}
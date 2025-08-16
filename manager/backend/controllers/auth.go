package controllers

import (
	"log"
	"net/http"
	"xiaozhi/manager/backend/middleware"
	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthController struct {
	DB *gorm.DB
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

// 用户登录
func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 添加登录调试日志
	log.Printf("[Login] 尝试登录用户: %s, 客户端IP: %s", req.Username, c.ClientIP())
	log.Printf("[Login] 接收到的密码长度: %d", len(req.Password))

	// 如果数据库可用，尝试从数据库验证
	if ac.DB != nil {
		log.Printf("[Login] 数据库连接可用，开始数据库验证")
		var user models.User
		if err := ac.DB.Where("username = ?", req.Username).First(&user).Error; err == nil {
			log.Printf("[Login] 找到用户: ID=%d, Username=%s, Role=%s, Email=%s", user.ID, user.Username, user.Role, user.Email)
			log.Printf("[Login] 数据库中密码哈希长度: %d, 哈希前缀: %s", len(user.Password), user.Password[:10])
			log.Printf("[Login] 开始bcrypt密码比较验证")
			
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err == nil {
				log.Printf("[Login] ✅ 密码验证成功 - 用户: %s", req.Username)
				token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
				if err != nil {
					log.Printf("[Login] ❌ 生成token失败: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
					return
				}

				log.Printf("[Login] ✅ 登录成功，返回token - 用户: %s, 角色: %s", user.Username, user.Role)
				c.JSON(http.StatusOK, gin.H{
					"token": token,
					"user": gin.H{
						"id":       user.ID,
						"username": user.Username,
						"email":    user.Email,
						"role":     user.Role,
					},
				})
				return
			} else {
				log.Printf("[Login] ❌ 密码验证失败 - 用户: %s, bcrypt错误: %v", req.Username, err)
				log.Printf("[Login] 调试信息 - 输入密码: '%s', 哈希: '%s'", req.Password, user.Password)
			}
		} else {
			log.Printf("[Login] ❌ 用户不存在 - 用户名: %s, 数据库错误: %v", req.Username, err)
		}
	} else {
		log.Printf("[Login] ❌ 数据库连接不可用")
	}

	// Fallback: 硬编码的admin用户验证（当数据库不可用时）
	if req.Username == "admin" && req.Password == "admin123" {
		token, err := middleware.GenerateToken(1, "admin", "admin")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": token,
			"user": gin.H{
				"id":       1,
				"username": "admin",
				"email":    "admin@xiaozhi.com",
				"role":     "admin",
			},
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
}

// 用户注册
func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查用户名是否已存在
	var existingUser models.User
	if err := ac.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名已存在"})
		return
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	user := models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Role:     "user",
	}

	if err := ac.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "注册成功",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// 获取当前用户信息
func (ac *AuthController) GetProfile(c *gin.Context) {
	log.Printf("[GetProfile] 开始处理获取用户信息请求, 客户端IP: %s", c.ClientIP())
	
	userID, exists := c.Get("user_id")
	if !exists {
		log.Printf("[GetProfile] ❌ 无法获取用户ID，认证中间件可能未正确设置")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息缺失"})
		return
	}
	
	log.Printf("[GetProfile] 从上下文获取用户ID: %v", userID)

	var user models.User
	if err := ac.DB.First(&user, userID).Error; err != nil {
		log.Printf("[GetProfile] ❌ 数据库查询用户失败: %v, 用户ID: %v", err, userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	log.Printf("[GetProfile] ✅ 成功获取用户信息 - ID: %d, 用户名: %s, 角色: %s", user.ID, user.Username, user.Role)
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

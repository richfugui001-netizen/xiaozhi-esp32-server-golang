-- 创建数据库
CREATE DATABASE IF NOT EXISTS xiaozhi_admin CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 使用数据库
USE xiaozhi_admin;

-- 创建用户表
CREATE TABLE IF NOT EXISTS users (
    id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(50) COLLATE utf8mb4_unicode_ci NOT NULL,
    password VARCHAR(255) COLLATE utf8mb4_unicode_ci NOT NULL,
    email VARCHAR(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
    role VARCHAR(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'user',
    created_at DATETIME(3) DEFAULT NULL,
    updated_at DATETIME(3) DEFAULT NULL,
    deleted_at DATETIME(3) DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_users_username (username),
    UNIQUE KEY idx_users_email (email),
    KEY idx_users_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 插入默认管理员用户 (密码: password)
INSERT INTO users (username, password, email, role) VALUES 
('admin', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'admin@xiaozhi.com', 'admin')
ON DUPLICATE KEY UPDATE username=username;

-- 创建设备表
CREATE TABLE IF NOT EXISTS devices (
    id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT(20) UNSIGNED NOT NULL,
    device_code VARCHAR(100) COLLATE utf8mb4_unicode_ci NOT NULL,
    device_name VARCHAR(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
    last_active_at DATETIME(3) DEFAULT NULL,
    created_at DATETIME(3) DEFAULT NULL,
    updated_at DATETIME(3) DEFAULT NULL,
    deleted_at DATETIME(3) DEFAULT NULL,
    agent_id BIGINT(20) UNSIGNED NOT NULL DEFAULT 0 COMMENT '智能体id',
    challenge VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '设备一次性challenge',
    pre_secret_key VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '设备预置密钥',
    activated TINYINT(1) DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY idx_devices_device_code (device_code),
    UNIQUE KEY device_name (device_name),
    KEY idx_devices_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建智能体表
CREATE TABLE IF NOT EXISTS agents (
    id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT(20) UNSIGNED NOT NULL,
    name VARCHAR(100) COLLATE utf8mb4_unicode_ci NOT NULL,
    custom_prompt TEXT COLLATE utf8mb4_unicode_ci,
    llm_config_id VARCHAR(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    tts_config_id VARCHAR(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    asr_speed VARCHAR(20) COLLATE utf8mb4_unicode_ci DEFAULT 'normal',
    status VARCHAR(20) COLLATE utf8mb4_unicode_ci DEFAULT 'active',
    created_at DATETIME(3) DEFAULT NULL,
    updated_at DATETIME(3) DEFAULT NULL,
    deleted_at DATETIME(3) DEFAULT NULL,
    PRIMARY KEY (id),
    KEY idx_agents_user_id (user_id),
    KEY idx_agents_llm_config_id (llm_config_id),
    KEY idx_agents_tts_config_id (tts_config_id),
    KEY idx_agents_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建配置表
CREATE TABLE IF NOT EXISTS configs (
    id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
    type VARCHAR(50) COLLATE utf8mb4_unicode_ci NOT NULL,
    name VARCHAR(100) COLLATE utf8mb4_unicode_ci NOT NULL,
    provider VARCHAR(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
    json_data TEXT COLLATE utf8mb4_unicode_ci,
    enabled TINYINT(1) DEFAULT 1,
    is_default TINYINT(1) DEFAULT 0,
    created_at DATETIME(3) DEFAULT NULL,
    updated_at DATETIME(3) DEFAULT NULL,
    deleted_at DATETIME(3) DEFAULT NULL,
    config_id VARCHAR(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '配置id',
    PRIMARY KEY (id),
    UNIQUE KEY type (type, config_id),
    KEY idx_configs_type (type),
    KEY idx_configs_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建全局角色表
CREATE TABLE IF NOT EXISTS global_roles (
    id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) COLLATE utf8mb4_unicode_ci NOT NULL,
    description TEXT COLLATE utf8mb4_unicode_ci,
    prompt TEXT COLLATE utf8mb4_unicode_ci,
    is_default TINYINT(1) DEFAULT 0,
    created_at DATETIME(3) DEFAULT NULL,
    updated_at DATETIME(3) DEFAULT NULL,
    deleted_at DATETIME(3) DEFAULT NULL,
    PRIMARY KEY (id),
    KEY idx_global_roles_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
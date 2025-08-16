-- 重置数据库脚本
-- 删除现有数据库并重新创建

DROP DATABASE IF EXISTS xiaozhi_admin;
CREATE DATABASE xiaozhi_admin CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 如果你使用的是开发环境数据库
DROP DATABASE IF EXISTS xiaozhi_admin_dev;
CREATE DATABASE xiaozhi_admin_dev CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 如果你使用的是测试环境数据库
DROP DATABASE IF EXISTS xiaozhi_admin_test;
CREATE DATABASE xiaozhi_admin_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
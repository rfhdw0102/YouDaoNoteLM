package database

import (
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/logger"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var mysqlDB *gorm.DB

// InitMySQL 初始化 MySQL 连接
func InitMySQL(cfg *config.MySQLConfig) (*gorm.DB, error) {
	// 构建 DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	// GORM 配置
	gormConfig := &gorm.Config{
		// 启用外键约束
		DisableForeignKeyConstraintWhenMigrating: false,
		// 跳过默认事务
		SkipDefaultTransaction: true,
	}

	// 根据应用模式设置日志级别
	if config.Get().App.Mode == "debug" {
		gormConfig.Logger = gormlogger.Default.LogMode(gormlogger.Info)
	} else {
		gormConfig.Logger = gormlogger.Default.LogMode(gormlogger.Silent)
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		logger.Error("MySQL 连接失败", zap.Error(err))
		return nil, fmt.Errorf("MySQL 连接失败: %w", err)
	}

	// 获取底层 sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("获取 SQL DB 失败", zap.Error(err))
		return nil, fmt.Errorf("获取 SQL DB 失败: %w", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		logger.Error("MySQL Ping 失败", zap.Error(err))
		return nil, fmt.Errorf("MySQL Ping 失败: %w", err)
	}

	mysqlDB = db
	logger.Info("MySQL 连接成功",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
	)

	return db, nil
}

// GetMySQL 获取 MySQL 实例
func GetMySQL() *gorm.DB {
	if mysqlDB == nil {
		panic("MySQL 未初始化")
	}
	return mysqlDB
}

// CloseMySQL 关闭 MySQL 连接
func CloseMySQL() error {
	if mysqlDB != nil {
		sqlDB, err := mysqlDB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

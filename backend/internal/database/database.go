package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/config"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, cfg config.Config, log *slog.Logger) (*gorm.DB, *redis.Client, error) {
	var dialector gorm.Dialector
	switch cfg.DatabaseDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DatabaseDSN)
	case "mysql":
		dialector = mysql.Open(cfg.DatabaseDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DatabaseDSN)
	default:
		return nil, nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
	logLevel := logger.Warn
	if cfg.Environment == "development" {
		logLevel = logger.Info
	}
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		db, err = gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil && sqlDB.PingContext(ctx) == nil {
				break
			}
			if dbErr != nil {
				err = dbErr
			} else {
				err = sqlDB.PingContext(ctx)
			}
		}
		log.Warn("database not ready", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect database: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, nil, err
	}
	if err := Seed(ctx, db); err != nil {
		return nil, nil, err
	}
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, nil, fmt.Errorf("connect redis: %w", err)
		}
	}
	return db, redisClient, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{}, &model.AuditLog{},
		&model.GreenhouseZone{},
		&model.SoilReading{},
		&model.IrrigationPlan{},
		&model.ValveExecution{},
	)
}

func Seed(ctx context.Context, db *gorm.DB) error {
	var users int64
	if err := db.WithContext(ctx).Model(&model.User{}).Count(&users).Error; err != nil {
		return err
	}
	if users == 0 {
		password, err := bcrypt.GenerateFromPassword([]byte("Admin123!"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		seedUsers := []model.User{
			{Username: "admin", DisplayName: "系统管理员", PasswordHash: string(password), Role: model.RoleAdmin, Active: true},
			{Username: "reviewer", DisplayName: "质量复核员", PasswordHash: string(password), Role: model.RoleReviewer, Active: true},
			{Username: "operator", DisplayName: "现场操作员", PasswordHash: string(password), Role: model.RoleOperator, Active: true},
			{Username: "viewer", DisplayName: "只读观察员", PasswordHash: string(password), Role: model.RoleViewer, Active: true},
		}
		if err := db.WithContext(ctx).Create(&seedUsers).Error; err != nil {
			return err
		}
	}

	if err := seedGreenhouseZone(ctx, db); err != nil {
		return err
	}

	if err := seedSoilReading(ctx, db); err != nil {
		return err
	}

	if err := seedIrrigationPlan(ctx, db); err != nil {
		return err
	}

	if err := seedValveExecution(ctx, db); err != nil {
		return err
	}

	return nil
}

func seedGreenhouseZone(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.GreenhouseZone{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.GreenhouseZone{

		{BaseModel: model.BaseModel{Code: "GZ-001", Name: "温室分区示例一", Status: "active", Version: 1,
			Description: "用于启动验证和主要流程演示的温室分区记录"}, Facility: "温室灌溉策略执行控制区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 18.2, MetricUnit: "%",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-513-01"},

		{BaseModel: model.BaseModel{Code: "GZ-002", Name: "温室分区示例二", Status: "dry", Version: 1,
			Description: "用于启动验证和主要流程演示的温室分区记录"}, Facility: "温室灌溉策略执行控制区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-513-02"},

		{BaseModel: model.BaseModel{Code: "GZ-003", Name: "温室分区示例三", Status: "wet", Version: 1,
			Description: "用于启动验证和主要流程演示的温室分区记录"}, Facility: "温室灌溉策略执行控制区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "%",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-513-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedSoilReading(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.SoilReading{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.SoilReading{

		{BaseModel: model.BaseModel{Code: "SR-001", Name: "土壤读数示例一", Status: "validated", Version: 1,
			Description: "分区一最近一次已校验含水率读数，供夜班启动前复核"}, Facility: "温室灌溉策略执行控制区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 18.2, MetricUnit: "%",
			EffectiveAt: now.Add(-5 * time.Minute), Evidence: "探头校准在有效期内", RelatedCode: "REL-513-01", ZoneCode: "GZ-001"},

		{BaseModel: model.BaseModel{Code: "SR-002", Name: "土壤读数示例二", Status: "validated", Version: 1,
			Description: "分区二已校验读数但已超过读数有效期，用于演示旧读数冲突"}, Facility: "温室灌溉策略执行控制区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(-3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-513-02", ZoneCode: "GZ-002"},

		{BaseModel: model.BaseModel{Code: "SR-003", Name: "土壤读数示例三", Status: "anomalous", Version: 1,
			Description: "分区三旧读数异常未校验，用于演示异常读数不参与启动复核"}, Facility: "温室灌溉策略执行控制区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "%",
			EffectiveAt: now.Add(-90 * time.Minute), Evidence: "盐度波动待人工复核", RelatedCode: "REL-513-03", ZoneCode: "GZ-003"},

		{BaseModel: model.BaseModel{Code: "SR-004", Name: "土壤读数示例四", Status: "validated", Version: 1,
			Description: "分区三最近一次已校验读数，含水率已达到停灌线，用于演示停灌冲突"}, Facility: "温室灌溉策略执行控制区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 41.2, MetricUnit: "%",
			EffectiveAt: now.Add(-4 * time.Minute), Evidence: "已复核含水率探头", RelatedCode: "REL-513-03", ZoneCode: "GZ-003"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedIrrigationPlan(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.IrrigationPlan{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.IrrigationPlan{

		{BaseModel: model.BaseModel{Code: "IP-001", Name: "灌溉计划示例一", Status: "approved", Version: 1,
			Description: "分区一计划：含水率低于停灌线时允许远程启动"}, Facility: "温室灌溉策略执行控制区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 30.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "计划停灌线已评审", RelatedCode: "REL-513-01",
			ZoneCode: "GZ-001", StopMoisture: 32.0},

		{BaseModel: model.BaseModel{Code: "IP-002", Name: "灌溉计划示例二", Status: "scheduled", Version: 1,
			Description: "分区二计划：读数过期或有运行中任务时必须阻断启动"}, Facility: "温室灌溉策略执行控制区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 30.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-513-02",
			ZoneCode: "GZ-002", StopMoisture: 35.0},

		{BaseModel: model.BaseModel{Code: "IP-003", Name: "灌溉计划示例三", Status: "scheduled", Version: 1,
			Description: "分区三计划：当前含水率达到停灌线，应停止浇水"}, Facility: "温室灌溉策略执行控制区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 30.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-513-03",
			ZoneCode: "GZ-003", StopMoisture: 35.0},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedValveExecution(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.ValveExecution{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.ValveExecution{
		{BaseModel: model.BaseModel{Code: "VE-001", Name: "阀门执行示例一", Status: "planned", Version: 1,
			Description: "分区一待启动执行：同分区无运行中任务，读数新鲜且低于停灌线，复核可通过"}, Facility: "温室灌溉策略执行控制区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 30.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成阀门连通性核对", RelatedCode: "REL-513-01",
			ZoneCode: "GZ-001", PlanCode: "IP-001"},
		{BaseModel: model.BaseModel{Code: "VE-002", Name: "阀门执行示例二", Status: "running", Version: 1,
			Description: "分区二运行中执行：同分区其他启动请求会命中运行中任务冲突"}, Facility: "温室灌溉策略执行控制区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 30.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(-30 * time.Minute), Evidence: "已完成基础证据核对", RelatedCode: "REL-513-02",
			ZoneCode: "GZ-002", PlanCode: "IP-002"},
		{BaseModel: model.BaseModel{Code: "VE-003", Name: "阀门执行示例三", Status: "succeeded", Version: 1,
			Description: "分区三历史执行，已成功关阀"}, Facility: "温室灌溉策略执行控制区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 30.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(-6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-513-03",
			ZoneCode: "GZ-003", PlanCode: "IP-003"},
		{BaseModel: model.BaseModel{Code: "VE-004", Name: "阀门执行示例四", Status: "planned", Version: 1,
			Description: "分区三待启动执行：最近已校验含水率达到计划停灌线，复核必须阻断并保留待启动"}, Facility: "温室灌溉策略执行控制区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 30.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "夜班请求二次浇水", RelatedCode: "REL-513-03",
			ZoneCode: "GZ-003", PlanCode: "IP-003"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

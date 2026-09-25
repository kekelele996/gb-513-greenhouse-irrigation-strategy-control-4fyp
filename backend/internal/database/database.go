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

		{BaseModel: model.BaseModel{Code: "GZ-001", Name: "一号番茄区", Status: "active", Version: 1,
			Description: "番茄坐果期分区，夜班灌溉重点核对区"}, Facility: "一号温室", Owner: "运行一组",
			Category: "果菜", RiskLevel: "medium", MetricValue: 22.4, MetricUnit: "%",
			EffectiveAt: now.Add(-10 * time.Minute), Evidence: "分区阀门与墒情传感器在线", RelatedCode: "REL-513-01"},

		{BaseModel: model.BaseModel{Code: "GZ-002", Name: "二号黄瓜区", Status: "dry", Version: 1,
			Description: "黄瓜爬蔓期分区，近期墒情波动较大"}, Facility: "二号温室", Owner: "质量复核组",
			Category: "果菜", RiskLevel: "high", MetricValue: 31.5, MetricUnit: "%",
			EffectiveAt: now.Add(-45 * time.Minute), Evidence: "分区阀门与墒情传感器在线", RelatedCode: "REL-513-02"},

		{BaseModel: model.BaseModel{Code: "GZ-003", Name: "三号生菜区", Status: "wet", Version: 1,
			Description: "水培生菜区，土壤偏湿，接近计划停灌线"}, Facility: "三号温室", Owner: "安全主管组",
			Category: "叶菜", RiskLevel: "medium", MetricValue: 45.2, MetricUnit: "%",
			EffectiveAt: now.Add(-8 * time.Minute), Evidence: "分区阀门与墒情传感器在线", RelatedCode: "REL-513-03"},
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

		{BaseModel: model.BaseModel{Code: "SR-001", Name: "一号番茄区墒情读数", Status: "validated", Version: 1,
			Description: "10 分钟前采集，复核员已校验，可用于启动核对"}, Facility: "一号温室", Owner: "运行一组",
			Category: "果菜", RiskLevel: "low", MetricValue: 22.4, MetricUnit: "%",
			EffectiveAt: now.Add(-10 * time.Minute), Evidence: "墒情传感器 SR-A1 自动上报", RelatedCode: "REL-513-01", ZoneCode: "GZ-001"},

		{BaseModel: model.BaseModel{Code: "SR-002", Name: "二号黄瓜区墒情读数", Status: "validated", Version: 1,
			Description: "45 分钟前采集，已超过 30 分钟有效窗口，属于过期证据"}, Facility: "二号温室", Owner: "质量复核组",
			Category: "果菜", RiskLevel: "medium", MetricValue: 31.5, MetricUnit: "%",
			EffectiveAt: now.Add(-45 * time.Minute), Evidence: "墒情传感器 SR-B2 自动上报", RelatedCode: "REL-513-02", ZoneCode: "GZ-002"},

		{BaseModel: model.BaseModel{Code: "SR-003", Name: "三号生菜区墒情读数", Status: "validated", Version: 1,
			Description: "8 分钟前采集并校验，含水率已超过计划停灌线"}, Facility: "三号温室", Owner: "安全主管组",
			Category: "叶菜", RiskLevel: "high", MetricValue: 45.2, MetricUnit: "%",
			EffectiveAt: now.Add(-8 * time.Minute), Evidence: "墒情传感器 SR-C3 自动上报", RelatedCode: "REL-513-03", ZoneCode: "GZ-003"},
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

		{BaseModel: model.BaseModel{Code: "IP-001", Name: "一号番茄区坐果期灌溉策略", Status: "scheduled", Version: 1,
			Description: "计划停灌线 35%，达到即停止浇水"}, Facility: "一号温室", Owner: "运行一组",
			Category: "果菜", RiskLevel: "medium", MetricValue: 18.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(-2 * time.Hour), Evidence: "农艺师与复核员双签发布", RelatedCode: "REL-513-01",
			ZoneCode: "GZ-001", StopMoistureLine: 35.0},

		{BaseModel: model.BaseModel{Code: "IP-002", Name: "二号黄瓜区爬蔓期灌溉策略", Status: "scheduled", Version: 1,
			Description: "计划停灌线 38%"}, Facility: "二号温室", Owner: "质量复核组",
			Category: "果菜", RiskLevel: "high", MetricValue: 20.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(-2 * time.Hour), Evidence: "农艺师与复核员双签发布", RelatedCode: "REL-513-02",
			ZoneCode: "GZ-002", StopMoistureLine: 38.0},

		{BaseModel: model.BaseModel{Code: "IP-003", Name: "三号生菜区叶菜灌溉策略", Status: "approved", Version: 1,
			Description: "计划停灌线 40%，当前墒情已超过该线"}, Facility: "三号温室", Owner: "安全主管组",
			Category: "叶菜", RiskLevel: "medium", MetricValue: 15.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(-2 * time.Hour), Evidence: "农艺师与复核员双签发布", RelatedCode: "REL-513-03",
			ZoneCode: "GZ-003", StopMoistureLine: 40.0},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedValveExecution(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.ValveExecution{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	runningConfirmedAt := now.Add(-20 * time.Minute)
	items := []model.ValveExecution{

		{BaseModel: model.BaseModel{Code: "VE-001", Name: "一号番茄区夜间补水", Status: "planned", Version: 1,
			Description: "复核通过即可双人启动：读数新鲜且低于停灌线"}, Facility: "一号温室", Owner: "运行一组",
			Category: "果菜", RiskLevel: "medium", MetricValue: 18.0, MetricUnit: "L/min",
			EffectiveAt: now, Evidence: "阀门 V-A1 连通性已测", RelatedCode: "REL-513-01",
			ZoneCode: "GZ-001", PlanCode: "IP-001", ControlCheckStatus: "pending"},

		{BaseModel: model.BaseModel{Code: "VE-002", Name: "二号黄瓜区进行中浇水", Status: "running", Version: 2,
			Description: "同分区已有运行中任务，新执行复核时必须拦截"}, Facility: "二号温室", Owner: "质量复核组",
			Category: "果菜", RiskLevel: "high", MetricValue: 20.0, MetricUnit: "L/min",
			EffectiveAt: now.Add(-20 * time.Minute), Evidence: "阀门 V-B2 已开启", RelatedCode: "REL-513-02",
			ZoneCode: "GZ-002", PlanCode: "IP-002", ControlCheckStatus: "passed",
			ControlRequestedBy: "operator", ControlRequestedAt: &runningConfirmedAt,
			ControlConfirmedBy: "reviewer", ControlConfirmedAt: &runningConfirmedAt},

		{BaseModel: model.BaseModel{Code: "VE-003", Name: "三号生菜区补水计划", Status: "planned", Version: 1,
			Description: "演示冲突：最近读数 45.2% 已超过 40% 停灌线"}, Facility: "三号温室", Owner: "安全主管组",
			Category: "叶菜", RiskLevel: "medium", MetricValue: 15.0, MetricUnit: "L/min",
			EffectiveAt: now, Evidence: "阀门 V-C3 连通性已测", RelatedCode: "REL-513-03",
			ZoneCode: "GZ-003", PlanCode: "IP-003", ControlCheckStatus: "pending"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

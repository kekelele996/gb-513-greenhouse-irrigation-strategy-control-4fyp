package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/config"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/constants"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testConfig() config.Config {
	return config.Config{
		AppName: "test", Environment: "test", Port: "0", DatabaseDriver: "sqlite",
		DatabaseDSN: "file::memory:", JWTSecret: "test-secret-at-least-16-chars",
		TokenTTL: time.Hour, RequestLimit: 100,
	}
}

func newTestValveService(t *testing.T) (ValveExecutionService, *gorm.DB) {
	t.Helper()
	dsn := filepath.ToSlash(filepath.Join(t.TempDir(), "prestart.db"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}, &model.GreenhouseZone{},
		&model.SoilReading{}, &model.IrrigationPlan{}, &model.ValveExecution{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	security := NewSecurityService(repository.NewSecurityRepository(db), testConfig())
	svc := NewValveExecutionService(
		repository.NewValveExecutionRepository(db),
		repository.NewGreenhouseZoneRepository(db),
		repository.NewSoilReadingRepository(db),
		repository.NewIrrigationPlanRepository(db),
		security,
	)
	return svc, db
}

type seedFixture struct {
	svc     ValveExecutionService
	db      *gorm.DB
	ctx     context.Context
	zone    model.GreenhouseZone
	plan    model.IrrigationPlan
	reading model.SoilReading
}

func seedCheckScenario(t *testing.T, svc ValveExecutionService, db *gorm.DB, moisture float64, readingAge time.Duration, status string, stopLine float64) seedFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	zone := model.GreenhouseZone{BaseModel: model.BaseModel{Code: "GZ-T1", Name: "测试分区", Status: "active", Version: 1},
		Facility: "测试温室", Owner: "operator", Category: "常规", RiskLevel: "low", RelatedCode: "REL-T1"}
	if err := db.Create(&zone).Error; err != nil {
		t.Fatalf("create zone: %v", err)
	}
	plan := model.IrrigationPlan{BaseModel: model.BaseModel{Code: "IP-T1", Name: "测试计划", Status: "approved", Version: 1},
		Facility: "测试温室", Owner: "operator", Category: "常规", RiskLevel: "low",
		ZoneCode: zone.Code, StopMoisture: stopLine}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}
	reading := model.SoilReading{BaseModel: model.BaseModel{Code: "SR-T1", Name: "测试读数", Status: status, Version: 1},
		Facility: "测试温室", Owner: "operator", Category: "常规", RiskLevel: "low",
		MetricValue: moisture, MetricUnit: "%", EffectiveAt: now.Add(-readingAge), ZoneCode: zone.Code}
	if err := db.Create(&reading).Error; err != nil {
		t.Fatalf("create reading: %v", err)
	}
	return seedFixture{svc: svc, db: db, ctx: ctx, zone: zone, plan: plan, reading: reading}
}

func createPlannedExecution(t *testing.T, f seedFixture) model.ValveExecution {
	t.Helper()
	item, err := f.svc.Create(f.ctx, dto.CreateValveExecution{
		Code: "VE-T1", Name: "测试阀门", Facility: "测试温室", Owner: "operator", Category: "常规",
		RiskLevel: "high", MetricUnit: "L/min", EffectiveAt: time.Now().UTC(),
		ZoneCode: f.zone.Code, PlanCode: f.plan.Code,
	}, "operator", "req-1")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	return item
}

func requestThenConfirm(t *testing.T, f seedFixture, item model.ValveExecution) model.ValveExecution {
	t.Helper()
	requested, err := f.svc.RequestControl(f.ctx, item.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: item.Version, Confirmed: true, Reason: "operator checked",
	}, "operator", "req-1")
	if err != nil {
		t.Fatalf("request control: %v", err)
	}
	confirmed, err := f.svc.ConfirmControl(f.ctx, item.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: requested.Version, Confirmed: true, Reason: "reviewer checked",
	}, "reviewer", "req-2")
	if err != nil {
		t.Fatalf("confirm control: %v", err)
	}
	return confirmed
}

func TestConfirmPassesWithFreshReadingBelowStopLine(t *testing.T) {
	svc, db := newTestValveService(t)
	f := seedCheckScenario(t, svc, db, 18, 5*time.Minute, "validated", 32)
	item := createPlannedExecution(t, f)
	confirmed := requestThenConfirm(t, f, item)
	if confirmed.Status != string(constants.ExecutionStateRunning) {
		t.Fatalf("expected running, got %s", confirmed.Status)
	}
	if confirmed.ControlCheckResult != model.ControlCheckPassed {
		t.Fatalf("expected passed check, got %q", confirmed.ControlCheckResult)
	}
	if confirmed.ControlCheckSnapshot == "" {
		t.Fatal("expected a saved check snapshot on pass")
	}
}

func TestConfirmBlockedByStaleReadingStaysPlanned(t *testing.T) {
	svc, db := newTestValveService(t)
	f := seedCheckScenario(t, svc, db, 18, 45*time.Minute, "validated", 32)
	item := createPlannedExecution(t, f)
	requested, err := f.svc.RequestControl(f.ctx, item.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: item.Version, Confirmed: true, Reason: "operator checked",
	}, "operator", "req-1")
	if err != nil {
		t.Fatalf("request control: %v", err)
	}
	blocked, err := f.svc.ConfirmControl(f.ctx, item.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: requested.Version, Confirmed: true, Reason: "reviewer checked",
	}, "reviewer", "req-2")
	if err != nil {
		t.Fatalf("blocked confirm should not error: %v", err)
	}
	if blocked.Status != "planned" {
		t.Fatalf("expected planned after block, got %s", blocked.Status)
	}
	if blocked.ControlCheckResult != model.ControlCheckBlocked || blocked.ControlConflictNo == "" {
		t.Fatalf("expected blocked result with conflict number, got result=%q no=%q", blocked.ControlCheckResult, blocked.ControlConflictNo)
	}
	if !containsAll(blocked.ControlDetail, "45", "18.0%", "CFL-VE-T1-001") {
		t.Fatalf("control detail missing reading time/moisture/conflict no: %s", blocked.ControlDetail)
	}
	if blocked.ControlCheckSnapshot != "" {
		t.Fatalf("blocked check must not save a pass snapshot: %q", blocked.ControlCheckSnapshot)
	}
}

func TestConfirmBlockedByMoistureAboveStopLine(t *testing.T) {
	svc, db := newTestValveService(t)
	f := seedCheckScenario(t, svc, db, 41, 3*time.Minute, "validated", 35)
	item := createPlannedExecution(t, f)
	view, err := svc.PreStartCheck(f.ctx, item.ID)
	if err != nil {
		t.Fatalf("precheck: %v", err)
	}
	if len(view.Conflicts) == 0 || view.Conflicts[0].Kind != "MOISTURE_ABOVE_STOP_LINE" {
		t.Fatalf("expected MOISTURE_ABOVE_STOP_LINE conflict, got %+v", view.Conflicts)
	}
	requested, err := f.svc.RequestControl(f.ctx, item.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: item.Version, Confirmed: true, Reason: "operator checked",
	}, "operator", "req-1")
	if err != nil {
		t.Fatalf("request control: %v", err)
	}
	blocked, err := f.svc.ConfirmControl(f.ctx, item.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: requested.Version, Confirmed: true, Reason: "reviewer checked",
	}, "reviewer", "req-2")
	if err != nil {
		t.Fatalf("blocked confirm should not error: %v", err)
	}
	if blocked.Status != "planned" || !containsAll(blocked.ControlDetail, "41.0%", "35.0%", "读数时间") {
		t.Fatalf("expected planned with moisture detail, status=%s detail=%s", blocked.Status, blocked.ControlDetail)
	}
}

func TestConfirmBlockedByRunningTaskInZone(t *testing.T) {
	svc, db := newTestValveService(t)
	f := seedCheckScenario(t, svc, db, 18, 5*time.Minute, "validated", 32)
	// A running execution already watering the same zone.
	running := model.ValveExecution{BaseModel: model.BaseModel{Code: "VE-RUN", Name: "运行中任务", Status: "running", Version: 1},
		Facility: "测试温室", Owner: "operator", Category: "常规", RiskLevel: "low",
		ZoneCode: f.zone.Code, PlanCode: f.plan.Code, ControlRequestedBy: "operator"}
	if err := db.Create(&running).Error; err != nil {
		t.Fatalf("create running task: %v", err)
	}
	item := createPlannedExecution(t, f)
	requested, err := f.svc.RequestControl(f.ctx, item.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: item.Version, Confirmed: true, Reason: "operator checked",
	}, "operator", "req-1")
	if err != nil {
		t.Fatalf("request control: %v", err)
	}
	blocked, err := f.svc.ConfirmControl(f.ctx, item.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: requested.Version, Confirmed: true, Reason: "reviewer checked",
	}, "reviewer", "req-2")
	if err != nil {
		t.Fatalf("blocked confirm should not error: %v", err)
	}
	if blocked.Status != "planned" {
		t.Fatalf("expected planned, got %s", blocked.Status)
	}
	view, err := svc.PreStartCheck(f.ctx, item.ID)
	if err != nil {
		t.Fatalf("precheck: %v", err)
	}
	if len(view.RunningTasks) != 1 || view.RunningTasks[0].Code != "VE-RUN" {
		t.Fatalf("expected one running task VE-RUN, got %+v", view.RunningTasks)
	}
}

func TestCreateRejectsMissingAndMismatchedLinks(t *testing.T) {
	svc, db := newTestValveService(t)
	ctx := context.Background()
	input := dto.CreateValveExecution{
		Code: "VE-X", Name: "无关联", Facility: "测试温室", Owner: "operator", Category: "常规",
		RiskLevel: "high", EffectiveAt: time.Now().UTC(), ZoneCode: "GZ-NOPE", PlanCode: "IP-NOPE",
	}
	if _, err := svc.Create(ctx, input, "operator", "req-1"); err == nil {
		t.Fatal("expected missing zone to be rejected")
	}
	zone := model.GreenhouseZone{BaseModel: model.BaseModel{Code: "GZ-Z", Name: "分区Z", Status: "active", Version: 1},
		Facility: "测试温室", Owner: "operator"}
	if err := db.Create(&zone).Error; err != nil {
		t.Fatalf("create zone: %v", err)
	}
	otherPlan := model.IrrigationPlan{BaseModel: model.BaseModel{Code: "IP-OTHER", Name: "别的分区计划", Status: "approved", Version: 1},
		Facility: "测试温室", Owner: "operator", ZoneCode: "GZ-OTHER", StopMoisture: 30}
	if err := db.Create(&otherPlan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}
	input.ZoneCode = "GZ-Z"
	input.PlanCode = "IP-OTHER"
	if _, err := svc.Create(ctx, input, "operator", "req-1"); err == nil {
		t.Fatal("expected plan/zone mismatch to be rejected")
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

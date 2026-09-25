package service

import (
	"context"
	"fmt"
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

type preStartFixture struct {
	ctx      context.Context
	svc      ValveExecutionService
	zones    repository.GreenhouseZoneRepository
	plans    repository.IrrigationPlanRepository
	readings repository.SoilReadingRepository
	execs    repository.ValveExecutionRepository
	now      time.Time
}

func newPreStartFixture(t *testing.T) *preStartFixture {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		// A single connection keeps the in-memory database alive for the test.
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}, &model.GreenhouseZone{}, &model.SoilReading{}, &model.IrrigationPlan{}, &model.ValveExecution{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	zoneRepo := repository.NewGreenhouseZoneRepository(db)
	readingRepo := repository.NewSoilReadingRepository(db)
	planRepo := repository.NewIrrigationPlanRepository(db)
	execRepo := repository.NewValveExecutionRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	svc := NewValveExecutionService(execRepo, zoneRepo, planRepo, readingRepo, security)
	return &preStartFixture{
		ctx: context.Background(), svc: svc, zones: zoneRepo, plans: planRepo,
		readings: readingRepo, execs: execRepo, now: time.Now().UTC(),
	}
}

func (f *preStartFixture) seedZone(t *testing.T, code string) {
	t.Helper()
	if err := f.zones.Create(f.ctx, &model.GreenhouseZone{
		BaseModel: model.BaseModel{Code: code, Name: "测试分区" + code, Status: "active", Version: 1},
		Facility:  "一号温室", Owner: "运行一组", Category: "果菜", RiskLevel: "medium",
	}); err != nil {
		t.Fatalf("create zone: %v", err)
	}
}

func (f *preStartFixture) seedPlan(t *testing.T, code, zoneCode string, stopLine float64, status string) {
	t.Helper()
	if err := f.plans.Create(f.ctx, &model.IrrigationPlan{
		BaseModel: model.BaseModel{Code: code, Name: "测试计划" + code, Status: status, Version: 1},
		Facility:  "一号温室", Owner: "运行一组", Category: "果菜", RiskLevel: "medium",
		ZoneCode: zoneCode, StopMoistureLine: stopLine,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
}

func (f *preStartFixture) seedReading(t *testing.T, code, zoneCode, status string, moisture float64, measuredAt time.Time) {
	t.Helper()
	if err := f.readings.Create(f.ctx, &model.SoilReading{
		BaseModel: model.BaseModel{Code: code, Name: "墒情读数" + code, Status: status, Version: 1},
		Facility:  "一号温室", Owner: "运行一组", Category: "果菜", RiskLevel: "low",
		MetricValue: moisture, MetricUnit: "%", EffectiveAt: measuredAt, ZoneCode: zoneCode,
	}); err != nil {
		t.Fatalf("create reading: %v", err)
	}
}

func (f *preStartFixture) createRequestedExecution(t *testing.T, code, zoneCode, planCode string) model.ValveExecution {
	t.Helper()
	created, err := f.svc.Create(f.ctx, dto.CreateValveExecution{
		Code: code, Name: "待启动执行" + code, Facility: "一号温室", Owner: "运行一组",
		Category: "果菜", RiskLevel: "high", MetricValue: 18, MetricUnit: "L/min",
		EffectiveAt: f.now, Evidence: "阀门连通性已测", ZoneCode: zoneCode, PlanCode: planCode,
	}, "operator", "req-create")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	requested, err := f.svc.RequestControl(f.ctx, created.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: created.Version, Confirmed: true, Reason: "operator checked zone and valve",
	}, "operator", "req-request")
	if err != nil {
		t.Fatalf("request control: %v", err)
	}
	return requested
}

func (f *preStartFixture) confirm(t *testing.T, execution model.ValveExecution) model.ValveExecution {
	t.Helper()
	result, err := f.svc.ConfirmControl(f.ctx, execution.ID, dto.ControlConfirmationRequest{
		ExpectedVersion: execution.Version, Confirmed: true, Reason: "reviewer independent confirmation",
	}, "reviewer", "req-confirm")
	if err != nil {
		t.Fatalf("confirm control: %v", err)
	}
	return result
}

func conflictKinds(snapshot *model.ControlCheckSnapshot) map[string]bool {
	kinds := make(map[string]bool)
	for _, conflict := range snapshot.Conflicts {
		kinds[conflict.Kind] = true
	}
	return kinds
}

func TestConfirmBlockedByRunningTaskSameZone(t *testing.T) {
	f := newPreStartFixture(t)
	f.seedZone(t, "GZ-100")
	f.seedPlan(t, "IP-100", "GZ-100", 35, "scheduled")
	f.seedReading(t, "SR-100", "GZ-100", "validated", 22.0, f.now.Add(-10*time.Minute))

	runningStartedAt := f.now.Add(-20 * time.Minute)
	if err := f.execs.Create(f.ctx, &model.ValveExecution{
		BaseModel: model.BaseModel{Code: "VE-RUN", Name: "同分区运行中任务", Status: "running", Version: 1},
		Facility:  "一号温室", Owner: "运行一组", Category: "果菜", RiskLevel: "high",
		ZoneCode: "GZ-100", PlanCode: "IP-100", ControlCheckStatus: "passed",
		ControlConfirmedBy: "reviewer", ControlConfirmedAt: &runningStartedAt,
	}); err != nil {
		t.Fatalf("create running execution: %v", err)
	}

	target := f.createRequestedExecution(t, "VE-100", "GZ-100", "IP-100")
	result := f.confirm(t, target)
	if result.Status != "planned" {
		t.Fatalf("blocked execution must stay planned, got %s", result.Status)
	}
	if result.ControlCheckStatus != "blocked" || result.ControlConflictNo == "" {
		t.Fatalf("expected blocked check with conflict number, got status=%s no=%s", result.ControlCheckStatus, result.ControlConflictNo)
	}
	snapshot, err := result.Snapshot()
	if err != nil || snapshot == nil {
		t.Fatalf("snapshot must be persisted: %v", err)
	}
	if !conflictKinds(snapshot)[model.ControlConflictRunningTask] {
		t.Fatalf("expected running task conflict, got %+v", snapshot.Conflicts)
	}
	if len(snapshot.RunningTasks) != 1 || snapshot.RunningTasks[0].Code != "VE-RUN" {
		t.Fatalf("expected frozen running task VE-RUN, got %+v", snapshot.RunningTasks)
	}
	if snapshot.CheckedBy != "reviewer" {
		t.Fatalf("snapshot must record reviewer, got %q", snapshot.CheckedBy)
	}
}

func TestConfirmBlockedByStaleReading(t *testing.T) {
	f := newPreStartFixture(t)
	f.seedZone(t, "GZ-200")
	f.seedPlan(t, "IP-200", "GZ-200", 38, "scheduled")
	f.seedReading(t, "SR-200", "GZ-200", "validated", 24.0, f.now.Add(-45*time.Minute))

	target := f.createRequestedExecution(t, "VE-200", "GZ-200", "IP-200")
	result := f.confirm(t, target)
	if result.Status != "planned" {
		t.Fatalf("stale reading must keep execution planned, got %s", result.Status)
	}
	snapshot, _ := result.Snapshot()
	if !conflictKinds(snapshot)[model.ControlConflictReadingStale] {
		t.Fatalf("expected stale reading conflict, got %+v", snapshot.Conflicts)
	}
	stale := snapshot.Conflicts[0]
	if stale.ReadingCode != "SR-200" || stale.Moisture != 24.0 || stale.ReadingTime.IsZero() {
		t.Fatalf("conflict must carry reading code/time/moisture, got %+v", stale)
	}
	if snapshot.LatestReading == nil || snapshot.LatestReading.AgeMinutes < int(constants.PreStartReadingFreshness/time.Minute) {
		t.Fatalf("frozen reading must show age beyond freshness window, got %+v", snapshot.LatestReading)
	}
}

func TestConfirmBlockedAboveStopMoistureLine(t *testing.T) {
	f := newPreStartFixture(t)
	f.seedZone(t, "GZ-300")
	f.seedPlan(t, "IP-300", "GZ-300", 40, "approved")
	f.seedReading(t, "SR-300", "GZ-300", "validated", 45.2, f.now.Add(-8*time.Minute))

	target := f.createRequestedExecution(t, "VE-300", "GZ-300", "IP-300")
	result := f.confirm(t, target)
	if result.Status != "planned" {
		t.Fatalf("above-stop-line reading must keep execution planned, got %s", result.Status)
	}
	snapshot, _ := result.Snapshot()
	if !conflictKinds(snapshot)[model.ControlConflictAboveStopMoisture] {
		t.Fatalf("expected above stop moisture conflict, got %+v", snapshot.Conflicts)
	}
	if snapshot.StopMoistureLine != 40 {
		t.Fatalf("snapshot must freeze stop line 40, got %v", snapshot.StopMoistureLine)
	}
}

func TestConfirmBlockedWithoutValidatedReading(t *testing.T) {
	f := newPreStartFixture(t)
	f.seedZone(t, "GZ-400")
	f.seedPlan(t, "IP-400", "GZ-400", 35, "scheduled")
	f.seedReading(t, "SR-400", "GZ-400", "fresh", 22.0, f.now.Add(-5*time.Minute))

	target := f.createRequestedExecution(t, "VE-400", "GZ-400", "IP-400")
	result := f.confirm(t, target)
	if result.Status != "planned" {
		t.Fatalf("missing validated reading must keep execution planned, got %s", result.Status)
	}
	snapshot, _ := result.Snapshot()
	if !conflictKinds(snapshot)[model.ControlConflictReadingMissing] {
		t.Fatalf("expected missing reading conflict, got %+v", snapshot.Conflicts)
	}
}

func TestConfirmBlockedByPlanZoneMismatch(t *testing.T) {
	f := newPreStartFixture(t)
	f.seedZone(t, "GZ-500")
	f.seedZone(t, "GZ-OTHER")
	f.seedPlan(t, "IP-500", "GZ-OTHER", 35, "scheduled")
	f.seedReading(t, "SR-500", "GZ-500", "validated", 22.0, f.now.Add(-5*time.Minute))

	if _, err := f.svc.Create(f.ctx, dto.CreateValveExecution{
		Code: "VE-500", Name: "分区错配执行", Facility: "一号温室", Owner: "运行一组",
		Category: "果菜", RiskLevel: "high", EffectiveAt: f.now, ZoneCode: "GZ-500", PlanCode: "IP-500",
	}, "operator", "req-mismatch"); err == nil {
		t.Fatal("create must reject a plan targeting another zone")
	}
}

func TestConfirmPassesAndFreezesSnapshot(t *testing.T) {
	f := newPreStartFixture(t)
	f.seedZone(t, "GZ-600")
	f.seedPlan(t, "IP-600", "GZ-600", 35, "scheduled")
	f.seedReading(t, "SR-600", "GZ-600", "validated", 22.4, f.now.Add(-10*time.Minute))

	target := f.createRequestedExecution(t, "VE-600", "GZ-600", "IP-600")
	result := f.confirm(t, target)
	if result.Status != "running" {
		t.Fatalf("clean check must start the valve, got %s", result.Status)
	}
	if result.ControlConfirmedBy != "reviewer" || result.ControlCheckStatus != "passed" {
		t.Fatalf("expected reviewer-confirmed passed check, got %+v", result)
	}
	snapshot, err := result.Snapshot()
	if err != nil || snapshot == nil {
		t.Fatalf("passing check must persist snapshot: %v", err)
	}
	if snapshot.Status != "passed" || len(snapshot.Conflicts) != 0 {
		t.Fatalf("expected passed snapshot without conflicts, got %+v", snapshot)
	}
	if snapshot.LatestReading == nil || snapshot.LatestReading.Moisture != 22.4 {
		t.Fatalf("snapshot must freeze latest validated reading, got %+v", snapshot.LatestReading)
	}
}

func TestBlockedExecutionCanPassAfterConflictResolved(t *testing.T) {
	f := newPreStartFixture(t)
	f.seedZone(t, "GZ-700")
	f.seedPlan(t, "IP-700", "GZ-700", 35, "scheduled")
	f.seedReading(t, "SR-STALE", "GZ-700", "validated", 24.0, f.now.Add(-45*time.Minute))

	target := f.createRequestedExecution(t, "VE-700", "GZ-700", "IP-700")
	blocked := f.confirm(t, target)
	if blocked.Status != "planned" || blocked.ControlCheckStatus != "blocked" {
		t.Fatalf("expected blocked planned execution, got %s/%s", blocked.Status, blocked.ControlCheckStatus)
	}

	// Night shift refreshes the validated reading after re-measuring the zone.
	f.seedReading(t, "SR-FRESH", "GZ-700", "validated", 23.1, f.now.Add(-3*time.Minute))
	passed := f.confirm(t, blocked)
	if passed.Status != "running" || passed.ControlCheckStatus != "passed" {
		t.Fatalf("re-check with fresh reading must start the valve, got %s/%s", passed.Status, passed.ControlCheckStatus)
	}
	snapshot, _ := passed.Snapshot()
	if snapshot.LatestReading == nil || snapshot.LatestReading.Code != "SR-FRESH" {
		t.Fatalf("passed snapshot must reference the fresh reading, got %+v", snapshot.LatestReading)
	}
}

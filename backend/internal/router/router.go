package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/config"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/handler"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/middleware"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(middleware.RequestContext(logger))
	engine.Use(middleware.RequestTimeout(15 * time.Second))
	if len(cfg.TrustedProxies) > 0 {
		_ = engine.SetTrustedProxies(cfg.TrustedProxies)
	} else {
		_ = engine.SetTrustedProxies(nil)
	}

	securityRepository := repository.NewSecurityRepository(db)
	securityService := service.NewSecurityService(securityRepository, cfg)
	greenhouseZoneRepository := repository.NewGreenhouseZoneRepository(db)
	soilReadingRepository := repository.NewSoilReadingRepository(db)
	irrigationPlanRepository := repository.NewIrrigationPlanRepository(db)
	valveExecutionRepository := repository.NewValveExecutionRepository(db)
	greenhouseZoneService := service.NewGreenhouseZoneService(greenhouseZoneRepository, securityService)
	soilReadingService := service.NewSoilReadingService(soilReadingRepository, securityService)
	irrigationPlanService := service.NewIrrigationPlanService(irrigationPlanRepository, securityService)
	valveExecutionService := service.NewValveExecutionService(valveExecutionRepository, greenhouseZoneRepository, irrigationPlanRepository, soilReadingRepository, securityService)
	greenhouseZoneHandler := handler.NewGreenhouseZoneHandler(greenhouseZoneService)
	soilReadingHandler := handler.NewSoilReadingHandler(soilReadingService)
	irrigationPlanHandler := handler.NewIrrigationPlanHandler(irrigationPlanService)
	valveExecutionHandler := handler.NewValveExecutionHandler(valveExecutionService)
	systemHandler := handler.NewSystemHandler(securityService, greenhouseZoneService, soilReadingService, irrigationPlanService, valveExecutionService, db, redisClient)

	engine.GET("/healthz", systemHandler.Health)
	engine.POST("/api/auth/login", systemHandler.Login)

	limiter := middleware.NewLimiter(redisClient, cfg.RequestLimit)
	api := engine.Group("/api")
	api.Use(limiter.Middleware(), middleware.Authenticate(cfg))
	api.GET("/overview", systemHandler.Overview)
	api.GET("/audits", middleware.RequireMinimumRole("reviewer"), systemHandler.Audits)
	api.GET("/session", systemHandler.Session)
	api.GET("/runtime", middleware.RequireMinimumRole("reviewer"), systemHandler.Runtime)
	api.GET("/audit-summary", middleware.RequireMinimumRole("reviewer"), systemHandler.AuditSummary)
	api.GET("/audits/:entityType/:id", middleware.RequireMinimumRole("reviewer"), systemHandler.EntityHistory)
	greenhouseZoneHandler.Register(api)
	soilReadingHandler.Register(api)
	irrigationPlanHandler.Register(api)
	valveExecutionHandler.Register(api)

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "route_not_found"})
	})
	return engine
}

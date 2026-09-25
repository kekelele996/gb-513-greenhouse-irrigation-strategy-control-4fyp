package handler

import (
	"net/http"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/middleware"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/service"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/util"
	"github.com/gin-gonic/gin"
)

type ValveExecutionHandler struct{ service service.ValveExecutionService }

func NewValveExecutionHandler(s service.ValveExecutionService) *ValveExecutionHandler {
	return &ValveExecutionHandler{service: s}
}

func (h *ValveExecutionHandler) Register(group *gin.RouterGroup) {
	resource := group.Group("/executions")
	resource.GET("", h.list)
	resource.GET("/:id", h.get)
	resource.GET("/:id/control-detail", h.controlDetail)
	resource.POST("", middleware.RequireRoles(model.RoleOperator, model.RoleAdmin), h.create)
	resource.PUT("/:id", middleware.RequireRoles(model.RoleOperator, model.RoleAdmin), h.update)
	resource.POST("/:id/transition", middleware.RequireRoles(model.RoleOperator, model.RoleReviewer, model.RoleAdmin), h.transition)
	resource.POST("/:id/control-request", middleware.RequireRoles(model.RoleOperator, model.RoleAdmin), h.requestControl)
	resource.POST("/:id/control-confirm", middleware.RequireRoles(model.RoleReviewer, model.RoleAdmin), h.confirmControl)
	resource.DELETE("/:id", middleware.RequireRoles(model.RoleAdmin), h.remove)
}

func (h *ValveExecutionHandler) list(c *gin.Context) {
	query := bindPage(c)
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}
	util.Page(c, result.Items, result.Page, result.PageSize, result.Total)
}

func (h *ValveExecutionHandler) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *ValveExecutionHandler) controlDetail(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	detail, err := h.service.ControlDetail(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	// util.OK wraps the payload as {data: ...}; unwrap the DTO fields so the
	// contract stays {execution, snapshot, live} instead of a nested data key.
	util.OK(c, gin.H{
		"execution": detail.Execution,
		"snapshot":  detail.Snapshot,
		"live":      detail.Live,
	})
}

func (h *ValveExecutionHandler) create(c *gin.Context) {
	var input dto.CreateValveExecution
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Create(c.Request.Context(), input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.Created(c, item)
}

func (h *ValveExecutionHandler) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.UpdateValveExecution
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *ValveExecutionHandler) transition(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.TransitionRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Transition(c.Request.Context(), id, input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *ValveExecutionHandler) requestControl(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.ControlConfirmationRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.RequestControl(c.Request.Context(), id, input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *ValveExecutionHandler) confirmControl(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.ControlConfirmationRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.ConfirmControl(c.Request.Context(), id, input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *ValveExecutionHandler) remove(c *gin.Context) {
	if roleFromContext(c) != "admin" {
		util.Fail(c, http.StatusForbidden, "forbidden", "admin role is required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id, actorFromContext(c), requestIDFromContext(c)); err != nil {
		handleError(c, err)
		return
	}
	util.NoContent(c)
}

package handler

import (
	"errors"
	"net/http"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/service"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		util.Fail(c, http.StatusNotFound, "not_found", "record was not found")
	case errors.Is(err, repository.ErrVersionConflict):
		util.Fail(c, http.StatusConflict, "version_conflict", "record changed; refresh and retry")
	case errors.Is(err, service.ErrInvalidTransition), errors.Is(err, service.ErrInvalidInput),
		errors.Is(err, service.ErrDualConfirmation), errors.Is(err, service.ErrControlRequested),
		errors.Is(err, service.ErrControlNotRequested), errors.Is(err, service.ErrSelfConfirmation),
		errors.Is(err, service.ErrExplicitConfirmation),
		errors.Is(err, service.ErrZoneNotFound), errors.Is(err, service.ErrPlanNotFound),
		errors.Is(err, service.ErrPlanZoneMismatch), errors.Is(err, service.ErrPlanMissingStopLine):
		util.Fail(c, http.StatusUnprocessableEntity, "business_rule", err.Error())
	default:
		_ = c.Error(err)
		util.Fail(c, http.StatusInternalServerError, "internal_error", "request could not be completed")
	}
}

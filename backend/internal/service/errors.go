package service

import "errors"

var (
	ErrInvalidTransition    = errors.New("requested status transition is not allowed")
	ErrInvalidInput         = errors.New("business input validation failed")
	ErrUnauthorized         = errors.New("invalid username or password")
	ErrInactiveUser         = errors.New("user account is inactive")
	ErrDualConfirmation     = errors.New("remote valve start requires request and independent confirmation")
	ErrControlRequested     = errors.New("remote valve start is already awaiting confirmation")
	ErrControlNotRequested  = errors.New("remote valve start has not been requested")
	ErrSelfConfirmation     = errors.New("requester cannot confirm their own remote valve start")
	ErrExplicitConfirmation = errors.New("explicit confirmation is required for remote valve start")
)

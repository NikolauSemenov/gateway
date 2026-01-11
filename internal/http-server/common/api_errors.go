package common

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type ErrorResponse struct {
	Detail string `json:"detail"`
}

func WriteError(c echo.Context, status int, detail string) error {
	_ = c.JSON(status, ErrorResponse{Detail: detail})
	return nil
}

func WriteBadRequest(c echo.Context, detail string) error {
	return WriteError(c, http.StatusBadRequest, detail)
}

func WriteUnauthorized(c echo.Context, detail string) error {
	return WriteError(c, http.StatusUnauthorized, detail)
}

func WriteForbidden(c echo.Context, detail string) error {
	return WriteError(c, http.StatusForbidden, detail)
}

func WriteNotFound(c echo.Context, detail string) error {
	return WriteError(c, http.StatusNotFound, detail)
}

func WriteServiceUnavailable(c echo.Context, detail string) error {
	return WriteError(c, http.StatusServiceUnavailable, detail)
}

func UpstreamError(c echo.Context, upstreamStatus int, fallbackStatus int, detail string) error {
	if upstreamStatus >= 100 && upstreamStatus <= 599 {
		return WriteError(c, upstreamStatus, detail)
	}
	if fallbackStatus <= 0 {
		fallbackStatus = http.StatusBadGateway
	}
	return WriteError(c, fallbackStatus, detail)
}

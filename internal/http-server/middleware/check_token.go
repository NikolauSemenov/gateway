package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"gateway/external/httpClient"
	"gateway/internal/http-server/common"

	"github.com/labstack/echo/v4"
)

type VerifyUserResponse struct {
	UserID string `json:"user_id"`
}

const UserIDContextKey = "user_id"

type Doer interface {
	MakeRequest(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error)
}

func JWTAuthMiddleware(client Doer, authBaseURL string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			log := LoggerFromContext(c, slog.Default())

			token := c.Request().Header.Get("Authorization")
			if token == "" {
				log.Warn("jwt auth denied: missing token")
				return common.WriteUnauthorized(c, "Missing token")
			}

			var resp VerifyUserResponse

			headers := http.Header{}
			headers.Set("Authorization", token)

			targetURL := authBaseURL + "/v1/verify"
			httpRes, err := client.MakeRequest(
				c.Request().Context(),
				httpClient.RequestOptions{
					Method:   http.MethodGet,
					URL:      targetURL,
					Headers:  headers,
					Response: &resp,
				},
			)
			if err != nil {
				if httpRes != nil {
					log.Warn("jwt auth denied: auth service returned error",
						"status", httpRes.StatusCode,
						"err", err,
					)
					return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
				}
				log.Error("jwt auth failed: auth service unavailable", "err", err)
				return common.WriteServiceUnavailable(c, "Auth service unavailable")
			}

			if resp.UserID == "" {
				log.Warn("jwt auth denied: invalid user (empty user_id from auth service)")
				return common.WriteUnauthorized(c, "Invalid user")
			}

			c.Set(UserIDContextKey, resp.UserID)
			log.Debug("jwt auth ok", "user_id", resp.UserID)

			return next(c)
		}
	}
}

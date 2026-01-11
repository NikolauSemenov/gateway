package middleware

import (
	"gateway/external/httpClient"
	"net/http"

	"github.com/labstack/echo/v4"
)

type VerifyUserResponse struct {
	UserID string `json:"user_id"`
}

const UserIDContextKey = "user_id"

func JWTAuthMiddleware(client *httpClient.APIClient, authBaseURL string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := c.Request().Header.Get("Authorization")
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing token"})
			}

			var resp VerifyUserResponse

			headers := http.Header{}
			headers.Set("Authorization", token)
			httpRes, err := client.MakeRequest(
				c.Request().Context(),
				httpClient.RequestOptions{
					Method:   http.MethodGet,
					URL:      authBaseURL + "/v1/verify",
					Headers:  headers,
					Response: &resp,
				},
			)
			if err != nil {
				if httpRes != nil {
					return c.JSON(httpRes.StatusCode, err.Error())
				}
				return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
			}

			if resp.UserID == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid user"})
			}
			c.Set(UserIDContextKey, resp.UserID)

			return next(c)
		}
	}
}

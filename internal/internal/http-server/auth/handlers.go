package auth

import (
	"gateway/external/httpClient"
	"gateway/internal/config"
	"net/http"

	"github.com/labstack/echo/v4"
)

type Auth struct {
	clientHttp *httpClient.APIClient
	config     *config.Config
}

func NewAuth(config *config.Config, clientHttp *httpClient.APIClient) *Auth {
	return &Auth{
		config:     config,
		clientHttp: clientHttp,
	}
}

// SendEmailCode обрабатывает запрос на /send-email-code
// @Summary Отправка кода на email
// @Description Проксирует запрос по пути /send-email-code
// @Tags auth
// @Param request body SendEmailCodeRequest true "Данные запроса"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Неверный запрос"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/auth/send-email-code [post]
func (a *Auth) SendEmailCode(c echo.Context) error {
	var req SendEmailCodeRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method: http.MethodPost,
			URL:    a.config.BaseUrl + a.config.V1SendEmailCode,
			Body:   req,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}
	return c.JSON(httpRes.StatusCode, "Запрос на отправку кода отправлен")
}

// VerifyEmailCode обрабатывает запрос на /verify-email-code
// @Summary Проверка кода, отправленного на email
// @Description Проксирует запрос по пути /verify-email-code
// @Tags auth
// @Param request body VerifyEmailCodeRequest true "Данные запроса"
// @Success 200 {object} VerifyEmailCodeResponse "OK"
// @Failure 400 {string} string "Неверный запрос"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/auth/verify-email-code [put]
func (a *Auth) VerifyEmailCode(c echo.Context) error {
	var req VerifyEmailCodeRequest
	var resp VerifyEmailCodeResponse

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}
	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodPut,
			URL:      a.config.BaseUrl + a.config.V1VerifyEmail,
			Body:     req,
			Response: &resp,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}
	return c.JSON(httpRes.StatusCode, resp)
}

// Register обрабатывает запрос на /register
// @Summary Регистрация нового пользователя
// @Description Проксирует запрос по пути /register
// @Tags auth
// @Param request body RegisterRequest true "Данные запроса"
// @Success 201 {string} string "Created"
// @Failure 400 {string} string "Неверный запрос"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/auth/register [post]
func (a *Auth) Register(c echo.Context) error {
	var req RegisterRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method: http.MethodPost,
			URL:    a.config.BaseUrl + a.config.V1Register,
			Body:   req,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}
	return c.JSON(httpRes.StatusCode, "Запрос на регистрацию отправлен")
}

// Login обрабатывает запрос на /login
// @Summary Аутентификация пользователя
// @Description Проиксирует запрос по пути /login
// @Tags auth
// @Param request body LoginRequest true "Данные запроса"
// @Success 200 {object} LoginResponse "OK"
// @Failure 400 {string} string "Неверный запрос"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/auth/login [post]
func (a *Auth) Login(c echo.Context) error {
	var req LoginRequest
	var resp LoginResponse

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodPost,
			URL:      a.config.BaseUrl + a.config.V1Login,
			Body:     req,
			Response: &resp,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}
	return c.JSON(httpRes.StatusCode, resp)
}

// Verify обрабатывает запрос на /verify
// @Summary Проверка верификации пользователя
// @Description Проксирует запрос по пути /verify
// @Tags auth
// @Param Authorization header string true "Bearer token"
// @Success 200 {string} string "OK"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/auth/verify [get]
func (a *Auth) Verify(c echo.Context) error {
	headers := http.Header{}
	headers.Set("Authorization", c.Request().Header.Get("Authorization"))

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:  http.MethodGet,
			URL:     a.config.BaseUrl + a.config.V1Verify,
			Headers: headers,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}
	return c.JSON(httpRes.StatusCode, nil)
}

// Logout обрабатывает запрос на /logout
// @Summary Выход пользователя
// @Description Проксирует запрос по пути /logout
// @Tags auth
// @Param Authorization header string true "Bearer token"
// @Success 204 {string} string "OK"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/auth/logout [delete]
func (a *Auth) Logout(c echo.Context) error {

	headers := http.Header{}
	headers.Set("Authorization", c.Request().Header.Get("Authorization"))

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:  http.MethodGet,
			URL:     a.config.BaseUrl + a.config.V1Verify,
			Headers: headers,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}
	return c.JSON(httpRes.StatusCode, nil)
}

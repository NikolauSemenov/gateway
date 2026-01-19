package auth

import (
	"context"
	"net/http"
	"strings"

	"gateway/external/httpClient"
	"gateway/internal/config"
	"gateway/internal/http-server/common"
	myMiddleware "gateway/internal/http-server/middleware"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type Doer interface {
	MakeRequest(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error)
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type Auth struct {
	clientHttp Doer
	config     *config.Config
	log        Logger

	redis *redis.Client
}

func NewAuth(cfg *config.Config, client Doer, log Logger, rdb *redis.Client) *Auth {
	return &Auth{
		config:     cfg,
		clientHttp: client,
		log:        log,
		redis:      rdb,
	}
}

func (a *Auth) authURL(path string) string {
	base := strings.TrimRight(a.config.BaseUrl, "/")
	p := strings.TrimSpace(path)
	if p == "" {
		return base
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return base + p
}

func (a *Auth) authHeader(c echo.Context) http.Header {
	h := http.Header{}
	token := strings.TrimSpace(c.Request().Header.Get("Authorization"))
	if token != "" {
		h.Set("Authorization", token)
	}
	return h
}

func (a *Auth) bindJSON(c echo.Context, dst any) bool {
	if err := c.Bind(dst); err != nil {
		LoggerFromEcho(c, a.log).Warn("failed to bind request body", logKeyErr, err)
		_ = common.WriteBadRequest(c, msgInvalidBody)
		return false
	}
	return true
}

func (a *Auth) doUpstream(c echo.Context, opts httpClient.RequestOptions, op string) (*http.Response, error) {
	log := LoggerFromEcho(c, a.log)

	httpRes, err := a.clientHttp.MakeRequest(c.Request().Context(), opts)
	if err != nil {
		if httpRes != nil {
			log.Warn("auth upstream returned error",
				"op", op,
				logKeyUpstreamURL, opts.URL,
				logKeyStatus, httpRes.StatusCode,
				logKeyErr, err,
			)
			return httpRes, err
		}

		log.Error("auth upstream unavailable",
			"op", op,
			logKeyUpstreamURL, opts.URL,
			logKeyErr, err,
		)
		return nil, err
	}

	log.Info("auth request proxied successfully",
		"op", op,
		logKeyUpstreamURL, opts.URL,
		logKeyStatus, httpRes.StatusCode,
		logKeyAuthorizationHint, strings.TrimSpace(c.Request().Header.Get("Authorization")) != "",
	)

	return httpRes, nil
}

// ---- handlers ----

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
	if ok := a.bindJSON(c, &req); !ok {
		return nil
	}

	opts := httpClient.RequestOptions{
		Method: http.MethodPost,
		URL:    a.authURL(a.config.V1SendEmailCode),
		Body:   req, // keep value type consistent with prior code/tests
	}

	httpRes, err := a.doUpstream(c, opts, "send_email_code")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgAuthUnavailable)
	}

	return c.JSON(httpRes.StatusCode, msgEmailCodeSent)
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
	if ok := a.bindJSON(c, &req); !ok {
		return nil
	}

	var resp VerifyEmailCodeResponse
	opts := httpClient.RequestOptions{
		Method:   http.MethodPut,
		URL:      a.authURL(a.config.V1VerifyEmail),
		Body:     req,
		Response: &resp,
	}

	httpRes, err := a.doUpstream(c, opts, "verify_email_code")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgAuthUnavailable)
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
	if ok := a.bindJSON(c, &req); !ok {
		return nil
	}

	opts := httpClient.RequestOptions{
		Method: http.MethodPost,
		URL:    a.authURL(a.config.V1Register),
		Body:   req,
	}

	httpRes, err := a.doUpstream(c, opts, "register")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgAuthUnavailable)
	}

	return c.JSON(httpRes.StatusCode, msgRegistrationSent)
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
	if ok := a.bindJSON(c, &req); !ok {
		return nil
	}

	var resp LoginResponse
	opts := httpClient.RequestOptions{
		Method:   http.MethodPost,
		URL:      a.authURL(a.config.V1Login),
		Body:     req,
		Response: &resp,
	}

	httpRes, err := a.doUpstream(c, opts, "login")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgAuthUnavailable)
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
	opts := httpClient.RequestOptions{
		Method:  http.MethodGet,
		URL:     a.authURL(a.config.V1Verify),
		Headers: a.authHeader(c),
	}

	httpRes, err := a.doUpstream(c, opts, "verify")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgAuthUnavailable)
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
	// 1) Call upstream logout endpoint.
	opts := httpClient.RequestOptions{
		Method:  http.MethodDelete,
		URL:     a.authURL(a.config.V1Logout),
		Headers: a.authHeader(c),
	}

	httpRes, err := a.doUpstream(c, opts, "logout")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgAuthUnavailable)
	}

	// 2) Invalidate JWT verify cache (best-effort) so token isn't treated as valid by the gateway cache.
	_ = myMiddleware.InvalidateJWTVerifyCache(
		c.Request().Context(),
		a.redis,
		strings.TrimSpace(c.Request().Header.Get("Authorization")),
	)

	return c.NoContent(httpRes.StatusCode)
}

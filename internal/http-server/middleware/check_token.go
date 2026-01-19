package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gateway/external/httpClient"
	"gateway/internal/http-server/common"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type VerifyUserResponse struct {
	UserID string `json:"user_id"`
}

type verifyCacheEntry struct {
	UserID string `json:"user_id"`
}

const UserIDContextKey = "user_id"

type Doer interface {
	MakeRequest(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error)
}

var verifySF singleflight.Group

func JWTAuthMiddleware(client Doer, authBaseURL string) echo.MiddlewareFunc {
	// Default behavior (used by tests): call {baseURL}/v1/verify and do NOT use Redis cache.
	verifyURL := strings.TrimRight(strings.TrimSpace(authBaseURL), "/") + "/v1/verify"
	return JWTAuthMiddlewareWithRedis(client, verifyURL, nil, 0)
}

// JWTAuthMiddlewareWithRedis verifies JWT via auth service and caches successful verifications in Redis.
// - Cache key is derived from a SHA-256 of the token (token itself is never stored).
// - On cache hit, the middleware does not call the auth service.
// - TTL: if <= 0, caching is effectively disabled (but still supports singleflight).
// - verifyURL: MUST be the full URL to the verify endpoint (e.g. "http://auth:80/auth/api/v1/verify").
//
// IMPORTANT: The middleware must return exactly one response on error. Therefore the singleflight closure MUST NOT
// write to the response (no common.Write* calls inside), it should only return a typed error.
func JWTAuthMiddlewareWithRedis(client Doer, verifyURL string, rdb *redis.Client, ttl time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			log := LoggerFromContext(c, slog.Default())

			authz := strings.TrimSpace(c.Request().Header.Get("Authorization"))
			if authz == "" {
				log.Warn("jwt auth denied: missing token")
				return common.WriteUnauthorized(c, "Missing token")
			}

			bearer, err := parseBearer(authz)
			if err != nil {
				log.Warn("jwt auth denied: invalid authorization header", "err", err)
				return common.WriteUnauthorized(c, "Invalid token")
			}

			// 1) Redis cache lookup
			if rdb != nil && ttl > 0 {
				if uid, ok := cacheGetUserID(c.Request().Context(), rdb, bearer); ok && uid != "" {
					c.Set(UserIDContextKey, uid)
					log.Debug("jwt auth ok (cache)", "user_id", uid)
					return next(c)
				}
			}

			// 2) Singleflight + upstream verify (no response writes inside the closure!)
			v, verr, _ := verifySF.Do(cacheKeyForToken(bearer)+":verify", func() (any, error) {
				var resp VerifyUserResponse

				headers := http.Header{}
				// Preserve original Authorization header value for upstream calls (tests expect this).
				headers.Set("Authorization", authz)

				targetURL := strings.TrimSpace(verifyURL)

				httpRes, reqErr := client.MakeRequest(
					c.Request().Context(),
					httpClient.RequestOptions{
						Method:   http.MethodGet,
						URL:      targetURL,
						Headers:  headers,
						Response: &resp,
					},
				)
				if reqErr != nil {
					// Preserve 401/403 semantics AND preserve upstream body message for tests.
					if httpRes != nil {
						if httpRes.StatusCode == http.StatusUnauthorized || httpRes.StatusCode == http.StatusForbidden {
							return "", upstreamStatusErr{status: httpRes.StatusCode, detail: reqErr.Error()}
						}
						return "", errAuthUnavailable
					}
					return "", errAuthUnavailable
				}

				if resp.UserID == "" {
					return "", errInvalidUser
				}

				// 3) Cache success
				if rdb != nil && ttl > 0 {
					_ = cacheSetUserID(c.Request().Context(), rdb, bearer, resp.UserID, ttl)
				}

				return resp.UserID, nil
			})

			if verr != nil {
				var use upstreamStatusErr
				if errors.As(verr, &use) {
					// For upstream 401/403, preserve upstream error body as "detail".
					if use.status == http.StatusUnauthorized || use.status == http.StatusForbidden {
						return common.WriteError(c, use.status, use.detail)
					}
				}

				switch {
				case errors.Is(verr, errInvalidUser):
					log.Warn("jwt auth denied: invalid user", "err", verr)
					return common.WriteUnauthorized(c, "Invalid user")
				case errors.Is(verr, errAuthUnavailable):
					log.Error("jwt auth failed: auth service unavailable", "err", verr)
					return common.WriteServiceUnavailable(c, "Auth service unavailable")
				default:
					log.Error("jwt auth failed: unexpected error", "err", verr)
					return common.WriteServiceUnavailable(c, "Auth service unavailable")
				}
			}

			uid, _ := v.(string)
			if uid == "" {
				log.Warn("jwt auth denied: empty user_id after verify")
				return common.WriteUnauthorized(c, "Invalid user")
			}

			c.Set(UserIDContextKey, uid)
			log.Debug("jwt auth ok", "user_id", uid)

			return next(c)
		}
	}
}

type upstreamStatusErr struct {
	status int
	detail string
}

func (e upstreamStatusErr) Error() string { return e.detail }

var (
	errInvalidUser     = errors.New("invalid user")
	errAuthUnavailable = errors.New("auth unavailable")
)

func InvalidateJWTVerifyCache(ctx context.Context, rdb *redis.Client, authorizationHeader string) error {
	if rdb == nil {
		return nil
	}
	authz := strings.TrimSpace(authorizationHeader)
	if authz == "" {
		return nil
	}
	bearer, err := parseBearer(authz)
	if err != nil || bearer == "" {
		return nil
	}
	return rdb.Del(ctx, cacheKeyForToken(bearer)).Err()
}

func parseBearer(authorization string) (string, error) {
	authorization = strings.TrimSpace(authorization)
	if authorization == "" {
		return "", errors.New("empty authorization")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) {
		return "", errors.New("missing Bearer prefix")
	}
	tok := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
	if tok == "" {
		return "", errors.New("empty bearer token")
	}
	// Basic hardening: prevent absurdly large headers from being used as cache keys.
	if len(tok) > 8192 {
		return "", errors.New("token too large")
	}
	return tok, nil
}

func cacheKeyForToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "jwtverify:" + hex.EncodeToString(sum[:])
}

func cacheGetUserID(ctx context.Context, rdb *redis.Client, token string) (string, bool) {
	b, err := rdb.Get(ctx, cacheKeyForToken(token)).Bytes()
	if err != nil {
		return "", false
	}
	var e verifyCacheEntry
	if err := json.Unmarshal(b, &e); err != nil {
		return "", false
	}
	return e.UserID, e.UserID != ""
}

func cacheSetUserID(ctx context.Context, rdb *redis.Client, token string, userID string, ttl time.Duration) error {
	b, err := json.Marshal(verifyCacheEntry{UserID: userID})
	if err != nil {
		return err
	}
	return rdb.Set(ctx, cacheKeyForToken(token), b, ttl).Err()
}

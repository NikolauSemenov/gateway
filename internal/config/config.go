package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	LogLevel string `env:"LOG_LEVEL"`
	HTTPServer
	AuthConfig
	NoteFlowConfig
	RedisConfig
	CORSConfig
	SwaggerConfig
}

type AuthConfig struct {
	BaseUrl         string `env:"AUTH_BASE_URL"`
	V1Register      string `env:"AUTH_V1_REGISTER"`
	V1Login         string `env:"AUTH_V1_LOGIN"`
	V1Verify        string `env:"AUTH_V1_VERIFY"`
	V1Logout        string `env:"AUTH_V1_LOGOUT"`
	V1SendEmailCode string `env:"AUTH_V1_SEND_EMAIL_CODE"`
	V1VerifyEmail   string `env:"AUTH_V1_VERIFY_EMAIL"`
}

type NoteFlowConfig struct {
	BaseUrlNotes  string `env:"NOTE_FLOW_BASE_URL"`
	V1Notes       string `env:"NOTE_FLOW_V1_NOTES"`
	V1Note        string `env:"NOTE_FLOW_V1_NOTE"`
	V1NoteActive  string `env:"NOTE_FLOW_V1_NOTE_ACTIVE"`
	V1NoteLike    string `env:"NOTE_FLOW_V1_NOTE_LIKE"`
	V1NoteComment string `env:"NOTE_FLOW_V1_NOTE_COMMENT"`
}

type HTTPServer struct {
	Address     string        `env:"ADDRESS"`
	Timeout     time.Duration `env:"TIMEOUT"`
	IdleTimeout time.Duration `env:"IDLE_TIMEOUT"`
	Prefix      string        `env:"API_PREFIX"`
	AppVersion  string        `env:"APP_VERSION"`
	SizeLimit   int64         `env:"LIMIT_READ"`
}

func (h *HTTPServer) APIPrefix() string {
	return fmt.Sprintf("/%s/api/%s", h.Prefix, h.AppVersion)
}

type RedisConfig struct {
	Addr     string        `env:"REDIS_ADDR"`
	Password string        `env:"REDIS_PASSWORD"`
	DB       int           `env:"REDIS_DB"`
	TTL      time.Duration `env:"JWT_VERIFY_CACHE_TTL"`
}

type CORSConfig struct {
	AllowOrigins     string `env:"CORS_ALLOW_ORIGINS"`     // comma-separated
	AllowMethods     string `env:"CORS_ALLOW_METHODS"`     // comma-separated
	AllowHeaders     string `env:"CORS_ALLOW_HEADERS"`     // comma-separated
	ExposeHeaders    string `env:"CORS_EXPOSE_HEADERS"`    // comma-separated
	AllowCredentials bool   `env:"CORS_ALLOW_CREDENTIALS"` // true/false
	MaxAge           int    `env:"CORS_MAX_AGE"`           // seconds
}

type SwaggerConfig struct {
	Enabled bool `env:"SWAGGER_ENABLED"`
}

func GetConfig() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	cfg.Normalize()
	return &cfg, nil
}

func (c *Config) Normalize() {
	c.LogLevel = strings.TrimSpace(c.LogLevel)

	c.HTTPServer.Address = strings.TrimSpace(c.HTTPServer.Address)
	c.HTTPServer.Prefix = strings.Trim(strings.TrimSpace(c.HTTPServer.Prefix), "/")
	c.HTTPServer.AppVersion = strings.Trim(strings.TrimSpace(c.HTTPServer.AppVersion), "/")

	c.AuthConfig.BaseUrl = strings.TrimRight(strings.TrimSpace(c.AuthConfig.BaseUrl), "/")
	c.NoteFlowConfig.BaseUrlNotes = strings.TrimRight(strings.TrimSpace(c.NoteFlowConfig.BaseUrlNotes), "/")

	c.RedisConfig.Addr = strings.TrimSpace(c.RedisConfig.Addr)

	if c.HTTPServer.Timeout <= 0 {
		c.HTTPServer.Timeout = 5 * time.Second
	}
	if c.HTTPServer.IdleTimeout <= 0 {
		c.HTTPServer.IdleTimeout = 60 * time.Second
	}
	if c.HTTPServer.SizeLimit <= 0 {
		c.HTTPServer.SizeLimit = 64 * 1024
	}

	if c.RedisConfig.TTL <= 0 {
		c.RedisConfig.TTL = 60 * time.Second
	}

	// Sensible CORS defaults: safest is "none" unless explicitly configured.
	// Leave strings as-is; parsing happens where CORS middleware is configured.
}

func (c *Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.HTTPServer.Address) == "" {
		errs = append(errs, errors.New("ADDRESS is required"))
	}
	if strings.TrimSpace(c.HTTPServer.Prefix) == "" {
		errs = append(errs, errors.New("API_PREFIX is required"))
	}
	if strings.TrimSpace(c.HTTPServer.AppVersion) == "" {
		errs = append(errs, errors.New("APP_VERSION is required"))
	}

	if strings.TrimSpace(c.AuthConfig.BaseUrl) == "" {
		errs = append(errs, errors.New("AUTH_BASE_URL is required"))
	}
	if strings.TrimSpace(c.AuthConfig.V1Verify) == "" {
		errs = append(errs, errors.New("AUTH_V1_VERIFY is required"))
	}
	if strings.TrimSpace(c.AuthConfig.V1Logout) == "" {
		errs = append(errs, errors.New("AUTH_V1_LOGOUT is required"))
	}

	if strings.TrimSpace(c.NoteFlowConfig.BaseUrlNotes) == "" {
		errs = append(errs, errors.New("NOTE_FLOW_BASE_URL is required"))
	}

	// Redis is required for JWT verify caching if you enable the middleware that depends on it.
	// Keep it optional at config level to avoid breaking local dev; runtime should fail fast if caching is enabled but Redis isn't configured.

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

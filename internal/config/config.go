package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	LogLevel string `env:"LOG_LEVEL"`
	HTTPServer
	AuthConfig
	NoteFlowConfig
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

func GetConfig() *Config {
	_ = godotenv.Load()

	var cfg Config
	errEnv := cleanenv.ReadEnv(&cfg)
	if errEnv != nil {
		return nil
	}
	return &cfg
}

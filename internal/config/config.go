package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env string `env:"ENV" env-default:"local" env:"ENV"`
	HTTPServer
	AuthConfig
	NoteFlowConfig
}

type AuthConfig struct {
	BaseUrl         string `env:"BASE_URL" env-default:"http://auth:80/auth/api"`
	V1Register      string `env:"V1_REGISTER" env-default:"/v1/register"`
	V1Login         string `env:"V1_LOGIN" env-default:"/v1/login"`
	V1Verify        string `env:"V1_LOGIN" env-default:"/v1/verify"`
	V1Logout        string `env:"V1_LOGOUT" env-default:"/v1/logout"`
	V1SendEmailCode string `env:"V1_SEND_EMAIL_CODE" env-default:"/v1/send-email-code"`
	V1VerifyEmail   string `env:"V1_VERIFY_EMAIL" env-default:"/v1/verify-email-code"`
}

type NoteFlowConfig struct {
	BaseUrlNotes  string `env:"BASE_URL" env-default:"http://note-flow:80/note-flow/api"`
	V1Notes       string `env:"V1_REGISTER" env-default:"/v1/notes"`
	V1Note        string `env:"V1_NOTE" env-default:"/v1/note"`
	V1NoteActive  string `env:"V1_NOTE_ACTIVE" env-default:"/v1/note/active"`
	V1NoteLike    string `env:"V1_NOTE_LIKE" env-default:"/v1/note/like"`
	V1NoteComment string `env:"V1_NOTE_COMMENT" env-default:"/v1/note/comment"`
}

type HTTPServer struct {
	Address     string        `env-default:"0.0.0.0:8000" env:"ADDRESS"`
	Timeout     time.Duration `env-default:"5s" env:"TIMEOUT"`
	IdleTimeout time.Duration `env-default:"60s" env:"IDLE_TIMEOUT"`
	Prefix      string        `env:"API_PREFIX" env-default:"gateway"`
	AppVersion  string        `env:"APP_VERSION" env-default:"v1"`
	SizeLimit   int64         `env:"LIMIT_READ" env-default:"65536"`
}

func (h *HTTPServer) APIPrefix() string {
	return fmt.Sprintf("/%s/api/%s", h.Prefix, h.AppVersion)
}

func GetConfig() *Config {
	var cfg Config
	errEnv := cleanenv.ReadEnv(&cfg)
	if errEnv != nil {
		return nil
	}
	return &cfg
}

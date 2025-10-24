package config

import (
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/samber/oops"
	"gopkg.in/yaml.v3"
)

type Config struct {
	// Service name for telemetry and logs
	ServiceName string `yaml:"service_name" env:"SERVICE_NAME" example:"nicemaxxingbot" validate:"required"`
	// Streamer username (lowercase)
	Streamer   string     `yaml:"streamer" env:"STREAMER" example:"k0per1s" validate:"required"`
	Sentry     Sentry     `yaml:"sentry" envPrefix:"SENTRY_"`
	Log        Log        `yaml:"log" envPrefix:"LOG_"`
	Telemetry  Telemetry  `yaml:"telemetry" envPrefix:"TELEMETRY_"`
	Twitch     Twitch     `yaml:"twitch" envPrefix:"TWITCH_"`
	FreeOpenAI OpenAI     `yaml:"free_openai" envPrefix:"FREE_OPENAI_"`
	OpenAI     OpenAI     `yaml:"openai" envPrefix:"OPENAI_"`
	Whisper    Whisper    `yaml:"whisper" envPrefix:"WHISPER_"`
	Processing Processing `yaml:"processing" envPrefix:"PROCESSING_"`
}

type Sentry struct {
	DSN string `yaml:"dsn" env:"DSN" example:"https://a1b2c3d4e5f6g7h8a1b2c3d4e5f6g7h8@o123456.ingest.sentry.io/1234567"`
}

type Log struct {
	// Telegram logging config
	Telegram TelegramLog `yaml:"telegram" envPrefix:"TELEGRAM_"`
}

type TelegramLog struct {
	// Chat bot token, obtain it via BotFather
	Token string `yaml:"token" env:"TOKEN" example:"1234567890:ABCdefGHIjklMNopQRstUVwxyZ-123456789"`
	// Chat ID to send messages to
	ChatID string `yaml:"chat_id" env:"CHAT_ID" example:"1001234567890"`
}

type Telemetry struct {
	// Whether to enable opentelemetry logs/metrics/traces export
	Enabled bool `yaml:"enabled" env:"ENABLED" example:"false"`
}

type Twitch struct {
	// ClientID of the twitch application
	ClientID string `yaml:"client_id" example:"a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p" validate:"required"`
	// Client secret of the twitch application
	ClientSecret string `yaml:"client_secret" example:"abc123def456ghi789jkl012mno345pqr678stu901" validate:"required"`
	// Username of the bot account
	Username string `yaml:"username" example:"PogChamp123" validate:"required"`
	// User refresh token of the bot account
	RefreshToken string `yaml:"refresh_token" example:"v1.abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567" validate:"required"`
	// Disable notifications
	DisableNotifications bool `yaml:"disable_notifications" example:"false"`
	// Minimum streak length in minutes
	MinStreakLength int `yaml:"min_streak_length" example:"30"`
}

type OpenAI struct {
	// OpenAI base url
	BaseURL string `yaml:"base_url" example:"https://openrouter.ai/api/v1" validate:"required"`
	// OpenAI token
	Token string `yaml:"token" example:"sk-proj-abc123456789DEF789ghi012JKL345mno678PQR901stu234VWX" validate:"required"`
	// OpenAI model
	Model string `yaml:"model" example:"deepseek/deepseek-chat-v3-0324:free" validate:"required"`
}

type Whisper struct {
	// OpenAI base url
	BaseURL string `yaml:"base_url" example:"http://whisper-api:8080/api/v1" validate:"required"`
	// OpenAI model
	Model string `yaml:"model" example:"ggml-medium-q5_0" validate:"required"`
}

type Processing struct {
	// How many characters to accumulate before calling OpenAI
	BatchSize int `yaml:"batch_size" env:"BATCH_SIZE" example:"10000"`
	// How many seconds to wait before calling OpenAI (if BatchSize character limit was not reached)
	BatchTimeout int `yaml:"batch_timeout" env:"BATCH_TIMEOUT" example:"120"`
}

func Load(configPath string) (*Config, error) {
	var result Config

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, oops.Errorf("failed to read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, oops.Errorf("failed to parse YAML config: %w", err)
	}

	if err := env.ParseWithOptions(&result, env.Options{ //nolint:exhaustruct
		Prefix: "NICEMAXXINGBOT_",
	}); err != nil {
		return nil, oops.Errorf("failed to parse environment variables: %w", err)
	}

	if result.ServiceName == "" {
		result.ServiceName = "nicemaxxingbot"
	}
	if result.Processing.BatchSize == 0 {
		result.Processing.BatchSize = 10000
	}
	if result.Processing.BatchTimeout == 0 {
		result.Processing.BatchTimeout = 120
	}
	if result.Twitch.MinStreakLength == 0 {
		result.Twitch.MinStreakLength = 30
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(result); err != nil {
		return nil, oops.Errorf("failed to validate config: %w", err)
	}

	return &result, nil
}

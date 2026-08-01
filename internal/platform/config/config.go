// Package config loads infrastructure configuration from the environment.
//
// Only infrastructure and secrets live here. Anything the business might change
// without a deploy — tax rates, capacities, cutoffs, templates — lives in the
// sys_parameters table instead (BR-1.4.1).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App      App
	Database Database
	Auth     Auth
	Storage  Storage
	Mail     Mail
	WhatsApp WhatsApp
	Payment  Payment
}

type App struct {
	Env             string
	Port            int
	BaseURL         string
	AdminBaseURL    string
	LogLevel        string
	TrustedProxies  []string
	AllowedOrigins  []string
	ShutdownTimeout time.Duration
}

func (a App) IsProduction() bool { return a.Env == "production" }

type Database struct {
	URL          string
	TestURL      string
	MaxOpenConns int
	MaxIdleConns int
}

type Auth struct {
	SigningKey  string
	PreviousKey string
	Issuer      string
	Google      OAuthProvider
	Instagram   OAuthProvider
}

type OAuthProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Configured reports whether credentials exist. Providers without credentials
// stay disabled at runtime (docs/00 Q8) rather than failing at startup.
func (p OAuthProvider) Configured() bool { return p.ClientID != "" && p.ClientSecret != "" }

type Storage struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
}

type Mail struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
	TLS       bool
}

type WhatsApp struct {
	WAHAURL     string
	WAHASession string
	WAHAAPIKey  string
	MetaPhoneID string
	MetaToken   string
}

type Payment struct {
	WebhookSecret string
}

// Load reads the environment and validates what the service cannot run without.
func Load() (*Config, error) {
	c := &Config{
		App: App{
			Env:             getString("APP_ENV", "development"),
			Port:            getInt("APP_PORT", 8080),
			BaseURL:         getString("APP_BASE_URL", "http://127.0.0.1:8080"),
			AdminBaseURL:    getString("APP_ADMIN_BASE_URL", ""),
			LogLevel:        getString("LOG_LEVEL", "info"),
			TrustedProxies:  getList("TRUSTED_PROXIES", []string{"127.0.0.1"}),
			AllowedOrigins:  getList("CORS_ALLOWED_ORIGINS", nil),
			ShutdownTimeout: 15 * time.Second,
		},
		Database: Database{
			URL:          getString("DATABASE_URL", ""),
			TestURL:      getString("TEST_DATABASE_URL", ""),
			MaxOpenConns: getInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getInt("DATABASE_MAX_IDLE_CONNS", 5),
		},
		Auth: Auth{
			SigningKey:  getString("JWT_SIGNING_KEY", ""),
			PreviousKey: getString("JWT_PREVIOUS_KEY", ""),
			Issuer:      getString("JWT_ISSUER", "ruuma"),
			Google: OAuthProvider{
				ClientID:     getString("GOOGLE_OAUTH_CLIENT_ID", ""),
				ClientSecret: getString("GOOGLE_OAUTH_CLIENT_SECRET", ""),
				RedirectURL:  getString("GOOGLE_OAUTH_REDIRECT_URL", ""),
			},
			Instagram: OAuthProvider{
				ClientID:     getString("INSTAGRAM_OAUTH_CLIENT_ID", ""),
				ClientSecret: getString("INSTAGRAM_OAUTH_CLIENT_SECRET", ""),
				RedirectURL:  getString("INSTAGRAM_OAUTH_REDIRECT_URL", ""),
			},
		},
		Storage: Storage{
			Endpoint:       getString("MINIO_ENDPOINT", "127.0.0.1:9002"),
			PublicEndpoint: getString("MINIO_PUBLIC_ENDPOINT", ""),
			AccessKey:      getString("MINIO_ACCESS_KEY", ""),
			SecretKey:      getString("MINIO_SECRET_KEY", ""),
			Bucket:         getString("MINIO_BUCKET", "ruuma"),
			UseSSL:         getBool("MINIO_USE_SSL", false),
		},
		Mail: Mail{
			Host:      getString("SMTP_HOST", "127.0.0.1"),
			Port:      getInt("SMTP_PORT", 1025),
			Username:  getString("SMTP_USERNAME", ""),
			Password:  getString("SMTP_PASSWORD", ""),
			FromEmail: getString("SMTP_FROM_EMAIL", "no-reply@ruuma.id"),
			FromName:  getString("SMTP_FROM_NAME", "ruuma"),
			TLS:       getBool("SMTP_TLS", false),
		},
		WhatsApp: WhatsApp{
			WAHAURL:     getString("WAHA_URL", "http://127.0.0.1:3000"),
			WAHASession: getString("WAHA_SESSION", "default"),
			WAHAAPIKey:  getString("WAHA_API_KEY", ""),
			MetaPhoneID: getString("META_WA_PHONE_NUMBER_ID", ""),
			MetaToken:   getString("META_WA_ACCESS_TOKEN", ""),
		},
		Payment: Payment{
			WebhookSecret: getString("PAYMENT_WEBHOOK_SECRET", ""),
		},
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	var missing []string
	if c.Database.URL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.Auth.SigningKey == "" {
		missing = append(missing, "JWT_SIGNING_KEY")
	}
	if len(missing) > 0 {
		return fmt.Errorf("config: missing required environment: %s", strings.Join(missing, ", "))
	}
	if c.App.IsProduction() {
		if len(c.Auth.SigningKey) < 32 {
			return fmt.Errorf("config: JWT_SIGNING_KEY must be at least 32 bytes in production")
		}
		if len(c.App.AllowedOrigins) == 0 {
			return fmt.Errorf("config: CORS_ALLOWED_ORIGINS must be set in production (docs/12 A01)")
		}
		if c.Storage.AccessKey == "" || c.Storage.SecretKey == "" {
			return fmt.Errorf("config: MinIO credentials are required in production")
		}
	}
	return nil
}

// Redacted returns a copy safe to log: every secret is replaced (BR-1.4.3).
func (c *Config) Redacted() map[string]any {
	return map[string]any{
		"app_env":         c.App.Env,
		"app_port":        c.App.Port,
		"base_url":        c.App.BaseURL,
		"log_level":       c.App.LogLevel,
		"allowed_origins": c.App.AllowedOrigins,
		"database":        redactDSN(c.Database.URL),
		"storage_bucket":  c.Storage.Bucket,
		"smtp_host":       c.Mail.Host,
		"waha_url":        c.WhatsApp.WAHAURL,
		"google_oauth":    c.Auth.Google.Configured(),
		"instagram_oauth": c.Auth.Instagram.Configured(),
	}
}

func redactDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	at := strings.LastIndex(dsn, "@")
	scheme := strings.Index(dsn, "://")
	if at < 0 || scheme < 0 {
		return "[redacted]"
	}
	return dsn[:scheme+3] + "[redacted]" + dsn[at:]
}

func getString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getList(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

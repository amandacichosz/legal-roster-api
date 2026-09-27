package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                           string
	AppEnv                         string
	DatabaseURL                    string
	StaticDir                      string
	APIBaseURL                     string
	FrontendURL                    string
	CORSOrigins                    string
	SessionCookieSecure            bool
	BootstrapPlatformAdminEmail    string
	BootstrapPlatformAdminPassword string
	BootstrapPlatformAdminName     string
	SMTPHost                       string
	SMTPPort                       string
	SMTPUsername                   string
	SMTPPassword                   string
	EmailFrom                      string
	ObjectStorageProvider          string
	ObjectStorageLocalDir          string
	ObjectStorageEndpoint          string
	ObjectStorageRegion            string
	ObjectStorageBucket            string
	ObjectStorageAccessKeyID       string
	ObjectStorageSecretAccessKey   string
	ObjectStorageForcePathStyle    bool
}

func FromEnv() Config {
	return Config{
		Port:                           envOrDefault("PORT", "8080"),
		AppEnv:                         envOrDefault("APP_ENV", "development"),
		DatabaseURL:                    envOrDefault("DATABASE_URL", "postgres://legal_bench:legal_bench_dev_password@localhost:5432/legal_bench?sslmode=disable"),
		StaticDir:                      envOrDefault("STATIC_DIR", "../legal-bench/dist"),
		APIBaseURL:                     envOrDefault("API_BASE_URL", "https://api.example.com"),
		FrontendURL:                    envOrDefault("FRONTEND_URL", "https://legal-bench.example.com"),
		CORSOrigins:                    envOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:5173,https://legal-bench.example.com"),
		SessionCookieSecure:            envBoolOrDefault("SESSION_COOKIE_SECURE", false),
		BootstrapPlatformAdminEmail:    os.Getenv("BOOTSTRAP_PLATFORM_ADMIN_EMAIL"),
		BootstrapPlatformAdminPassword: os.Getenv("BOOTSTRAP_PLATFORM_ADMIN_PASSWORD"),
		BootstrapPlatformAdminName:     envOrDefault("BOOTSTRAP_PLATFORM_ADMIN_NAME", "Platform Admin"),
		SMTPHost:                       os.Getenv("SMTP_HOST"),
		SMTPPort:                       envOrDefault("SMTP_PORT", "587"),
		SMTPUsername:                   os.Getenv("SMTP_USERNAME"),
		SMTPPassword:                   os.Getenv("SMTP_PASSWORD"),
		EmailFrom:                      os.Getenv("EMAIL_FROM"),
		ObjectStorageProvider:          envOrDefault("OBJECT_STORAGE_PROVIDER", "local"),
		ObjectStorageLocalDir:          envOrDefault("OBJECT_STORAGE_LOCAL_DIR", "data/private-objects"),
		ObjectStorageEndpoint:          os.Getenv("OBJECT_STORAGE_ENDPOINT"),
		ObjectStorageRegion:            envOrDefault("OBJECT_STORAGE_REGION", "us-east-1"),
		ObjectStorageBucket:            os.Getenv("OBJECT_STORAGE_BUCKET"),
		ObjectStorageAccessKeyID:       os.Getenv("OBJECT_STORAGE_ACCESS_KEY_ID"),
		ObjectStorageSecretAccessKey:   os.Getenv("OBJECT_STORAGE_SECRET_ACCESS_KEY"),
		ObjectStorageForcePathStyle:    envBoolOrDefault("OBJECT_STORAGE_FORCE_PATH_STYLE", true),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBoolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

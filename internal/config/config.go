package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                           string
	AppEnv                         string
	DatabaseURL                    string
	SupabaseURL                    string
	SupabasePublishableKey         string
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
	objectStorageProvider := os.Getenv("OBJECT_STORAGE_PROVIDER")
	objectStorageEndpoint := os.Getenv("OBJECT_STORAGE_ENDPOINT")
	objectStorageRegion := os.Getenv("OBJECT_STORAGE_REGION")
	objectStorageBucket := os.Getenv("OBJECT_STORAGE_BUCKET")
	objectStorageAccessKeyID := os.Getenv("OBJECT_STORAGE_ACCESS_KEY_ID")
	objectStorageSecretAccessKey := os.Getenv("OBJECT_STORAGE_SECRET_ACCESS_KEY")

	r2AccountID := os.Getenv("R2_ACCOUNT_ID")
	if objectStorageProvider == "" && (r2AccountID != "" || os.Getenv("R2_BUCKET") != "") {
		objectStorageProvider = "s3"
	}
	if objectStorageEndpoint == "" && r2AccountID != "" {
		objectStorageEndpoint = "https://" + r2AccountID + ".r2.cloudflarestorage.com"
	}
	if objectStorageRegion == "" && r2AccountID != "" {
		objectStorageRegion = "auto"
	}
	if objectStorageBucket == "" {
		objectStorageBucket = os.Getenv("R2_BUCKET")
	}
	if objectStorageAccessKeyID == "" {
		objectStorageAccessKeyID = os.Getenv("R2_ACCESS_KEY_ID")
	}
	if objectStorageSecretAccessKey == "" {
		objectStorageSecretAccessKey = os.Getenv("R2_SECRET_ACCESS_KEY")
	}

	return Config{
		Port:                           envOrDefault("PORT", "8080"),
		AppEnv:                         envOrDefault("APP_ENV", "development"),
		DatabaseURL:                    envOrDefault("DATABASE_URL", "postgres://legal_roster:legal_roster_dev_password@localhost:5432/legal_roster?sslmode=disable"),
		SupabaseURL:                    os.Getenv("SUPABASE_URL"),
		SupabasePublishableKey:         os.Getenv("SUPABASE_PUBLISHABLE_KEY"),
		StaticDir:                      envOrDefault("STATIC_DIR", "../legal-roster/dist"),
		APIBaseURL:                     envOrDefault("API_BASE_URL", "https://api.example.com"),
		FrontendURL:                    envOrDefault("FRONTEND_URL", "https://legal-roster.example.com"),
		CORSOrigins:                    envOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:5173,https://legal-roster.example.com"),
		SessionCookieSecure:            envBoolOrDefault("SESSION_COOKIE_SECURE", false),
		BootstrapPlatformAdminEmail:    os.Getenv("BOOTSTRAP_PLATFORM_ADMIN_EMAIL"),
		BootstrapPlatformAdminPassword: os.Getenv("BOOTSTRAP_PLATFORM_ADMIN_PASSWORD"),
		BootstrapPlatformAdminName:     envOrDefault("BOOTSTRAP_PLATFORM_ADMIN_NAME", "Platform Admin"),
		SMTPHost:                       os.Getenv("SMTP_HOST"),
		SMTPPort:                       envOrDefault("SMTP_PORT", "587"),
		SMTPUsername:                   os.Getenv("SMTP_USERNAME"),
		SMTPPassword:                   os.Getenv("SMTP_PASSWORD"),
		EmailFrom:                      os.Getenv("EMAIL_FROM"),
		ObjectStorageProvider:          valueOrDefault(objectStorageProvider, "local"),
		ObjectStorageLocalDir:          envOrDefault("OBJECT_STORAGE_LOCAL_DIR", "data/private-objects"),
		ObjectStorageEndpoint:          objectStorageEndpoint,
		ObjectStorageRegion:            valueOrDefault(objectStorageRegion, "us-east-1"),
		ObjectStorageBucket:            objectStorageBucket,
		ObjectStorageAccessKeyID:       objectStorageAccessKeyID,
		ObjectStorageSecretAccessKey:   objectStorageSecretAccessKey,
		ObjectStorageForcePathStyle:    envBoolOrDefault("OBJECT_STORAGE_FORCE_PATH_STYLE", true),
	}
}

func valueOrDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
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

package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	CORSOrigins      string
	OpenAIAPIKey     string
	VisionModel      string
	DefaultUserName  string
	HouseholdName    string
	UploadDir        string
	NFCeFixturePath  string
	NFCeForceFixture bool
}

func Load() Config {
	return Config{
		HTTPAddr:         getenv("HTTP_ADDR", ":3847"),
		DatabaseURL:      getenv("DATABASE_URL", "postgres://estoque:estoque@127.0.0.1:5432/estoque?sslmode=disable"),
		CORSOrigins:      getenv("CORS_ORIGINS", "http://127.0.0.1:3848,http://localhost:3848"),
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		VisionModel:      getenv("VISION_MODEL", "gpt-4o-mini"),
		DefaultUserName:  getenv("DEFAULT_USER_NAME", "Morador"),
		HouseholdName:    getenv("HOUSEHOLD_NAME", "Casa"),
		UploadDir:        getenv("UPLOAD_DIR", "./data/uploads"),
		NFCeFixturePath:  getenv("NFCE_FIXTURE_PATH", ""),
		NFCeForceFixture: getenv("NFCE_FORCE_FIXTURE", "false") == "true",
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

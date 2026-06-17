// Package config loads runtime configuration from environment variables with
// sensible defaults, so the service runs out of the box for local development.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the server runtime configuration.
type Config struct {
	Port          string        // HTTP port to listen on
	AllowOrigin   string        // CORS allowed origin
	DataPath      string        // JSON file the master data is persisted to (when no DB)
	DatabaseURL   string        // PostgreSQL DSN; when set, used instead of the JSON file
	SessionTTL    time.Duration // bearer-token session lifetime
	GoogleCreds   string        // path to the Google service-account JSON (enables Sheets sync)
	GoogleSheetID string        // Google Spreadsheet ID to sync from
	SyncIntervalM int           // auto-sync interval in minutes (0 = off at startup)
}

// Load reads configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		Port:          getenv("SALES_PORT", "8085"),
		AllowOrigin:   getenv("SALES_ALLOW_ORIGIN", "*"),
		DataPath:      getenv("SALES_DATA_PATH", "data/sales-data.json"),
		DatabaseURL:   getenv("SALES_DATABASE_URL", ""),
		SessionTTL:    12 * time.Hour,
		GoogleCreds:   getenv("SALES_GOOGLE_CREDENTIALS", ""),
		GoogleSheetID: getenv("SALES_GSHEET_ID", "1FR0xlB5pEmrbsm3SAtfVAUUG3sDM9MHiseUdTyTD1j8"),
		SyncIntervalM: atoiDefault(getenv("SALES_SYNC_INTERVAL_MIN", "0"), 0),
	}
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

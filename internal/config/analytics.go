package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// AnalyticsConfig controls optional cold-path export to ClickHouse (never authoritative vs OLTP).
type AnalyticsConfig struct {
	// ClickHouseEnabled turns on HTTP client wiring and Ping at worker startup when the worker process loads config.
	ClickHouseEnabled bool
	// ClickHouseHTTPURL is a ClickHouse HTTP endpoint including database path, e.g.
	// http://localhost:8123/avf (optionally include userinfo when your deployment requires auth).
	ClickHouseHTTPURL string
	// MirrorOutboxPublished schedules one JSONEachRow insert per successfully marked outbox publish (async, bounded).
	MirrorOutboxPublished bool
	// ProjectOutboxEvents writes typed analytics projection rows derived from published outbox events.
	ProjectOutboxEvents bool
	// MirrorTable is the table name within the configured database (default avf_outbox_mirror).
	MirrorTable string
	// ProjectionTable is the table name for typed analytics projections (default avf_analytics_projection_events).
	ProjectionTable string
	// MirrorMaxConcurrent inserts in flight from the worker (default 8).
	MirrorMaxConcurrent int
	// InsertTimeout bounds each HTTP insert attempt (default 5s).
	InsertTimeout time.Duration
	// InsertMaxAttempts is retries per row including the first attempt (default 3, min 1).
	InsertMaxAttempts int
}

func loadAnalyticsConfig() AnalyticsConfig {
	tc := strings.TrimSpace(getenv("ANALYTICS_CLICKHOUSE_TABLE", "avf_outbox_mirror"))
	if tc == "" {
		tc = "avf_outbox_mirror"
	}
	pc := strings.TrimSpace(getenv("ANALYTICS_PROJECTION_TABLE", "avf_analytics_projection_events"))
	if pc == "" {
		pc = "avf_analytics_projection_events"
	}
	mc := getenvInt("ANALYTICS_MIRROR_MAX_CONCURRENT", 8)
	if mc < 1 {
		mc = 1
	}
	if mc > 256 {
		mc = 256
	}
	attempts := getenvInt("ANALYTICS_INSERT_MAX_ATTEMPTS", 3)
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 10 {
		attempts = 10
	}
	return AnalyticsConfig{
		ClickHouseEnabled:     getenvBool("ANALYTICS_CLICKHOUSE_ENABLED", false),
		ClickHouseHTTPURL:     strings.TrimSpace(getenv("ANALYTICS_CLICKHOUSE_HTTP_URL", "")),
		MirrorOutboxPublished: getenvBool("ANALYTICS_MIRROR_OUTBOX_PUBLISHED", false),
		ProjectOutboxEvents:   getenvBool("ANALYTICS_PROJECT_OUTBOX_EVENTS", false),
		MirrorTable:           tc,
		ProjectionTable:       pc,
		MirrorMaxConcurrent:   mc,
		InsertTimeout:         mustParseDuration("ANALYTICS_INSERT_TIMEOUT", getenv("ANALYTICS_INSERT_TIMEOUT", "5s")),
		InsertMaxAttempts:     attempts,
	}
}

func (a AnalyticsConfig) validate() error {
	if a.MirrorOutboxPublished && !a.ClickHouseEnabled {
		return errors.New("config: ANALYTICS_MIRROR_OUTBOX_PUBLISHED=true requires ANALYTICS_CLICKHOUSE_ENABLED=true")
	}
	if a.ProjectOutboxEvents && !a.ClickHouseEnabled {
		return errors.New("config: ANALYTICS_PROJECT_OUTBOX_EVENTS=true requires ANALYTICS_CLICKHOUSE_ENABLED=true")
	}
	if !a.ClickHouseEnabled {
		return nil
	}
	if strings.TrimSpace(a.ClickHouseHTTPURL) == "" {
		return errors.New("config: ANALYTICS_CLICKHOUSE_HTTP_URL is required when ANALYTICS_CLICKHOUSE_ENABLED=true")
	}
	u, err := url.Parse(a.ClickHouseHTTPURL)
	if err != nil {
		return fmt.Errorf("config: ANALYTICS_CLICKHOUSE_HTTP_URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("config: ANALYTICS_CLICKHOUSE_HTTP_URL must use http or https scheme (got %q)", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("config: ANALYTICS_CLICKHOUSE_HTTP_URL must include a host")
	}
	db := strings.Trim(strings.TrimSpace(u.Path), "/")
	if db == "" {
		return errors.New("config: ANALYTICS_CLICKHOUSE_HTTP_URL must include database path (e.g. http://host:8123/avf)")
	}
	if err := validateMirrorTableName(a.MirrorTable); err != nil {
		return err
	}
	if err := validateAnalyticsTableName("ANALYTICS_PROJECTION_TABLE", a.ProjectionTable); err != nil {
		return err
	}
	return nil
}

func validateMirrorTableName(name string) error {
	return validateAnalyticsTableName("ANALYTICS_CLICKHOUSE_TABLE", name)
}

func validateAnalyticsTableName(envName, name string) error {
	s := strings.TrimSpace(name)
	if s == "" {
		return fmt.Errorf("config: %s is empty", envName)
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		default:
			return fmt.Errorf("config: %s %q contains invalid character %q", envName, name, r)
		}
	}
	return nil
}

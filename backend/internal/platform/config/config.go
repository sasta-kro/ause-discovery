package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDatabaseURL               = "postgres://ause:ause@postgres:5432/ause_discovery?sslmode=disable"
	defaultMeilisearchURL            = "http://meilisearch:7700"
	defaultArtifactRoot              = "/var/lib/ause-discovery/artifacts"
	defaultImportTemporaryRoot       = "/var/lib/ause-discovery/imports"
	defaultMaxArtifactBytes    int64 = 262144000
	defaultMaxProjectBytes     int64 = 2147483648
	defaultSessionIdleTTL            = 30 * time.Minute
	defaultSessionAbsoluteTTL        = 12 * time.Hour
	defaultLogLevel                  = "info"
	defaultListenAddress             = ":8080"
	defaultMeilisearchIndex          = "projects"
	defaultCatalogRoot               = "config"
)

type Config struct {
	Environment              string
	ListenAddress            string
	PublicBasePath           string
	DatabaseURL              string
	MeilisearchURL           string
	MeilisearchAPIKey        string
	MeilisearchIndex         string
	ArtifactStorageBackend   string
	ArtifactRoot             string
	ArtifactB2Endpoint       string
	ArtifactB2Bucket         string
	ArtifactB2KeyID          string
	ArtifactB2ApplicationKey string
	ImportTemporaryRoot      string
	MaxArtifactBytes         int64
	MaxProjectArtifactBytes  int64
	SessionIdleTTL           time.Duration
	SessionAbsoluteTTL       time.Duration
	SessionSecret            string
	CookieSecure             bool
	TrustedProxyCIDRs        []string
	FeaturedProjectIDs       []string
	LogLevel                 string
	CatalogRoot              string
}

func Load(getenv func(string) string) (Config, error) {
	environment := valueOrDefault(getenv("AUSE_ENV"), "development")
	if environment != "development" && environment != "test" && environment != "production" {
		return Config{}, fmt.Errorf("AUSE_ENV must be development, test, or production")
	}

	publicBasePath, err := NormalizePublicBasePath(valueOrDefault(getenv("AUSE_PUBLIC_BASE_PATH"), "/ause-discovery/"))
	if err != nil {
		return Config{}, err
	}

	maxArtifactBytes, err := parsePositiveInt64("AUSE_MAX_ARTIFACT_BYTES", getenv("AUSE_MAX_ARTIFACT_BYTES"), defaultMaxArtifactBytes)
	if err != nil {
		return Config{}, err
	}

	maxProjectArtifactBytes, err := parsePositiveInt64("AUSE_MAX_PROJECT_ARTIFACT_BYTES", getenv("AUSE_MAX_PROJECT_ARTIFACT_BYTES"), defaultMaxProjectBytes)
	if err != nil {
		return Config{}, err
	}

	sessionIdleTTL, err := parsePositiveDuration("AUSE_SESSION_IDLE_TTL", getenv("AUSE_SESSION_IDLE_TTL"), defaultSessionIdleTTL)
	if err != nil {
		return Config{}, err
	}

	sessionAbsoluteTTL, err := parsePositiveDuration("AUSE_SESSION_ABSOLUTE_TTL", getenv("AUSE_SESSION_ABSOLUTE_TTL"), defaultSessionAbsoluteTTL)
	if err != nil {
		return Config{}, err
	}

	cookieSecure, err := strconv.ParseBool(valueOrDefault(getenv("AUSE_COOKIE_SECURE"), "false"))
	if err != nil {
		return Config{}, fmt.Errorf("AUSE_COOKIE_SECURE must be true or false: %w", err)
	}

	artifactStorageBackend := valueOrDefault(getenv("AUSE_ARTIFACT_STORAGE_BACKEND"), "local")
	if artifactStorageBackend != "local" && artifactStorageBackend != "b2" {
		return Config{}, errors.New("AUSE_ARTIFACT_STORAGE_BACKEND must be local or b2")
	}
	b2Endpoint := strings.TrimSpace(getenv("AUSE_ARTIFACT_B2_ENDPOINT"))
	b2Bucket := strings.TrimSpace(getenv("AUSE_ARTIFACT_B2_BUCKET"))
	b2KeyID := strings.TrimSpace(getenv("AUSE_ARTIFACT_B2_KEY_ID"))
	b2ApplicationKey := strings.TrimSpace(getenv("AUSE_ARTIFACT_B2_APPLICATION_KEY"))
	b2Values := []string{b2Endpoint, b2Bucket, b2KeyID, b2ApplicationKey}
	b2ConfiguredCount := 0
	for _, value := range b2Values {
		if value != "" {
			b2ConfiguredCount++
		}
	}
	if b2ConfiguredCount != 0 && b2ConfiguredCount != len(b2Values) {
		return Config{}, errors.New("AUSE_ARTIFACT_B2_* values must be set together: endpoint, bucket, key ID, and application key")
	}
	if artifactStorageBackend == "b2" && b2ConfiguredCount == 0 {
		return Config{}, errors.New("AUSE_ARTIFACT_STORAGE_BACKEND=b2 requires the four AUSE_ARTIFACT_B2_* values")
	}

	config := Config{
		Environment:              environment,
		ListenAddress:            valueOrDefault(getenv("AUSE_LISTEN_ADDR"), defaultListenAddress),
		PublicBasePath:           publicBasePath,
		DatabaseURL:              valueOrDefault(getenv("AUSE_DATABASE_URL"), defaultDatabaseURL),
		MeilisearchURL:           valueOrDefault(getenv("AUSE_MEILISEARCH_URL"), defaultMeilisearchURL),
		MeilisearchAPIKey:        getenv("AUSE_MEILISEARCH_API_KEY"),
		MeilisearchIndex:         valueOrDefault(getenv("AUSE_MEILISEARCH_INDEX"), defaultMeilisearchIndex),
		ArtifactStorageBackend:   artifactStorageBackend,
		ArtifactRoot:             valueOrDefault(getenv("AUSE_ARTIFACT_ROOT"), defaultArtifactRoot),
		ArtifactB2Endpoint:       b2Endpoint,
		ArtifactB2Bucket:         b2Bucket,
		ArtifactB2KeyID:          b2KeyID,
		ArtifactB2ApplicationKey: b2ApplicationKey,
		ImportTemporaryRoot:      valueOrDefault(getenv("AUSE_IMPORT_TEMP_ROOT"), defaultImportTemporaryRoot),
		MaxArtifactBytes:         maxArtifactBytes,
		MaxProjectArtifactBytes:  maxProjectArtifactBytes,
		SessionIdleTTL:           sessionIdleTTL,
		SessionAbsoluteTTL:       sessionAbsoluteTTL,
		SessionSecret:            getenv("AUSE_SESSION_SECRET"),
		CookieSecure:             cookieSecure,
		TrustedProxyCIDRs:        splitCommaSeparated(getenv("AUSE_TRUSTED_PROXY_CIDRS")),
		FeaturedProjectIDs:       splitCommaSeparated(getenv("AUSE_FEATURED_PROJECT_IDS")),
		LogLevel:                 valueOrDefault(getenv("AUSE_LOG_LEVEL"), defaultLogLevel),
		CatalogRoot:              valueOrDefault(getenv("AUSE_CATALOG_ROOT"), defaultCatalogRoot),
	}

	if err := config.validateProductionSecrets(); err != nil {
		return Config{}, err
	}

	if !filepath.IsAbs(config.ArtifactRoot) || !filepath.IsAbs(config.ImportTemporaryRoot) {
		return Config{}, errors.New("artifact and import temporary roots must be absolute paths")
	}
	if config.CatalogRoot == "" {
		return Config{}, errors.New("catalog root cannot be empty")
	}

	return config, nil
}

func (config Config) AcademicCatalogPath() string {
	return filepath.Join(config.CatalogRoot, "catalogs", "academic.yaml")
}

func (config Config) TaxonomyCatalogPath() string {
	return filepath.Join(config.CatalogRoot, "taxonomy", "values.yaml")
}

func NormalizePublicBasePath(value string) (string, error) {
	trimmedValue := strings.TrimSpace(value)
	if strings.ContainsAny(trimmedValue, "?#") {
		return "", errors.New("AUSE_PUBLIC_BASE_PATH cannot contain a query string or fragment")
	}

	segments := strings.FieldsFunc(trimmedValue, func(character rune) bool {
		return character == '/'
	})
	if len(segments) == 0 {
		return "/", nil
	}

	return "/" + strings.Join(segments, "/") + "/", nil
}

// ArtifactStorageDirectoriesActive reports whether the local filesystem
// artifact store is the primary write backend. When it is not, the artifact
// directory is neither created nor probed, so a B2 deployment does not fail
// readiness over a directory it does not use.
func (config Config) ArtifactStorageDirectoriesActive() bool {
	return config.ArtifactStorageBackend != "b2"
}

func (config Config) EnsureStorageDirectories() error {
	directories := []string{config.ImportTemporaryRoot}
	if config.ArtifactStorageDirectoriesActive() {
		directories = append(directories, config.ArtifactRoot)
	}
	for _, directory := range directories {
		if err := os.MkdirAll(directory, 0750); err != nil {
			return fmt.Errorf("create required storage directory %q: %w", directory, err)
		}
		probe, err := os.CreateTemp(directory, ".write-check-")
		if err != nil {
			return fmt.Errorf("verify required storage directory %q: %w", directory, err)
		}
		probeName := probe.Name()
		closeErr := probe.Close()
		if closeErr == nil {
			closeErr = os.Remove(probeName)
		}
		if closeErr != nil {
			return fmt.Errorf("verify required storage directory %q: %w", directory, closeErr)
		}
	}
	return nil
}

// CheckImportTemporaryDirectory probes only the metadata-import temporary
// directory. Artifact storage readiness belongs to the configured storage
// provider, whose own check covers the local root or the remote bucket.
func (config Config) CheckImportTemporaryDirectory() error {
	probe, err := os.CreateTemp(config.ImportTemporaryRoot, ".write-check-")
	if err != nil {
		return fmt.Errorf("verify required storage directory %q: %w", config.ImportTemporaryRoot, err)
	}

	probeName := probe.Name()
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close storage probe %q: %w", probeName, err)
	}
	if err := os.Remove(probeName); err != nil {
		return fmt.Errorf("remove storage probe %q: %w", probeName, err)
	}

	return nil
}

func (config Config) validateProductionSecrets() error {
	if config.Environment != "production" {
		return nil
	}

	if config.SessionSecret == "" {
		return errors.New("AUSE_SESSION_SECRET is required in production")
	}
	if config.MeilisearchAPIKey == "" {
		return errors.New("AUSE_MEILISEARCH_API_KEY is required in production")
	}
	if !config.CookieSecure {
		return errors.New("AUSE_COOKIE_SECURE must be true in production")
	}

	return nil
}

func parsePositiveInt64(name, value string, defaultValue int64) (int64, error) {
	if value == "" {
		return defaultValue, nil
	}

	parsedValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsedValue <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}

	return parsedValue, nil
}

func parsePositiveDuration(name, value string, defaultValue time.Duration) (time.Duration, error) {
	if value == "" {
		return defaultValue, nil
	}

	parsedValue, err := time.ParseDuration(value)
	if err != nil || parsedValue <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}

	return parsedValue, nil
}

func splitCommaSeparated(value string) []string {
	if value == "" {
		return nil
	}

	values := strings.Split(value, ",")
	result := make([]string, 0, len(values))
	for _, item := range values {
		trimmedItem := strings.TrimSpace(item)
		if trimmedItem != "" {
			result = append(result, trimmedItem)
		}
	}

	return result
}

func valueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}

	return value
}

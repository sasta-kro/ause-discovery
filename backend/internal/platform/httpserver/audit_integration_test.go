package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	api "ause-discovery.local/backend/generated/api"
	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/platform/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestListAuditEventsRequiresSessionAndMapsValidationErrors(t *testing.T) {
	databaseURL := os.Getenv("AUSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("AUSE_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := createAuditHTTPTestDatabase(t, ctx, databaseURL)
	configuration := config.Config{PublicBasePath: "/ause-discovery/", SessionIdleTTL: 30 * time.Minute, SessionAbsoluteTTL: 12 * time.Hour}
	handler := NewAPIHandler(pool, configuration)

	unauthenticated := httptest.NewRequest("GET", "/ause-discovery/api/v1/admin/audit-events", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticatedResponse, unauthenticated)
	if unauthenticatedResponse.Code != 401 {
		t.Fatalf("unauthenticated listing returned %d", unauthenticatedResponse.Code)
	}

	authService := auth.Service{Pool: pool, SessionIdleTTL: configuration.SessionIdleTTL, SessionAbsoluteTTL: configuration.SessionAbsoluteTTL}
	if _, err := authService.CreateUser(ctx, "audit-http-admin", "Audit-Http-2026!x"); err != nil {
		t.Fatalf("create test administrator: %v", err)
	}
	sessionCookie := loginThroughHandler(t, handler, "audit-http-admin", "Audit-Http-2026!x")

	sessionRequest := httptest.NewRequest("GET", "/ause-discovery/api/v1/admin/auth/session", nil)
	sessionRequest.Header.Set("Cookie", sessionCookie)
	sessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(sessionResponse, sessionRequest)
	var session api.SessionResponse
	if err := json.Unmarshal(sessionResponse.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if !containsPermission(session.User.Permissions, "audit.read") {
		t.Fatalf("session permissions omitted audit.read: %v", session.User.Permissions)
	}

	listing := httptest.NewRequest("GET", "/ause-discovery/api/v1/admin/audit-events", nil)
	listing.Header.Set("Cookie", sessionCookie)
	listingResponse := httptest.NewRecorder()
	handler.ServeHTTP(listingResponse, listing)
	if listingResponse.Code != 200 {
		t.Fatalf("authenticated listing returned %d with body %s", listingResponse.Code, listingResponse.Body.String())
	}
	var page api.AuditEventPage
	if err := json.Unmarshal(listingResponse.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode audit page: %v", err)
	}
	if page.Page.Limit != 20 || page.Items == nil {
		t.Fatalf("audit page had limit %d and items %v", page.Page.Limit, page.Items)
	}
	if len(page.Items) != 2 {
		t.Fatalf("audit page held %d events, expected login.success and admin.created", len(page.Items))
	}
	loginSuccess := page.Items[0]
	if loginSuccess.Action != "login.success" || loginSuccess.ActorId == nil {
		t.Fatalf("newest event was %+v, expected login.success with an actor", loginSuccess)
	}
	bootstrap := page.Items[1]
	if bootstrap.Action != "admin.created" || bootstrap.ActorId != nil {
		t.Fatalf("bootstrap event was %+v, expected admin.created without an actor", bootstrap)
	}
	if bootstrap.ResourceType != "application_user" || bootstrap.ResourceId == nil {
		t.Fatalf("bootstrap event resource was %s %v", bootstrap.ResourceType, bootstrap.ResourceId)
	}
	if bootstrap.Metadata == nil || bootstrap.Metadata["username"] != "audit-http-admin" {
		t.Fatalf("bootstrap metadata decoded to %v", bootstrap.Metadata)
	}

	for name, query := range map[string]string{
		"malformed cursor":  "?cursor=not-base64!",
		"invalid actor id":  "?actor_id=not-a-uuid",
		"invalid limit":     "?limit=0x1",
		"invalid actor hex": "?actor_id=018f0000-0000-7000-8000-0000000000zz",
	} {
		invalid := httptest.NewRequest("GET", "/ause-discovery/api/v1/admin/audit-events"+query, nil)
		invalid.Header.Set("Cookie", sessionCookie)
		invalidResponse := httptest.NewRecorder()
		handler.ServeHTTP(invalidResponse, invalid)
		if invalidResponse.Code != 400 {
			t.Fatalf("%s returned %d with body %s, expected 400", name, invalidResponse.Code, invalidResponse.Body.String())
		}
		var problem api.Problem
		if err := json.Unmarshal(invalidResponse.Body.Bytes(), &problem); err != nil || problem.Code != "validation_error" {
			t.Fatalf("%s produced %s, expected a validation_error problem", name, invalidResponse.Body.String())
		}
	}
}

func loginThroughHandler(t *testing.T, handler http.Handler, username, password string) string {
	t.Helper()
	login := httptest.NewRequest("POST", "/ause-discovery/api/v1/admin/auth/login", strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != 200 {
		t.Fatalf("login returned %d with body %s", loginResponse.Code, loginResponse.Body.String())
	}
	var session api.SessionResponse
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if !containsPermission(session.User.Permissions, "audit.read") {
		t.Fatalf("login permissions omitted audit.read: %v", session.User.Permissions)
	}
	for _, cookie := range loginResponse.Header().Values("Set-Cookie") {
		if value, found := strings.CutPrefix(cookie, sessionCookieName+"="); found {
			return sessionCookieName + "=" + strings.Split(value, ";")[0]
		}
	}
	t.Fatal("login response did not set a session cookie")
	return ""
}

func containsPermission(values []string, permission string) bool {
	for _, value := range values {
		if value == permission {
			return true
		}
	}
	return false
}

func createAuditHTTPTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	adminConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	adminConfig.Database = "postgres"
	adminConnection, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	databaseName := fmt.Sprintf("ause_audit_http_%d", time.Now().UnixNano())
	if _, err := adminConnection.Exec(ctx, "CREATE DATABASE "+databaseName); err != nil {
		adminConnection.Close(ctx)
		t.Fatalf("create temporary database: %v", err)
	}
	adminConnection.Close(ctx)

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse pool configuration: %v", err)
	}
	poolConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatalf("set migration dialect: %v", err)
	}
	sqlDatabase := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, sqlDatabase, "../../../migrations"); err != nil {
		sqlDatabase.Close()
		pool.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	sqlDatabase.Close()

	t.Cleanup(func() {
		pool.Close()
		adminConfig.Database = "postgres"
		connection, connectErr := pgx.ConnectConfig(ctx, adminConfig)
		if connectErr != nil {
			return
		}
		defer connection.Close(ctx)
		_, _ = connection.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)")
	})
	return pool
}

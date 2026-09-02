package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"ause-discovery.local/backend/generated"
	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/platform/database"
	"ause-discovery.local/backend/internal/platform/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MinimumPasswordLength = 12
	SessionTokenBytes     = 32
	CSRFTokenBytes        = 32
	LoginFailureLimit     = 5
	LoginFailureWindow    = 15 * time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrLoginThrottled     = errors.New("login throttled")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrCSRFInvalid        = errors.New("csrf token invalid")
)

type Service struct {
	Pool               *pgxpool.Pool
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	Now                func() time.Time
}

type Session struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Username          string
	Token             string
	CSRFToken         string
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

type Actor struct {
	UserID   uuid.UUID
	Username string
	Session  Session
}

func (service Service) CreateUser(ctx context.Context, username, password string) (uuid.UUID, error) {
	normalizedUsername := NormalizeUsername(username)
	if normalizedUsername == "" {
		return uuid.Nil, errors.New("username is required")
	}
	if err := ValidatePassword(password); err != nil {
		return uuid.Nil, err
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}
	userID := uuid.Must(uuid.NewV7())
	err = database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		queries := generated.New(transaction)
		if _, err := queries.CreateApplicationUser(ctx, generated.CreateApplicationUserParams{ID: identity.UUID(userID), Username: normalizedUsername}); err != nil {
			return err
		}
		if _, err := queries.CreateLocalCredential(ctx, generated.CreateLocalCredentialParams{UserID: identity.UUID(userID), PasswordHash: passwordHash}); err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{EventType: "admin.created", TargetType: "application_user", TargetID: userID, Metadata: map[string]any{"username": normalizedUsername}})
	})
	return userID, err
}

func (service Service) ResetPassword(ctx context.Context, username, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return err
	}
	normalizedUsername := NormalizeUsername(username)
	return database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		queries := generated.New(transaction)
		user, err := queries.GetApplicationUserByUsername(ctx, normalizedUsername)
		if err != nil {
			return err
		}
		if _, err := queries.UpdateLocalCredential(ctx, generated.UpdateLocalCredentialParams{UserID: user.ID, PasswordHash: passwordHash}); err != nil {
			return err
		}
		if _, err := transaction.Exec(ctx, "UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL", user.ID); err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: identity.UUIDValue(user.ID), EventType: "admin.password_reset", TargetType: "application_user", TargetID: identity.UUIDValue(user.ID), Metadata: map[string]any{"username": normalizedUsername}})
	})
}

func (service Service) DisableUser(ctx context.Context, username string) error {
	normalizedUsername := NormalizeUsername(username)
	return database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		queries := generated.New(transaction)
		user, err := queries.GetApplicationUserByUsername(ctx, normalizedUsername)
		if err != nil {
			return err
		}
		if _, err := queries.DisableApplicationUser(ctx, generated.DisableApplicationUserParams{ID: user.ID, Revision: user.Revision}); err != nil {
			return err
		}
		if _, err := transaction.Exec(ctx, "UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL", user.ID); err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: identity.UUIDValue(user.ID), EventType: "admin.disabled", TargetType: "application_user", TargetID: identity.UUIDValue(user.ID), Metadata: map[string]any{"username": normalizedUsername}})
	})
}

func (service Service) Login(ctx context.Context, username, password string, sourceIP netip.Addr) (Session, error) {
	normalizedUsername := NormalizeUsername(username)
	queries := generated.New(service.Pool)
	now := service.now()
	failures, err := queries.CountRecentLoginFailures(ctx, generated.CountRecentLoginFailuresParams{Username: normalizedUsername, SourceIp: sourceIP, AttemptedAt: pgtype.Timestamptz{Time: now.Add(-LoginFailureWindow), Valid: true}})
	if err != nil {
		return Session{}, err
	}
	if failures >= LoginFailureLimit {
		return Session{}, ErrLoginThrottled
	}
	credential, err := queries.GetCredentialByUsername(ctx, normalizedUsername)
	if err != nil || credential.Status != "active" || !VerifyPassword(credential.PasswordHash, password) {
		_ = database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
			q := generated.New(transaction)
			if insertErr := q.InsertLoginFailure(ctx, generated.InsertLoginFailureParams{Username: normalizedUsername, SourceIp: sourceIP}); insertErr != nil {
				return insertErr
			}
			return audit.AppendTx(ctx, transaction, audit.Event{EventType: "login.failure", TargetType: "application_user", Metadata: map[string]any{"username": normalizedUsername, "source_ip": sourceIP.String()}})
		})
		return Session{}, ErrInvalidCredentials
	}
	session, err := service.newSession(identity.UUIDValue(credential.ID), credential.Username)
	if err != nil {
		return Session{}, err
	}
	err = database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		if _, err := transaction.Exec(ctx, "UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL", credential.ID); err != nil {
			return err
		}
		queries := generated.New(transaction)
		if _, err := queries.CreateSession(ctx, generated.CreateSessionParams{ID: identity.UUID(session.ID), UserID: credential.ID, TokenHash: tokenHash(session.Token), CsrfTokenHash: tokenHash(session.CSRFToken), IdleExpiresAt: pgtype.Timestamptz{Time: session.IdleExpiresAt, Valid: true}, AbsoluteExpiresAt: pgtype.Timestamptz{Time: session.AbsoluteExpiresAt, Valid: true}}); err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: session.UserID, EventType: "login.success", TargetType: "application_user", TargetID: session.UserID, Metadata: map[string]any{"source_ip": sourceIP.String()}})
	})
	return session, err
}

func (service Service) Authenticate(ctx context.Context, rawToken string) (Actor, error) {
	if rawToken == "" {
		return Actor{}, ErrUnauthorized
	}
	var actor Actor
	err := database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		queries := generated.New(transaction)
		record, err := queries.GetActiveSessionForUpdate(ctx, tokenHash(rawToken))
		if err != nil || record.UserStatus != "active" {
			return ErrUnauthorized
		}
		now := service.now()
		idleExpiry := now.Add(service.idleTTL())
		absoluteExpiry := record.AbsoluteExpiresAt.Time
		if idleExpiry.After(absoluteExpiry) {
			idleExpiry = absoluteExpiry
		}
		updated, err := queries.TouchSession(ctx, generated.TouchSessionParams{ID: record.ID, IdleExpiresAt: pgtype.Timestamptz{Time: idleExpiry, Valid: true}})
		if err != nil {
			return err
		}
		user, err := queries.GetApplicationUserByID(ctx, record.UserID)
		if err != nil {
			return err
		}
		actor = Actor{UserID: identity.UUIDValue(record.UserID), Username: user.Username, Session: Session{ID: identity.UUIDValue(updated.ID), UserID: identity.UUIDValue(updated.UserID), IdleExpiresAt: updated.IdleExpiresAt.Time, AbsoluteExpiresAt: updated.AbsoluteExpiresAt.Time}}
		return nil
	})
	return actor, err
}

func (service Service) VerifyCSRF(ctx context.Context, sessionID uuid.UUID, token string) error {
	if token == "" {
		return ErrCSRFInvalid
	}
	var expected []byte
	err := service.Pool.QueryRow(ctx, "SELECT csrf_token_hash FROM sessions WHERE id = $1 AND revoked_at IS NULL AND idle_expires_at > now() AND absolute_expires_at > now()", identity.UUID(sessionID)).Scan(&expected)
	if err != nil || subtle.ConstantTimeCompare(expected, tokenHash(token)) != 1 {
		return ErrCSRFInvalid
	}
	return nil
}

func (service Service) Revoke(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID) error {
	return database.InTransaction(ctx, service.Pool, func(transaction pgx.Tx) error {
		queries := generated.New(transaction)
		if _, err := queries.RevokeSession(ctx, identity.UUID(sessionID)); err != nil {
			return err
		}
		return audit.AppendTx(ctx, transaction, audit.Event{ActorID: actorID, EventType: "logout", TargetType: "session", TargetID: sessionID})
	})
}

func (service Service) newSession(userID uuid.UUID, username string) (Session, error) {
	token, err := randomToken(SessionTokenBytes)
	if err != nil {
		return Session{}, err
	}
	csrfToken, err := randomToken(CSRFTokenBytes)
	if err != nil {
		return Session{}, err
	}
	now := service.now()
	return Session{ID: uuid.Must(uuid.NewV7()), UserID: userID, Username: username, Token: token, CSRFToken: csrfToken, IdleExpiresAt: now.Add(service.idleTTL()), AbsoluteExpiresAt: now.Add(service.absoluteTTL())}, nil
}

func (service Service) now() time.Time {
	if service.Now != nil {
		return service.Now().UTC()
	}
	return time.Now().UTC()
}
func (service Service) idleTTL() time.Duration {
	if service.SessionIdleTTL > 0 {
		return service.SessionIdleTTL
	}
	return 30 * time.Minute
}
func (service Service) absoluteTTL() time.Duration {
	if service.SessionAbsoluteTTL > 0 {
		return service.SessionAbsoluteTTL
	}
	return 12 * time.Hour
}
func NormalizeUsername(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func ValidatePassword(value string) error {
	if len([]rune(value)) < MinimumPasswordLength {
		return fmt.Errorf("password must contain at least %d characters", MinimumPasswordLength)
	}
	return nil
}
func randomToken(length int) (string, error) {
	value := make([]byte, length)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func tokenHash(value string) []byte { sum := sha256.Sum256([]byte(value)); return sum[:] }

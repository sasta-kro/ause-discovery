package httpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	api "ause-discovery.local/backend/generated/api"
	"ause-discovery.local/backend/internal/artifacts"
	"ause-discovery.local/backend/internal/audit"
	"ause-discovery.local/backend/internal/auth"
	importservice "ause-discovery.local/backend/internal/imports"
	"ause-discovery.local/backend/internal/people"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/platform/pagecursor"
	"ause-discovery.local/backend/internal/projects"
	searchservice "ause-discovery.local/backend/internal/search"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sessionCookieName = "ause_session"
const csrfCookieName = "ause_csrf"

var administratorPermissions = []string{"project.create", "project.edit", "project.delete", "person.create", "person.edit", "import.execute", "audit.read"}

type Controller struct {
	api.Unimplemented
	Audit     audit.Service
	Auth      auth.Service
	Artifacts artifacts.Service
	Imports   importservice.Service
	People    people.Service
	Projects  projects.Service
	Search    searchservice.Service
	Config    config.Config
}
type requestContextKey string

const actorContextKey requestContextKey = "actor"

func NewAPIHandler(pool *pgxpool.Pool, configuration config.Config) http.Handler {
	controller := Controller{
		Audit:     audit.Service{Pool: pool},
		Auth:      auth.Service{Pool: pool, SessionIdleTTL: configuration.SessionIdleTTL, SessionAbsoluteTTL: configuration.SessionAbsoluteTTL},
		Artifacts: artifacts.Service{Pool: pool, Storage: artifacts.Storage{Root: configuration.ArtifactRoot, MaxBytes: configuration.MaxArtifactBytes}, MaxProjectBytes: configuration.MaxProjectArtifactBytes},
		Imports:   importservice.Service{Pool: pool, TemporaryRoot: configuration.ImportTemporaryRoot},
		People:    people.Service{Pool: pool},
		Projects:  projects.Service{Pool: pool},
		Search:    searchservice.Service{Pool: pool, Index: searchservice.MeilisearchClient{BaseURL: configuration.MeilisearchURL, APIKey: configuration.MeilisearchAPIKey, TaskTimeout: 10 * time.Second}, IndexUID: configuration.MeilisearchIndex},
		Config:    configuration,
	}
	router := chi.NewRouter()
	router.Use(requestID, securityHeaders)
	baseURL := strings.TrimSuffix(configuration.PublicBasePath, "/") + "/api/v1"
	return api.HandlerWithOptions(&controller, api.ChiServerOptions{
		BaseRouter: router,
		BaseURL:    baseURL,
		ErrorHandlerFunc: func(writer http.ResponseWriter, request *http.Request, err error) {
			problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "Invalid request parameters.")
		},
	})
}

func (controller *Controller) Login(writer http.ResponseWriter, request *http.Request) {
	var body api.LoginRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	session, err := controller.Auth.Login(request.Context(), body.Username, body.Password, clientIP(request, controller.Config))
	if errors.Is(err, auth.ErrInvalidCredentials) {
		problem(writer, request, http.StatusUnauthorized, "unauthorized", "Unauthorized", "Invalid credentials.")
		return
	}
	if errors.Is(err, auth.ErrLoginThrottled) {
		problem(writer, request, http.StatusTooManyRequests, "login_throttled", "Too many requests", "Try again later.")
		return
	}
	if err != nil {
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: session.Token, Path: controller.Config.PublicBasePath, HttpOnly: true, Secure: controller.Config.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: session.AbsoluteExpiresAt})
	http.SetCookie(writer, &http.Cookie{Name: csrfCookieName, Value: session.CSRFToken, Path: controller.Config.PublicBasePath, Secure: controller.Config.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: session.AbsoluteExpiresAt})
	writeJSON(writer, http.StatusOK, api.SessionResponse{ExpiresAt: session.AbsoluteExpiresAt, User: api.SessionUser{Id: session.UserID, Username: session.Username, Permissions: administratorPermissions}})
}
func (controller *Controller) GetSession(writer http.ResponseWriter, request *http.Request) {
	actor, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	writeJSON(writer, http.StatusOK, api.SessionResponse{ExpiresAt: actor.Session.AbsoluteExpiresAt, User: api.SessionUser{Id: actor.UserID, Username: actor.Username, Permissions: administratorPermissions}})
}
func (controller *Controller) GetCsrfToken(writer http.ResponseWriter, request *http.Request) {
	actor, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	cookie, err := request.Cookie(csrfCookieName)
	if err != nil || controller.Auth.VerifyCSRF(request.Context(), actor.Session.ID, cookie.Value) != nil {
		problem(writer, request, http.StatusUnauthorized, "unauthorized", "Unauthorized", "")
		return
	}
	writeJSON(writer, http.StatusOK, api.CsrfTokenResponse{Token: cookie.Value})
}
func (controller *Controller) Logout(writer http.ResponseWriter, request *http.Request, params api.LogoutParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	if err := controller.Auth.Revoke(request.Context(), actor.Session.ID, actor.UserID); err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: "", Path: controller.Config.PublicBasePath, HttpOnly: true, Secure: controller.Config.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(writer, &http.Cookie{Name: csrfCookieName, Value: "", Path: controller.Config.PublicBasePath, Secure: controller.Config.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	writer.WriteHeader(http.StatusNoContent)
}
func (controller *Controller) CreatePerson(writer http.ResponseWriter, request *http.Request, _ api.CreatePersonParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.CreatePersonRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	person, err := controller.People.Create(request.Context(), actor.UserID, people.Input{DisplayName: body.DisplayName, StudentID: body.StudentId, StaffID: body.StaffId})
	if err != nil {
		problem(writer, request, 400, "validation_error", "Validation error", err.Error())
		return
	}
	writeJSON(writer, 201, personResponse(person))
}
func (controller *Controller) UpdatePerson(writer http.ResponseWriter, request *http.Request, personID api.PersonId, _ api.UpdatePersonParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.UpdatePersonRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	person, err := controller.People.Update(request.Context(), actor.UserID, uuid.UUID(personID), int64(body.ExpectedRevision), people.Input{DisplayName: body.DisplayName, StudentID: body.StudentId, StaffID: body.StaffId})
	if errors.Is(err, people.ErrConflict) {
		revision, _ := people.CurrentRevision(request.Context(), controller.People.Pool, uuid.UUID(personID))
		revisionConflict(writer, request, revision)
		return
	}
	if errors.Is(err, people.ErrNotFound) {
		problem(writer, request, 404, "not_found", "Not found", "")
		return
	}
	if err != nil {
		problem(writer, request, 400, "validation_error", "Validation error", err.Error())
		return
	}
	writeJSON(writer, 200, personResponse(person))
}
func (controller *Controller) GetAdminPerson(writer http.ResponseWriter, request *http.Request, personID api.PersonId) {
	_, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	person, err := controller.People.Get(request.Context(), uuid.UUID(personID))
	if errors.Is(err, people.ErrNotFound) {
		problem(writer, request, 404, "not_found", "Not found", "")
		return
	}
	if err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	writeJSON(writer, 200, personResponse(person))
}
func (controller *Controller) ListAdminPeople(writer http.ResponseWriter, request *http.Request, params api.ListAdminPeopleParams) {
	_, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	query := ""
	if params.Q != nil {
		query = *params.Q
	}
	cursor := ""
	if params.Cursor != nil {
		cursor = string(*params.Cursor)
	}
	page, err := controller.People.List(request.Context(), query, valueOrZero(params.Limit), cursor)
	if errors.Is(err, pagecursor.ErrInvalid) {
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "The People list cursor is invalid.")
		return
	}
	if err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	response := api.AdminPersonPage{Items: []api.AdminPerson{}, Page: api.PageInfo{Limit: page.Limit, NextCursor: page.NextCursor}}
	for _, item := range page.Items {
		response.Items = append(response.Items, personResponse(item))
	}
	writeJSON(writer, 200, response)
}

func (controller *Controller) CreateImport(writer http.ResponseWriter, request *http.Request, _ api.CreateImportParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	file, fileHeader, cleanup, ok := readImportMultipart(writer, request)
	if !ok {
		return
	}
	defer cleanup()
	defer file.Close()
	batch, err := controller.Imports.CreatePreview(request.Context(), actor.UserID, fileHeader.Filename, fileHeader.Size, file)
	if err != nil {
		controller.writeImportError(writer, request, uuid.Nil, err)
		return
	}
	writeJSON(writer, http.StatusCreated, importBatchResponse(batch))
}

func (controller *Controller) GetImport(writer http.ResponseWriter, request *http.Request, batchID api.BatchId) {
	if _, ok := controller.requireActor(writer, request, false); !ok {
		return
	}
	batch, err := controller.Imports.Get(request.Context(), uuid.UUID(batchID))
	if err != nil {
		controller.writeImportError(writer, request, uuid.UUID(batchID), err)
		return
	}
	writeJSON(writer, http.StatusOK, importBatchResponse(batch))
}

func (controller *Controller) ListImportRows(writer http.ResponseWriter, request *http.Request, batchID api.BatchId, params api.ListImportRowsParams) {
	if _, ok := controller.requireActor(writer, request, false); !ok {
		return
	}
	offset, err := decodeImportCursor(params.Cursor)
	if err != nil {
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "The import row cursor is invalid.")
		return
	}
	batch, err := controller.Imports.Get(request.Context(), uuid.UUID(batchID))
	if err != nil {
		controller.writeImportError(writer, request, uuid.UUID(batchID), err)
		return
	}
	pageLimit := limit(params.Limit)
	rows, err := controller.Imports.ListRows(request.Context(), uuid.UUID(batchID), pageLimit, offset)
	if err != nil {
		controller.writeImportError(writer, request, uuid.UUID(batchID), err)
		return
	}
	response := api.ImportRowPage{Items: []api.ImportRow{}, Page: api.PageInfo{Limit: pageLimit}}
	for _, row := range rows {
		item, responseErr := controller.importRowResponse(request.Context(), row)
		if responseErr != nil {
			problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
			return
		}
		response.Items = append(response.Items, item)
	}
	if offset+len(rows) < batch.TotalRows {
		response.Page.NextCursor = encodeImportCursor(offset + len(rows))
	}
	writeJSON(writer, http.StatusOK, response)
}

func (controller *Controller) UpdateImportRows(writer http.ResponseWriter, request *http.Request, batchID api.BatchId, _ api.UpdateImportRowsParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.UpdateImportRowsRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	updates := make([]importservice.RowUpdate, 0, len(body.Rows))
	for _, row := range body.Rows {
		var resolution *string
		if row.DuplicateResolution != nil {
			value := string(*row.DuplicateResolution)
			resolution = &value
		}
		updates = append(updates, importservice.RowUpdate{RowNumber: row.RowNumber, Selected: row.Selected, AcknowledgeWarnings: row.AcknowledgeWarnings, DuplicateResolution: resolution})
	}
	batch, err := controller.Imports.UpdateRows(request.Context(), actor.UserID, uuid.UUID(batchID), int64(body.ExpectedRevision), updates)
	if err != nil {
		controller.writeImportError(writer, request, uuid.UUID(batchID), err)
		return
	}
	writeJSON(writer, http.StatusOK, importBatchResponse(batch))
}

func (controller *Controller) CommitImport(writer http.ResponseWriter, request *http.Request, batchID api.BatchId, _ api.CommitImportParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.ExpectedRevisionRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := controller.Imports.Commit(request.Context(), actor.UserID, uuid.UUID(batchID), int64(body.ExpectedRevision))
	if err != nil {
		controller.writeImportError(writer, request, uuid.UUID(batchID), err)
		return
	}
	writeJSON(writer, http.StatusOK, importCommitResponse(result))
}

func (controller *Controller) GetImportResult(writer http.ResponseWriter, request *http.Request, batchID api.BatchId) {
	if _, ok := controller.requireActor(writer, request, false); !ok {
		return
	}
	result, err := controller.Imports.GetResult(request.Context(), uuid.UUID(batchID))
	if err != nil {
		controller.writeImportError(writer, request, uuid.UUID(batchID), err)
		return
	}
	writeJSON(writer, http.StatusOK, importCommitResponse(result))
}

func (controller *Controller) UploadArtifact(writer http.ResponseWriter, request *http.Request, projectID api.ProjectId, _ api.UploadArtifactParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	form, file, fileHeader, cleanup, ok := controller.readArtifactMultipart(writer, request, "expected_project_revision", "artifact_type", "display_name")
	if !ok {
		return
	}
	defer cleanup()
	defer file.Close()
	expectedRevision, err := parseRevision(form["expected_project_revision"])
	if err != nil {
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", err.Error())
		return
	}
	result, err := controller.Artifacts.Upload(request.Context(), actor.UserID, uuid.UUID(projectID), expectedRevision, artifacts.UploadInput{
		ArtifactType: form["artifact_type"], DisplayName: form["display_name"], OriginalFilename: fileHeader.Filename, ExpectedSize: fileHeader.Size, Content: file,
	})
	if errors.Is(err, artifacts.ErrRevisionConflict) {
		project, projectErr := controller.Projects.Get(request.Context(), uuid.UUID(projectID), false)
		if projectErr == nil {
			revisionConflict(writer, request, project.Revision)
		} else {
			problem(writer, request, http.StatusConflict, "revision_conflict", "Revision conflict", "")
		}
		return
	}
	if controller.writeArtifactError(writer, request, uuid.Nil, err) {
		return
	}
	writeJSON(writer, http.StatusCreated, artifactResponse(result, controller.Config.PublicBasePath))
}

func (controller *Controller) UpdateArtifact(writer http.ResponseWriter, request *http.Request, artifactID api.ArtifactId, _ api.UpdateArtifactParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.UpdateArtifactRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := controller.Artifacts.Update(request.Context(), actor.UserID, uuid.UUID(artifactID), int64(body.ExpectedRevision), string(body.ArtifactType), body.DisplayName)
	if controller.writeArtifactError(writer, request, uuid.UUID(artifactID), err) {
		return
	}
	writeJSON(writer, http.StatusOK, artifactResponse(result, controller.Config.PublicBasePath))
}

func (controller *Controller) DeleteArtifact(writer http.ResponseWriter, request *http.Request, artifactID api.ArtifactId, _ api.DeleteArtifactParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.ExpectedRevisionRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := controller.Artifacts.Delete(request.Context(), actor.UserID, uuid.UUID(artifactID), int64(body.ExpectedRevision))
	if controller.writeArtifactError(writer, request, uuid.UUID(artifactID), err) {
		return
	}
	writeJSON(writer, http.StatusOK, artifactResponse(result, controller.Config.PublicBasePath))
}

func (controller *Controller) RestoreArtifact(writer http.ResponseWriter, request *http.Request, artifactID api.ArtifactId, _ api.RestoreArtifactParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.ExpectedRevisionRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := controller.Artifacts.Restore(request.Context(), actor.UserID, uuid.UUID(artifactID), int64(body.ExpectedRevision))
	if controller.writeArtifactError(writer, request, uuid.UUID(artifactID), err) {
		return
	}
	writeJSON(writer, http.StatusOK, artifactResponse(result, controller.Config.PublicBasePath))
}

func (controller *Controller) ReplaceArtifact(writer http.ResponseWriter, request *http.Request, artifactID api.ArtifactId, _ api.ReplaceArtifactParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	form, file, fileHeader, cleanup, ok := controller.readArtifactMultipart(writer, request, "expected_revision")
	if !ok {
		return
	}
	defer cleanup()
	defer file.Close()
	expectedRevision, err := parseRevision(form["expected_revision"])
	if err != nil {
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", err.Error())
		return
	}
	result, err := controller.Artifacts.Replace(request.Context(), actor.UserID, uuid.UUID(artifactID), expectedRevision, artifacts.ReplaceInput{OriginalFilename: fileHeader.Filename, ExpectedSize: fileHeader.Size, Content: file})
	if controller.writeArtifactError(writer, request, uuid.UUID(artifactID), err) {
		return
	}
	writeJSON(writer, http.StatusOK, artifactResponse(result, controller.Config.PublicBasePath))
}

func (controller *Controller) ViewArtifact(writer http.ResponseWriter, request *http.Request, artifactID api.ArtifactId) {
	controller.serveArtifact(writer, request, uuid.UUID(artifactID), true)
}

func (controller *Controller) DownloadArtifact(writer http.ResponseWriter, request *http.Request, artifactID api.ArtifactId) {
	controller.serveArtifact(writer, request, uuid.UUID(artifactID), false)
}

func (controller *Controller) SearchProjects(writer http.ResponseWriter, request *http.Request, params api.SearchProjectsParams) {
	result, err := controller.Search.Search(request.Context(), searchQuery(params))
	if errors.Is(err, searchservice.ErrInvalidFilter) || errors.Is(err, searchservice.ErrInvalidCursor) {
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", err.Error())
		return
	}
	if errors.Is(err, searchservice.ErrSearchUnavailable) {
		problem(writer, request, http.StatusServiceUnavailable, "search_unavailable", "Search unavailable", "Project detail pages remain available.")
		return
	}
	if err != nil {
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
		return
	}
	writeJSON(writer, http.StatusOK, searchResponse(result))
}

func (controller *Controller) GetSearchStatus(writer http.ResponseWriter, request *http.Request) {
	_, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	status, err := controller.Search.SearchStatus(request.Context())
	if err != nil {
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
		return
	}
	response := api.SearchStatus{Available: status.Available, PendingCount: status.PendingCount, FailedCount: status.FailedCount}
	if status.ActiveRebuild != nil {
		active := searchRebuildResponse(*status.ActiveRebuild)
		response.ActiveRebuild = &active
	}
	writeJSON(writer, http.StatusOK, response)
}

func (controller *Controller) ReindexProject(writer http.ResponseWriter, request *http.Request, projectID api.ProjectId, _ api.ReindexProjectParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	result, err := controller.Search.ReindexProject(request.Context(), actor.UserID, uuid.UUID(projectID))
	if errors.Is(err, searchservice.ErrProjectNotFound) {
		problem(writer, request, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	if err != nil {
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
		return
	}
	writeJSON(writer, http.StatusAccepted, api.ReindexProjectResponse{ProjectId: result.ProjectID, DesiredRevision: int(result.DesiredRevision), State: api.SearchSyncState(result.State)})
}

func (controller *Controller) CreateSearchRebuild(writer http.ResponseWriter, request *http.Request, _ api.CreateSearchRebuildParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	operation, err := controller.Search.CreateRebuild(request.Context(), actor.UserID)
	if errors.Is(err, searchservice.ErrRebuildActive) {
		problem(writer, request, http.StatusConflict, "search_rebuild_active", "Search rebuild already active", "")
		return
	}
	if err != nil {
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
		return
	}
	writeJSON(writer, http.StatusAccepted, searchRebuildResponse(operation))
}

func (controller *Controller) GetSearchRebuild(writer http.ResponseWriter, request *http.Request, operationID api.OperationId) {
	_, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	operation, err := controller.Search.GetRebuild(request.Context(), uuid.UUID(operationID))
	if errors.Is(err, searchservice.ErrRebuildNotFound) {
		problem(writer, request, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	if err != nil {
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
		return
	}
	writeJSON(writer, http.StatusOK, searchRebuildResponse(operation))
}

func (controller *Controller) ListAuditEvents(writer http.ResponseWriter, request *http.Request, params api.ListAuditEventsParams) {
	_, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	query := audit.PageQuery{Cursor: cursorValue(params.Cursor), Limit: valueOrZero(params.Limit)}
	if params.Action != nil {
		query.Action = *params.Action
	}
	if params.ActorId != nil {
		actorID := uuid.UUID(*params.ActorId)
		query.ActorID = &actorID
	}
	page, err := controller.Audit.List(request.Context(), query)
	if errors.Is(err, audit.ErrInvalidCursor) {
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "The audit cursor is invalid.")
		return
	}
	if err != nil {
		slog.Error("Audit listing failed", "error", err, "request_id", requestIDValue(request.Context()))
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
		return
	}
	response := api.AuditEventPage{Items: []api.AuditEvent{}, Page: api.PageInfo{Limit: page.Limit, NextCursor: page.NextCursor}}
	for _, item := range page.Items {
		response.Items = append(response.Items, auditEventResponse(item))
	}
	writeJSON(writer, http.StatusOK, response)
}

func (controller *Controller) CreateProject(writer http.ResponseWriter, request *http.Request, _ api.CreateProjectParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.CreateProjectRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	project, err := controller.Projects.Create(request.Context(), actor.UserID, projectInput(body))
	if err != nil {
		problem(writer, request, 400, "validation_error", "Validation error", err.Error())
		return
	}
	controller.writeProjectResult(writer, request, project, nil, http.StatusCreated)
}
func (controller *Controller) ReplaceProject(writer http.ResponseWriter, request *http.Request, projectID api.ProjectId, _ api.ReplaceProjectParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.ReplaceProjectRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	project, err := controller.Projects.Replace(request.Context(), actor.UserID, uuid.UUID(projectID), int64(body.ExpectedRevision), projectInputReplace(body))
	controller.writeProjectResult(writer, request, project, err, 200)
}
func (controller *Controller) GetAdminProject(writer http.ResponseWriter, request *http.Request, projectID api.ProjectId) {
	_, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	project, err := controller.Projects.Get(request.Context(), uuid.UUID(projectID), false)
	controller.writeProjectResult(writer, request, project, err, 200)
}
func (controller *Controller) ListAdminProjects(writer http.ResponseWriter, request *http.Request, params api.ListAdminProjectsParams) {
	_, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	var status *string
	if params.Status != nil {
		value := string(*params.Status)
		status = &value
	}
	query := ""
	if params.Q != nil {
		query = *params.Q
	}
	cursor := ""
	if params.Cursor != nil {
		cursor = string(*params.Cursor)
	}
	page, err := controller.Projects.List(request.Context(), status, query, valueOrZero(params.Limit), cursor)
	if errors.Is(err, pagecursor.ErrInvalid) {
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "The Projects list cursor is invalid.")
		return
	}
	if err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	response := api.AdminProjectPage{Items: []api.AdminProject{}, Page: api.PageInfo{Limit: page.Limit, NextCursor: page.NextCursor}}
	for _, item := range page.Items {
		responseItem, responseErr := controller.adminProjectResponse(request.Context(), item)
		if responseErr != nil {
			problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
			return
		}
		response.Items = append(response.Items, responseItem)
	}
	writeJSON(writer, 200, response)
}
func (controller *Controller) PublishProject(writer http.ResponseWriter, request *http.Request, projectID api.ProjectId, _ api.PublishProjectParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.ExpectedRevisionRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	project, err := controller.Projects.Publish(request.Context(), actor.UserID, uuid.UUID(projectID), int64(body.ExpectedRevision))
	controller.writeProjectResult(writer, request, project, err, 200)
}
func (controller *Controller) DeleteProject(writer http.ResponseWriter, request *http.Request, projectID api.ProjectId, _ api.DeleteProjectParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.DeleteProjectRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	project, err := controller.Projects.Delete(request.Context(), actor.UserID, uuid.UUID(projectID), int64(body.ExpectedRevision), body.Confirmation)
	controller.writeProjectResult(writer, request, project, err, 200)
}
func (controller *Controller) RestoreProject(writer http.ResponseWriter, request *http.Request, projectID api.ProjectId, _ api.RestoreProjectParams) {
	actor, ok := controller.requireActor(writer, request, true)
	if !ok {
		return
	}
	var body api.ExpectedRevisionRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	project, err := controller.Projects.Restore(request.Context(), actor.UserID, uuid.UUID(projectID), int64(body.ExpectedRevision))
	controller.writeProjectResult(writer, request, project, err, 200)
}
func (controller *Controller) GetPublicProject(writer http.ResponseWriter, request *http.Request, projectID api.ProjectId) {
	project, err := controller.Projects.Get(request.Context(), uuid.UUID(projectID), true)
	if errors.Is(err, projects.ErrNotFound) {
		problem(writer, request, 404, "not_found", "Not found", "")
		return
	}
	if err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	response, err := controller.publicProjectResponse(request.Context(), project)
	if err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	writeJSON(writer, 200, response)
}
func (controller *Controller) GetPublicPerson(writer http.ResponseWriter, request *http.Request, personID api.PersonId) {
	person, err := controller.People.Get(request.Context(), uuid.UUID(personID))
	if errors.Is(err, people.ErrNotFound) {
		problem(writer, request, 404, "not_found", "Not found", "")
		return
	}
	if err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	rows, err := controller.Projects.Pool.Query(request.Context(), `SELECT projects.id, project_participations.role FROM project_participations JOIN projects ON projects.id = project_participations.project_id WHERE project_participations.person_id = $1 AND projects.status = 'published' ORDER BY projects.published_at DESC, projects.id`, uuid.UUID(personID))
	if err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	defer rows.Close()
	response := api.PublicPerson{Id: person.ID, DisplayName: person.DisplayName, StudentId: person.StudentID, Projects: []api.PublicPersonProject{}}
	for rows.Next() {
		var projectID uuid.UUID
		var role string
		if err := rows.Scan(&projectID, &role); err != nil {
			problem(writer, request, 500, "internal_error", "Internal server error", "")
			return
		}
		project, err := controller.Projects.Get(request.Context(), projectID, true)
		if err != nil {
			problem(writer, request, 500, "internal_error", "Internal server error", "")
			return
		}
		projectResponse, err := controller.publicProjectResponse(request.Context(), project)
		if err != nil {
			problem(writer, request, 500, "internal_error", "Internal server error", "")
			return
		}
		response.Projects = append(response.Projects, api.PublicPersonProject{Id: projectResponse.Id, Title: projectResponse.Title, ReferenceCode: projectResponse.ReferenceCode, AcademicYear: projectResponse.AcademicYear, Semester: projectResponse.Semester, Program: projectResponse.Program, Major: projectResponse.Major, Course: projectResponse.Course, PublishedAt: projectResponse.PublishedAt, Role: api.ParticipationRole(role), ArtifactCount: len(projectResponse.Artifacts), Categories: filterTaxonomy(projectResponse.Taxonomy, "category"), Platforms: filterTaxonomy(projectResponse.Taxonomy, "platform"), People: projectResponse.Participations})
	}
	if err := rows.Err(); err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	writeJSON(writer, 200, response)
}
func (controller *Controller) writeProjectResult(writer http.ResponseWriter, request *http.Request, project projects.Project, err error, status int) {
	if errors.Is(err, projects.ErrNotFound) {
		problem(writer, request, 404, "not_found", "Not found", "")
		return
	}
	if errors.Is(err, projects.ErrRevisionConflict) {
		revisionConflict(writer, request, project.Revision)
		return
	}
	if errors.Is(err, projects.ErrInvalidConfirmation) {
		problem(writer, request, 400, "confirmation_mismatch", "Validation error", err.Error())
		return
	}
	if err != nil {
		problem(writer, request, 400, "validation_error", "Validation error", err.Error())
		return
	}
	response, responseErr := controller.adminProjectResponse(request.Context(), project)
	if responseErr != nil {
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
		return
	}
	writeJSON(writer, status, response)
}
func (controller *Controller) requireActor(writer http.ResponseWriter, request *http.Request, csrf bool) (auth.Actor, bool) {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil {
		problem(writer, request, 401, "unauthorized", "Unauthorized", "")
		return auth.Actor{}, false
	}
	actor, err := controller.Auth.Authenticate(request.Context(), cookie.Value)
	if err != nil {
		problem(writer, request, 401, "unauthorized", "Unauthorized", "")
		return auth.Actor{}, false
	}
	if csrf {
		if !trustedOrigin(request, controller.Config) {
			problem(writer, request, 403, "forbidden", "Forbidden", "")
			return auth.Actor{}, false
		}
		if err := controller.Auth.VerifyCSRF(request.Context(), actor.Session.ID, request.Header.Get("X-CSRF-Token")); err != nil {
			problem(writer, request, 403, "csrf_invalid", "Forbidden", "")
			return auth.Actor{}, false
		}
	}
	return actor, true
}
func projectInput(value api.ProjectDraft) projects.Input {
	result := projects.Input{ReferenceCode: value.ReferenceCode, Title: value.Title, Abstract: value.Abstract, AcademicYear: value.AcademicYear, ExtensionMetadata: map[string]any{}}
	if value.Semester != nil {
		semester := string(*value.Semester)
		result.Semester = &semester
	}
	if value.ProgramVersionId != nil {
		id := uuid.UUID(*value.ProgramVersionId)
		result.ProgramVersionID = &id
	}
	if value.MajorVersionId != nil {
		id := uuid.UUID(*value.MajorVersionId)
		result.MajorVersionID = &id
	}
	if value.CourseVersionId != nil {
		id := uuid.UUID(*value.CourseVersionId)
		result.CourseVersionID = &id
	}
	if value.ExtensionMetadata != nil {
		result.ExtensionMetadata = *value.ExtensionMetadata
	}
	if value.TitleAliases != nil {
		result.TitleAliases = *value.TitleAliases
	}
	if value.Participations != nil {
		for _, item := range *value.Participations {
			result.Participations = append(result.Participations, projects.Participation{PersonID: uuid.UUID(item.PersonId), Role: string(item.Role), SortOrder: item.SortOrder})
		}
	}
	if value.TaxonomyValues != nil {
		for _, item := range *value.TaxonomyValues {
			result.TaxonomyValues = append(result.TaxonomyValues, projects.TaxonomyValue{ID: uuid.UUID(item.TaxonomyValueId), SortOrder: item.SortOrder})
		}
	}
	return result
}
func projectInputReplace(value api.ReplaceProjectRequest) projects.Input {
	return projects.Input{ReferenceCode: value.ReferenceCode, Title: value.Title, Abstract: value.Abstract, AcademicYear: value.AcademicYear, Semester: semesterInput(value.Semester), ProgramVersionID: uuidInput(value.ProgramVersionId), MajorVersionID: uuidInput(value.MajorVersionId), CourseVersionID: uuidInput(value.CourseVersionId), ExtensionMetadata: mapInput(value.ExtensionMetadata), TitleAliases: sliceInput(value.TitleAliases), Participations: participationInput(value.Participations), TaxonomyValues: taxonomyInput(value.TaxonomyValues)}
}
func semesterInput(value *api.Semester) *string {
	if value == nil {
		return nil
	}
	result := string(*value)
	return &result
}
func uuidInput(value *api.Uuid) *uuid.UUID {
	if value == nil {
		return nil
	}
	result := uuid.UUID(*value)
	return &result
}
func mapInput(value *map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return *value
}
func sliceInput(value *[]string) []string {
	if value == nil {
		return nil
	}
	return *value
}
func participationInput(value *[]api.ParticipationInput) []projects.Participation {
	if value == nil {
		return nil
	}
	result := []projects.Participation{}
	for _, item := range *value {
		result = append(result, projects.Participation{PersonID: uuid.UUID(item.PersonId), Role: string(item.Role), SortOrder: item.SortOrder})
	}
	return result
}
func taxonomyInput(value *[]api.TaxonomyAssignmentInput) []projects.TaxonomyValue {
	if value == nil {
		return nil
	}
	result := []projects.TaxonomyValue{}
	for _, item := range *value {
		result = append(result, projects.TaxonomyValue{ID: uuid.UUID(item.TaxonomyValueId), SortOrder: item.SortOrder})
	}
	return result
}
func personResponse(value people.Person) api.AdminPerson {
	return api.AdminPerson{Id: value.ID, DisplayName: value.DisplayName, StudentId: value.StudentID, StaffId: value.StaffID, Revision: int(value.Revision), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func (controller *Controller) adminProjectResponse(ctx context.Context, value projects.Project) (api.AdminProject, error) {
	program, err := controller.catalogReference(ctx, "program", value.ProgramVersionID)
	if err != nil {
		return api.AdminProject{}, err
	}
	major, err := controller.catalogReference(ctx, "major", value.MajorVersionID)
	if err != nil {
		return api.AdminProject{}, err
	}
	course, err := controller.catalogReference(ctx, "course", value.CourseVersionID)
	if err != nil {
		return api.AdminProject{}, err
	}
	participations, err := controller.projectParticipations(ctx, value.ID)
	if err != nil {
		return api.AdminProject{}, err
	}
	taxonomy, err := controller.projectTaxonomy(ctx, value.ID)
	if err != nil {
		return api.AdminProject{}, err
	}
	artifacts, err := controller.projectArtifacts(ctx, value.ID, false)
	if err != nil {
		return api.AdminProject{}, err
	}
	return adminProjectResponse(value, program, major, course, participations, taxonomy, artifacts), nil
}

func adminProjectResponse(value projects.Project, program, major, course *api.CatalogReference, participations []api.Participation, taxonomy []api.TaxonomyValue, artifacts []api.Artifact) api.AdminProject {
	var semester *api.Semester
	if value.Semester != nil {
		convertedSemester := api.Semester(*value.Semester)
		semester = &convertedSemester
	}
	extensionMetadata := value.ExtensionMetadata
	return api.AdminProject{Id: value.ID, ReferenceCode: value.ReferenceCode, Title: value.Title, Abstract: value.Abstract, AcademicYear: value.AcademicYear, Semester: semester, Program: program, Major: major, Course: course, Status: api.ProjectStatus(value.Status), Revision: int(value.Revision), TitleAliases: &value.TitleAliases, Artifacts: artifacts, Participations: participations, Taxonomy: taxonomy, ExtensionMetadata: &extensionMetadata, PublishedAt: value.PublishedAt, DeletedAt: value.DeletedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func (controller *Controller) publicProjectResponse(ctx context.Context, value projects.Project) (api.PublicProject, error) {
	program, err := controller.catalogReference(ctx, "program", value.ProgramVersionID)
	if err != nil {
		return api.PublicProject{}, err
	}
	major, err := controller.catalogReference(ctx, "major", value.MajorVersionID)
	if err != nil {
		return api.PublicProject{}, err
	}
	course, err := controller.catalogReference(ctx, "course", value.CourseVersionID)
	if err != nil {
		return api.PublicProject{}, err
	}
	participations, err := controller.projectParticipations(ctx, value.ID)
	if err != nil {
		return api.PublicProject{}, err
	}
	taxonomy, err := controller.projectTaxonomy(ctx, value.ID)
	if err != nil {
		return api.PublicProject{}, err
	}
	artifacts, err := controller.projectArtifacts(ctx, value.ID, true)
	if err != nil {
		return api.PublicProject{}, err
	}
	response := api.PublicProject{Id: value.ID, Title: valueOrEmpty(value.Title), Abstract: valueOrEmpty(value.Abstract), AcademicYear: valueOrZero(value.AcademicYear), Semester: api.Semester(valueOrEmpty(value.Semester)), ReferenceCode: value.ReferenceCode, TitleAliases: &value.TitleAliases, Artifacts: artifacts, Participations: participations, Taxonomy: taxonomy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, PublishedAt: publishedAt(value), Status: value.Status}
	if program != nil {
		response.Program = *program
	}
	if major != nil {
		response.Major = major
	}
	if course != nil {
		response.Course = *course
	}
	return response, nil
}

func (controller *Controller) catalogReference(ctx context.Context, kind string, versionID *uuid.UUID) (*api.CatalogReference, error) {
	if versionID == nil {
		return nil, nil
	}
	var reference api.CatalogReference
	var err error
	switch kind {
	case "program":
		err = controller.Projects.Pool.QueryRow(ctx, `SELECT program_versions.id, programs.key, program_versions.label FROM program_versions JOIN programs ON programs.id = program_versions.program_id WHERE program_versions.id = $1`, *versionID).Scan(&reference.Id, &reference.Key, &reference.Label)
	case "major":
		err = controller.Projects.Pool.QueryRow(ctx, `SELECT major_versions.id, majors.key, major_versions.label FROM major_versions JOIN majors ON majors.id = major_versions.major_id WHERE major_versions.id = $1`, *versionID).Scan(&reference.Id, &reference.Key, &reference.Label)
	case "course":
		err = controller.Projects.Pool.QueryRow(ctx, `SELECT course_versions.id, courses.key, course_versions.label FROM course_versions JOIN courses ON courses.id = course_versions.course_id WHERE course_versions.id = $1`, *versionID).Scan(&reference.Id, &reference.Key, &reference.Label)
	}
	if err != nil {
		return nil, err
	}
	return &reference, nil
}

func (controller *Controller) projectParticipations(ctx context.Context, projectID uuid.UUID) ([]api.Participation, error) {
	rows, err := controller.Projects.Pool.Query(ctx, `SELECT people.id, people.display_name, people.student_id, project_participations.role, project_participations.position FROM project_participations JOIN people ON people.id = project_participations.person_id WHERE project_participations.project_id = $1 ORDER BY project_participations.role, project_participations.position`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []api.Participation{}
	for rows.Next() {
		var value api.Participation
		if err := rows.Scan(&value.Person.Id, &value.Person.DisplayName, &value.Person.StudentId, &value.Role, &value.SortOrder); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (controller *Controller) projectTaxonomy(ctx context.Context, projectID uuid.UUID) ([]api.TaxonomyValue, error) {
	rows, err := controller.Projects.Pool.Query(ctx, `SELECT taxonomy_values.id, taxonomy_values.dimension, taxonomy_values.key, taxonomy_values.labels, taxonomy_values.description, taxonomy_values.sort_order FROM project_taxonomy_values JOIN taxonomy_values ON taxonomy_values.id = project_taxonomy_values.taxonomy_value_id WHERE project_taxonomy_values.project_id = $1 ORDER BY project_taxonomy_values.dimension, project_taxonomy_values.position`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []api.TaxonomyValue{}
	for rows.Next() {
		var value api.TaxonomyValue
		var labels []byte
		if err := rows.Scan(&value.Id, &value.Dimension, &value.Key, &labels, &value.Description, &value.SortOrder); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(labels, &value.Labels); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (controller *Controller) projectArtifacts(ctx context.Context, projectID uuid.UUID, activeOnly bool) ([]api.Artifact, error) {
	statement := `SELECT id, project_id, type, display_name, original_filename, mime_type, byte_count, status, revision, created_at, updated_at, deleted_at FROM artifacts WHERE project_id = $1`
	if activeOnly {
		statement += " AND status = 'active'"
	}
	statement += " ORDER BY created_at, id"
	rows, err := controller.Projects.Pool.Query(ctx, statement, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []api.Artifact{}
	baseURL := strings.TrimSuffix(controller.Config.PublicBasePath, "/") + "/api/v1/artifacts/"
	for rows.Next() {
		var value api.Artifact
		if err := rows.Scan(&value.Id, &value.ProjectId, &value.ArtifactType, &value.DisplayName, &value.OriginalFilename, &value.MimeType, &value.ByteCount, &value.Status, &value.Revision, &value.CreatedAt, &value.UpdatedAt, &value.DeletedAt); err != nil {
			return nil, err
		}
		if value.Status == api.ArtifactStatusActive {
			downloadURL := baseURL + value.Id.String() + "/download"
			value.DownloadUrl = &downloadURL
			if value.MimeType == "application/pdf" {
				viewURL := baseURL + value.Id.String() + "/view"
				value.ViewUrl = &viewURL
			}
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func filterTaxonomy(values []api.TaxonomyValue, dimension string) []api.TaxonomyValue {
	result := []api.TaxonomyValue{}
	for _, value := range values {
		if string(value.Dimension) == dimension {
			result = append(result, value)
		}
	}
	return result
}
func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
func publishedAt(value projects.Project) time.Time {
	if value.PublishedAt != nil {
		return *value.PublishedAt
	}
	return time.Now().UTC()
}

func searchQuery(params api.SearchProjectsParams) searchservice.Query {
	query := searchservice.Query{Limit: limit(params.Limit)}
	if params.Q != nil {
		query.Text = *params.Q
	}
	if params.AcademicYear != nil {
		academicYear := int(*params.AcademicYear)
		query.AcademicYear = &academicYear
	}
	if params.Semester != nil {
		semester := string(*params.Semester)
		query.Semester = &semester
	}
	if params.ProgramKey != nil {
		query.ProgramKeys = append([]string{}, (*params.ProgramKey)...)
	}
	if params.MajorKey != nil {
		query.MajorKeys = append([]string{}, (*params.MajorKey)...)
	}
	if params.CourseKey != nil {
		query.CourseKeys = append([]string{}, (*params.CourseKey)...)
	}
	if params.PersonId != nil {
		query.PersonIDs = append([]uuid.UUID{}, (*params.PersonId)...)
	}
	if params.StudentId != nil {
		studentID := string(*params.StudentId)
		query.StudentID = &studentID
	}
	if params.AdvisorId != nil {
		query.AdvisorIDs = append([]uuid.UUID{}, (*params.AdvisorId)...)
	}
	if params.CategoryKey != nil {
		query.CategoryKeys = append([]string{}, (*params.CategoryKey)...)
	}
	if params.PlatformKey != nil {
		query.PlatformKeys = append([]string{}, (*params.PlatformKey)...)
	}
	if params.DomainKey != nil {
		query.DomainKeys = append([]string{}, (*params.DomainKey)...)
	}
	if params.TopicKey != nil {
		query.TopicKeys = append([]string{}, (*params.TopicKey)...)
	}
	if params.TechnologyKey != nil {
		query.TechnologyKeys = append([]string{}, (*params.TechnologyKey)...)
	}
	if params.ArtifactType != nil {
		for _, artifactType := range *params.ArtifactType {
			query.ArtifactTypes = append(query.ArtifactTypes, string(artifactType))
		}
	}
	query.HasArtifacts = params.HasArtifacts
	query.HasReport = params.HasReport
	query.HasSlides = params.HasSlides
	query.HasSourceCode = params.HasSourceCode
	query.HasDataset = params.HasDataset
	if params.Sort != nil {
		query.Sort = string(*params.Sort)
	}
	if params.Cursor != nil {
		query.Cursor = string(*params.Cursor)
	}
	return query
}

func searchResponse(result searchservice.Result) api.SearchResponse {
	response := api.SearchResponse{
		Items: []api.SearchResult{},
		Page:  api.PageInfo{Limit: result.Limit, NextCursor: result.NextCursor},
		Total: result.Total,
		Facets: api.SearchFacets{
			Programs:      searchFacets(result.Facets.Programs),
			Majors:        searchFacets(result.Facets.Majors),
			Courses:       searchFacets(result.Facets.Courses),
			AcademicYears: searchFacets(result.Facets.AcademicYears),
			People:        searchFacets(result.Facets.People),
			Categories:    searchFacets(result.Facets.Categories),
			Platforms:     searchFacets(result.Facets.Platforms),
			Domains:       searchFacets(result.Facets.Domains),
			Topics:        searchFacets(result.Facets.Topics),
			Technologies:  searchFacets(result.Facets.Technologies),
		},
	}
	for _, item := range result.Items {
		searchResult := api.SearchResult{
			Id: item.ID, ReferenceCode: item.ReferenceCode, Title: item.Title, AcademicYear: item.AcademicYear,
			Semester: api.Semester(item.Semester), Program: searchCatalogReference(item.Program), Course: searchCatalogReference(item.Course),
			People: searchParticipations(item.People), Categories: searchTaxonomyValues(item.Categories), Platforms: searchTaxonomyValues(item.Platforms),
			ArtifactCount: item.ArtifactCount, PublishedAt: item.PublishedAt, Highlights: []api.SearchHighlight{},
		}
		if item.Major != nil {
			major := searchCatalogReference(*item.Major)
			searchResult.Major = &major
		}
		for _, highlight := range item.Highlights {
			searchResult.Highlights = append(searchResult.Highlights, api.SearchHighlight{Field: api.SearchHighlightField(highlight.Field), Value: highlight.Value})
		}
		response.Items = append(response.Items, searchResult)
	}
	return response
}

func searchCatalogReference(value searchservice.CatalogReference) api.CatalogReference {
	return api.CatalogReference{Id: value.ID, Key: value.Key, Label: value.Label}
}

func searchParticipations(values []searchservice.Participation) []api.Participation {
	result := make([]api.Participation, 0, len(values))
	for _, value := range values {
		result = append(result, api.Participation{
			Person: api.PersonSummary{Id: value.Person.ID, DisplayName: value.Person.DisplayName, StudentId: value.Person.StudentID},
			Role:   api.ParticipationRole(value.Role), SortOrder: value.SortOrder,
		})
	}
	return result
}

func searchTaxonomyValues(values []searchservice.TaxonomyValue) []api.TaxonomyValue {
	result := make([]api.TaxonomyValue, 0, len(values))
	for _, value := range values {
		result = append(result, api.TaxonomyValue{
			Id: value.ID, Dimension: api.TaxonomyDimension(value.Dimension), Key: value.Key, Labels: value.Labels,
			Description: value.Description, SortOrder: value.SortOrder,
		})
	}
	return result
}

func searchFacets(values []searchservice.Facet) []api.SearchFacet {
	result := make([]api.SearchFacet, 0, len(values))
	for _, value := range values {
		result = append(result, api.SearchFacet{Key: value.Key, Count: value.Count})
	}
	return result
}

func searchRebuildResponse(operation searchservice.RebuildOperation) api.SearchRebuildOperation {
	failedProjects := operation.FailedProjects
	return api.SearchRebuildOperation{
		Id: operation.ID, State: api.SearchRebuildState(operation.State), RequestedAt: operation.RequestedAt,
		RequestedBy: operation.RequestedBy, StartedAt: operation.StartedAt, CompletedAt: operation.CompletedAt,
		TotalProjects: operation.TotalProjects, ProcessedProjects: operation.ProcessedProjects,
		FailedProjects: &failedProjects, ErrorCode: operation.ErrorCode,
	}
}

func auditEventResponse(value audit.PageItem) api.AuditEvent {
	response := api.AuditEvent{Action: value.Action, ActorId: (*api.Uuid)(value.ActorID), CreatedAt: value.CreatedAt, Id: value.ID, Metadata: value.Metadata, ResourceType: value.TargetType, ResourceId: (*string)(nil)}
	if value.TargetID != nil {
		resourceID := value.TargetID.String()
		response.ResourceId = &resourceID
	}
	return response
}

func cursorValue(value *api.Cursor) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func artifactResponse(value artifacts.Artifact, publicBasePath string) api.Artifact {
	response := api.Artifact{
		Id: value.ID, ProjectId: value.ProjectID, ArtifactType: api.ArtifactType(value.ArtifactType), DisplayName: value.DisplayName,
		OriginalFilename: value.OriginalFilename, MimeType: value.MIMEType, ByteCount: int(value.ByteCount), Status: api.ArtifactStatus(value.Status),
		Revision: int(value.Revision), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, DeletedAt: value.DeletedAt,
	}
	if value.Status != "active" {
		return response
	}
	baseURL := strings.TrimSuffix(publicBasePath, "/") + "/api/v1/artifacts/" + value.ID.String()
	downloadURL := baseURL + "/download"
	response.DownloadUrl = &downloadURL
	if value.MIMEType == "application/pdf" && value.Extension == "pdf" {
		viewURL := baseURL + "/view"
		response.ViewUrl = &viewURL
	}
	return response
}

func importBatchResponse(value importservice.Batch) api.ImportBatch {
	return api.ImportBatch{
		Id: value.ID, SourceFilename: value.SourceFilename, SourceSha256: value.SourceSHA256,
		Format: api.ImportBatchFormat(value.Format), State: api.ImportState(value.State), TotalRows: value.TotalRows,
		ValidRows: value.ValidRows, WarningRows: value.WarningRows, ErrorRows: value.ErrorRows,
		Revision: int(value.Revision), CreatedAt: value.CreatedAt, ExpiresAt: value.ExpiresAt, CommittedAt: value.CommittedAt,
	}
}

func importCommitResponse(value importservice.CommitResult) api.ImportCommitResult {
	projectIDs := make([]api.Uuid, 0, len(value.CreatedProjectIDs))
	for _, projectID := range value.CreatedProjectIDs {
		projectIDs = append(projectIDs, projectID)
	}
	return api.ImportCommitResult{BatchId: value.BatchID, CommittedAt: value.CommittedAt, CreatedProjectIds: projectIDs, SkippedRows: value.SkippedRows}
}

func (controller *Controller) importRowResponse(ctx context.Context, value importservice.Row) (api.ImportRow, error) {
	draftJSON, err := json.Marshal(value.Draft)
	if err != nil {
		return api.ImportRow{}, err
	}
	draft := map[string]any{}
	if err := json.Unmarshal(draftJSON, &draft); err != nil {
		return api.ImportRow{}, err
	}
	response := api.ImportRow{RowNumber: value.RowNumber, ImportKey: value.ImportKey, State: api.ImportRowState(value.State), Selected: value.Selected, WarningsAcknowledged: value.WarningsAcknowledged, Draft: &draft, Issues: []api.ImportIssue{}}
	if value.DuplicateResolution != nil {
		resolution := api.ImportRowDuplicateResolution(*value.DuplicateResolution)
		response.DuplicateResolution = &resolution
	}
	for _, issue := range value.Issues {
		message := issue.Message
		response.Issues = append(response.Issues, api.ImportIssue{Field: issue.Field, Code: issue.Code, Severity: api.ImportIssueSeverity(issue.Severity), Message: &message})
	}
	if len(value.DuplicateCandidateIDs) > 0 {
		candidates := make([]api.ProjectSummary, 0, len(value.DuplicateCandidateIDs))
		for _, projectID := range value.DuplicateCandidateIDs {
			candidate, err := controller.projectSummary(ctx, projectID)
			if err != nil {
				return api.ImportRow{}, err
			}
			candidates = append(candidates, candidate)
		}
		response.DuplicateCandidates = &candidates
	}
	return response, nil
}

func (controller *Controller) projectSummary(ctx context.Context, projectID uuid.UUID) (api.ProjectSummary, error) {
	value, err := controller.Projects.Get(ctx, projectID, false)
	if err != nil {
		return api.ProjectSummary{}, err
	}
	program, err := controller.catalogReference(ctx, "program", value.ProgramVersionID)
	if err != nil {
		return api.ProjectSummary{}, err
	}
	major, err := controller.catalogReference(ctx, "major", value.MajorVersionID)
	if err != nil {
		return api.ProjectSummary{}, err
	}
	course, err := controller.catalogReference(ctx, "course", value.CourseVersionID)
	if err != nil {
		return api.ProjectSummary{}, err
	}
	participations, err := controller.projectParticipations(ctx, projectID)
	if err != nil {
		return api.ProjectSummary{}, err
	}
	taxonomy, err := controller.projectTaxonomy(ctx, projectID)
	if err != nil {
		return api.ProjectSummary{}, err
	}
	artifacts, err := controller.projectArtifacts(ctx, projectID, true)
	if err != nil {
		return api.ProjectSummary{}, err
	}
	publishedAt := value.UpdatedAt
	if value.PublishedAt != nil {
		publishedAt = *value.PublishedAt
	}
	response := api.ProjectSummary{
		Id: value.ID, ReferenceCode: value.ReferenceCode, Title: valueOrEmpty(value.Title), AcademicYear: valueOrZero(value.AcademicYear),
		Semester: api.Semester(valueOrEmpty(value.Semester)), Major: major, People: participations, Categories: filterTaxonomy(taxonomy, "category"),
		Platforms: filterTaxonomy(taxonomy, "platform"), ArtifactCount: len(artifacts), PublishedAt: publishedAt,
	}
	if program != nil {
		response.Program = *program
	}
	if course != nil {
		response.Course = *course
	}
	return response, nil
}

func readImportMultipart(writer http.ResponseWriter, request *http.Request) (multipart.File, *multipart.FileHeader, func(), bool) {
	if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "multipart/form-data") {
		problem(writer, request, http.StatusUnsupportedMediaType, "unsupported_media_type", "Unsupported media type", "A multipart form is required.")
		return nil, nil, func() {}, false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, importservice.MaximumUploadBytes+(2<<20))
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		if request.MultipartForm != nil {
			_ = request.MultipartForm.RemoveAll()
		}
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			problem(writer, request, http.StatusRequestEntityTooLarge, "import_too_large", "Import too large", "The upload exceeds 25 MiB.")
		} else {
			problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "The multipart form is invalid.")
		}
		return nil, nil, func() {}, false
	}
	cleanup := func() { _ = request.MultipartForm.RemoveAll() }
	if len(request.MultipartForm.Value) != 0 || len(request.MultipartForm.File) != 1 {
		cleanup()
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "Only one file field is allowed.")
		return nil, nil, func() {}, false
	}
	fileHeaders := request.MultipartForm.File["file"]
	if len(fileHeaders) != 1 {
		cleanup()
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "file is required exactly once.")
		return nil, nil, func() {}, false
	}
	file, err := fileHeaders[0].Open()
	if err != nil {
		cleanup()
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "The uploaded file cannot be read.")
		return nil, nil, func() {}, false
	}
	return file, fileHeaders[0], cleanup, true
}

func encodeImportCursor(offset int) *string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
	return &encoded
}

func decodeImportCursor(cursor *api.Cursor) (int, error) {
	if cursor == nil {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(*cursor))
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return 0, errors.New("invalid import cursor")
	}
	return offset, nil
}

func (controller *Controller) writeImportError(writer http.ResponseWriter, request *http.Request, batchID uuid.UUID, err error) {
	switch {
	case errors.Is(err, importservice.ErrNotFound), errors.Is(err, importservice.ErrResultUnavailable):
		problem(writer, request, http.StatusNotFound, "not_found", "Not found", "")
	case errors.Is(err, importservice.ErrRevisionConflict):
		batch, getErr := controller.Imports.Get(request.Context(), batchID)
		if getErr == nil {
			revisionConflict(writer, request, batch.Revision)
		} else {
			problem(writer, request, http.StatusConflict, "revision_conflict", "Revision conflict", "")
		}
	case errors.Is(err, importservice.ErrInvalidState), errors.Is(err, importservice.ErrExpired):
		problem(writer, request, http.StatusConflict, "import_state_conflict", "Import state conflict", err.Error())
	case errors.Is(err, importservice.ErrInvalidFile), errors.Is(err, importservice.ErrInvalidSelection):
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", err.Error())
	default:
		slog.Error("Import operation failed", "batch_id", batchID, "error", err, "request_id", requestIDValue(request.Context()))
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
	}
}

func (controller *Controller) readArtifactMultipart(writer http.ResponseWriter, request *http.Request, requiredFields ...string) (map[string]string, multipart.File, *multipart.FileHeader, func(), bool) {
	if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "multipart/form-data") {
		problem(writer, request, http.StatusUnsupportedMediaType, "unsupported_media_type", "Unsupported media type", "A multipart form is required.")
		return nil, nil, nil, func() {}, false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, controller.Config.MaxArtifactBytes+(2<<20))
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		if request.MultipartForm != nil {
			_ = request.MultipartForm.RemoveAll()
		}
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			problem(writer, request, http.StatusRequestEntityTooLarge, "artifact_too_large", "Artifact too large", "The upload exceeds the configured file limit.")
		} else {
			problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "The multipart form is invalid.")
		}
		return nil, nil, nil, func() {}, false
	}
	cleanup := func() { _ = request.MultipartForm.RemoveAll() }
	allowedFields := map[string]bool{"file": true}
	values := map[string]string{}
	for _, field := range requiredFields {
		allowedFields[field] = true
		entries := request.MultipartForm.Value[field]
		if len(entries) != 1 || strings.TrimSpace(entries[0]) == "" {
			cleanup()
			problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", field+" is required exactly once.")
			return nil, nil, nil, func() {}, false
		}
		values[field] = entries[0]
	}
	for field := range request.MultipartForm.Value {
		if !allowedFields[field] {
			cleanup()
			problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "Unexpected multipart field: "+field)
			return nil, nil, nil, func() {}, false
		}
	}
	for field := range request.MultipartForm.File {
		if field != "file" {
			cleanup()
			problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "Unexpected multipart file field: "+field)
			return nil, nil, nil, func() {}, false
		}
	}
	fileHeaders := request.MultipartForm.File["file"]
	if len(fileHeaders) != 1 {
		cleanup()
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "file is required exactly once.")
		return nil, nil, nil, func() {}, false
	}
	file, err := fileHeaders[0].Open()
	if err != nil {
		cleanup()
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", "The uploaded file cannot be read.")
		return nil, nil, nil, func() {}, false
	}
	return values, file, fileHeaders[0], cleanup, true
}

func parseRevision(value string) (int64, error) {
	revision, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || revision <= 0 {
		return 0, errors.New("expected revision must be a positive integer")
	}
	return revision, nil
}

func (controller *Controller) writeArtifactError(writer http.ResponseWriter, request *http.Request, artifactID uuid.UUID, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, artifacts.ErrNotFound):
		problem(writer, request, http.StatusNotFound, "not_found", "Not found", "")
	case errors.Is(err, artifacts.ErrContentUnavailable):
		problem(writer, request, http.StatusNotFound, "artifact_content_unavailable", "Artifact content unavailable", "The Artifact metadata exists, but its content is unavailable.")
	case errors.Is(err, artifacts.ErrRevisionConflict):
		revision, revisionErr := controller.Artifacts.CurrentRevision(request.Context(), artifactID)
		if revisionErr != nil {
			problem(writer, request, http.StatusConflict, "revision_conflict", "Revision conflict", "")
		} else {
			revisionConflict(writer, request, revision)
		}
	case errors.Is(err, artifacts.ErrContentTooLarge):
		problem(writer, request, http.StatusRequestEntityTooLarge, "artifact_too_large", "Artifact too large", err.Error())
	case errors.Is(err, artifacts.ErrProjectQuotaExceeded):
		problem(writer, request, http.StatusBadRequest, "artifact_quota_exceeded", "Artifact quota exceeded", err.Error())
	case errors.Is(err, artifacts.ErrEmptyContent), errors.Is(err, artifacts.ErrSizeMismatch), errors.Is(err, artifacts.ErrArtifactTypeMismatch), errors.Is(err, artifacts.ErrContentTypeMismatch), errors.Is(err, artifacts.ErrUnsafeContentType), errors.Is(err, artifacts.ErrInvalidState):
		problem(writer, request, http.StatusBadRequest, "validation_error", "Validation error", err.Error())
	default:
		problem(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error", "")
	}
	return true
}

func (controller *Controller) serveArtifact(writer http.ResponseWriter, request *http.Request, artifactID uuid.UUID, inline bool) {
	content, err := controller.Artifacts.OpenPublic(request.Context(), artifactID)
	if err != nil {
		if errors.Is(err, artifacts.ErrContentUnavailable) {
			slog.Error("Artifact content unavailable", "artifact_id", artifactID, "request_id", requestIDValue(request.Context()))
		}
		controller.writeArtifactError(writer, request, artifactID, err)
		return
	}
	defer content.File.Close()
	if inline && (content.Artifact.Extension != "pdf" || content.Artifact.MIMEType != "application/pdf") {
		problem(writer, request, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	if content.Size != content.Artifact.ByteCount {
		slog.Error("Artifact content size mismatch", "artifact_id", artifactID, "request_id", requestIDValue(request.Context()))
		problem(writer, request, http.StatusNotFound, "artifact_content_unavailable", "Artifact content unavailable", "The Artifact metadata exists, but its content is unavailable.")
		return
	}
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}
	writer.Header().Set("Content-Type", content.Artifact.MIMEType)
	writer.Header().Set("Content-Disposition", artifactContentDisposition(disposition, content.Artifact.OriginalFilename))
	writer.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeContent(writer, request, content.Artifact.OriginalFilename, content.Artifact.UpdatedAt, content.File)
}

func artifactContentDisposition(disposition, filename string) string {
	var fallback strings.Builder
	for _, character := range filename {
		if character < 0x20 || character > 0x7e || character == '"' || character == '\\' {
			fallback.WriteByte('_')
		} else {
			fallback.WriteRune(character)
		}
	}
	return disposition + `; filename="` + fallback.String() + `"; filename*=UTF-8''` + url.PathEscape(filename)
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		problem(writer, request, 415, "unsupported_media_type", "Unsupported media type", "")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		problem(writer, request, 400, "validation_error", "Validation error", "Invalid JSON request body.")
		return false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		problem(writer, request, 400, "validation_error", "Validation error", "Request body must contain one JSON value.")
		return false
	}
	return true
}
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
func problem(writer http.ResponseWriter, request *http.Request, status int, code, title, detail string) {
	writer.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(api.Problem{Type: "about:blank", Title: title, Status: status, Code: code, Detail: &detail, RequestId: requestIDValue(request.Context())})
}
func revisionConflict(writer http.ResponseWriter, request *http.Request, revision int64) {
	writer.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	writer.WriteHeader(409)
	_ = json.NewEncoder(writer).Encode(api.RevisionConflictProblem{Type: "about:blank", Title: "Revision conflict", Status: 409, Code: "revision_conflict", CurrentRevision: int(revision), RequestId: requestIDValue(request.Context())})
}
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		id := request.Header.Get("X-Request-ID")
		if id == "" || len(id) > 128 {
			id = uuid.NewString()
		}
		writer.Header().Set("X-Request-ID", id)
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), requestContextKey("request_id"), id)))
	})
}
func requestIDValue(ctx context.Context) string {
	value, _ := ctx.Value(requestContextKey("request_id")).(string)
	return value
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "same-origin")
		writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'self'")
		next.ServeHTTP(writer, request)
	})
}
func clientIP(request *http.Request, configuration config.Config) netip.Addr {
	remoteAddress := remoteIP(request)
	if !isTrustedProxy(remoteAddress, configuration.TrustedProxyCIDRs) {
		return remoteAddress
	}
	clientAddress := remoteAddress
	forwardedAddresses := strings.Split(request.Header.Get("X-Forwarded-For"), ",")
	for index := len(forwardedAddresses) - 1; index >= 0; index-- {
		address, err := netip.ParseAddr(strings.TrimSpace(forwardedAddresses[index]))
		if err != nil {
			return remoteAddress
		}
		clientAddress = address
		if !isTrustedProxy(address, configuration.TrustedProxyCIDRs) {
			return address
		}
	}
	return clientAddress
}
func remoteIP(request *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return netip.IPv4Unspecified()
	}
	remoteAddress, err := netip.ParseAddr(host)
	if err != nil {
		return netip.IPv4Unspecified()
	}
	return remoteAddress
}
func trustedOrigin(request *http.Request, configuration config.Config) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	scheme := "http"
	remoteAddress := remoteIP(request)
	if request.TLS != nil || (isTrustedProxy(remoteAddress, configuration.TrustedProxyCIDRs) && request.Header.Get("X-Forwarded-Proto") == "https") {
		scheme = "https"
	}
	parsedOrigin, err := url.Parse(origin)
	return err == nil && parsedOrigin.Scheme == scheme && parsedOrigin.Host == request.Host && parsedOrigin.Path == "" && parsedOrigin.RawQuery == "" && parsedOrigin.Fragment == ""
}
func isTrustedProxy(address netip.Addr, cidrs []string) bool {
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err == nil && prefix.Contains(address) {
			return true
		}
	}
	return false
}
func limit(value *int) int {
	if value == nil || *value <= 0 {
		return 20
	}
	if *value > 100 {
		return 100
	}
	return *value
}

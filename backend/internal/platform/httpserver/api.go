package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	api "ause-discovery.local/backend/generated/api"
	"ause-discovery.local/backend/internal/auth"
	"ause-discovery.local/backend/internal/people"
	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/projects"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sessionCookieName = "ause_session"
const csrfCookieName = "ause_csrf"

type Controller struct {
	api.Unimplemented
	Auth     auth.Service
	People   people.Service
	Projects projects.Service
	Config   config.Config
}
type requestContextKey string

const actorContextKey requestContextKey = "actor"

func NewAPIHandler(pool *pgxpool.Pool, configuration config.Config) http.Handler {
	controller := Controller{Auth: auth.Service{Pool: pool, SessionIdleTTL: configuration.SessionIdleTTL, SessionAbsoluteTTL: configuration.SessionAbsoluteTTL}, People: people.Service{Pool: pool}, Projects: projects.Service{Pool: pool}, Config: configuration}
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
	writeJSON(writer, http.StatusOK, api.SessionResponse{ExpiresAt: session.AbsoluteExpiresAt, User: api.SessionUser{Id: session.UserID, Username: session.Username, Permissions: []string{"project.create", "project.edit", "project.delete", "person.create", "person.edit"}}})
}
func (controller *Controller) GetSession(writer http.ResponseWriter, request *http.Request) {
	actor, ok := controller.requireActor(writer, request, false)
	if !ok {
		return
	}
	writeJSON(writer, http.StatusOK, api.SessionResponse{ExpiresAt: actor.Session.AbsoluteExpiresAt, User: api.SessionUser{Id: actor.UserID, Username: actor.Username, Permissions: []string{"project.create", "project.edit", "project.delete", "person.create", "person.edit"}}})
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
	items, err := controller.People.List(request.Context(), query, limit(params.Limit), 0)
	if err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	response := api.AdminPersonPage{Page: api.PageInfo{Limit: limit(params.Limit)}}
	for _, item := range items {
		response.Items = append(response.Items, personResponse(item))
	}
	writeJSON(writer, 200, response)
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
	writeJSON(writer, 201, adminProjectResponse(project))
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
	items, err := controller.Projects.List(request.Context(), status, limit(params.Limit), 0)
	if err != nil {
		problem(writer, request, 500, "internal_error", "Internal server error", "")
		return
	}
	response := api.AdminProjectPage{Page: api.PageInfo{Limit: limit(params.Limit)}}
	for _, item := range items {
		response.Items = append(response.Items, adminProjectResponse(item))
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
	writeJSON(writer, status, adminProjectResponse(project))
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
func adminProjectResponse(value projects.Project) api.AdminProject {
	return api.AdminProject{Id: value.ID, ReferenceCode: value.ReferenceCode, Title: value.Title, Abstract: value.Abstract, AcademicYear: value.AcademicYear, Status: api.ProjectStatus(value.Status), Revision: int(value.Revision), TitleAliases: &value.TitleAliases, Artifacts: []api.Artifact{}, Participations: []api.Participation{}, Taxonomy: []api.TaxonomyValue{}, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
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
	artifacts, err := controller.projectArtifacts(ctx, value.ID)
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

func (controller *Controller) projectArtifacts(ctx context.Context, projectID uuid.UUID) ([]api.Artifact, error) {
	rows, err := controller.Projects.Pool.Query(ctx, `SELECT id, project_id, type, display_name, original_filename, mime_type, byte_count, status, revision, created_at, updated_at, deleted_at FROM artifacts WHERE project_id = $1 AND status = 'active' ORDER BY created_at, id`, projectID)
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
		downloadURL := baseURL + value.Id.String() + "/download"
		value.DownloadUrl = &downloadURL
		if value.MimeType == "application/pdf" {
			viewURL := baseURL + value.Id.String() + "/view"
			value.ViewUrl = &viewURL
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
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return netip.IPv4Unspecified()
	}
	remoteAddress, err := netip.ParseAddr(host)
	if err != nil {
		return netip.IPv4Unspecified()
	}
	if !isTrustedProxy(remoteAddress, configuration.TrustedProxyCIDRs) {
		return remoteAddress
	}
	for _, forwardedAddress := range strings.Split(request.Header.Get("X-Forwarded-For"), ",") {
		if address, parseErr := netip.ParseAddr(strings.TrimSpace(forwardedAddress)); parseErr == nil {
			return address
		}
	}
	return remoteAddress
}
func trustedOrigin(request *http.Request, configuration config.Config) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	scheme := "http"
	remoteAddress := clientIP(request, config.Config{})
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

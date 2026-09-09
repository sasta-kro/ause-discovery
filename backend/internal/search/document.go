package search

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrProjectNotPublished = errors.New("Project is not published")

const SchemaVersion = 1

type CatalogReference struct {
	ID    uuid.UUID `json:"id"`
	Key   string    `json:"key"`
	Label string    `json:"label"`
}

type PersonSummary struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	StudentID   *string   `json:"student_id,omitempty"`
}

type Participation struct {
	Person    PersonSummary `json:"person"`
	Role      string        `json:"role"`
	SortOrder int           `json:"sort_order"`
}

type TaxonomyValue struct {
	ID          uuid.UUID         `json:"id"`
	Dimension   string            `json:"dimension"`
	Key         string            `json:"key"`
	Labels      map[string]string `json:"labels"`
	Description *string           `json:"description,omitempty"`
	SortOrder   int               `json:"sort_order"`
}

type Document struct {
	ID               uuid.UUID         `json:"id"`
	Revision         int64             `json:"revision"`
	ReferenceCode    *string           `json:"reference_code,omitempty"`
	Title            string            `json:"title"`
	TitleSort        string            `json:"title_sort"`
	TitleAliases     []string          `json:"title_aliases"`
	Abstract         string            `json:"abstract"`
	AcademicYear     int               `json:"academic_year"`
	Semester         string            `json:"semester"`
	SemesterOrder    int               `json:"semester_order"`
	Program          CatalogReference  `json:"program"`
	Major            *CatalogReference `json:"major,omitempty"`
	Course           CatalogReference  `json:"course"`
	ProgramKey       string            `json:"program_key"`
	MajorKey         string            `json:"major_key,omitempty"`
	CourseKey        string            `json:"course_key"`
	People           []Participation   `json:"people"`
	PersonIDs        []string          `json:"person_ids"`
	PersonNames      []string          `json:"person_names"`
	StudentIDs       []string          `json:"student_ids"`
	StudentNames     []string          `json:"student_names"`
	AdvisorPersonIDs []string          `json:"advisor_person_ids"`
	AdvisorNames     []string          `json:"advisor_names"`
	CoAdvisorNames   []string          `json:"co_advisor_names"`
	CommitteeNames   []string          `json:"committee_names"`
	Taxonomy         []TaxonomyValue   `json:"taxonomy"`
	TaxonomyKeys     []string          `json:"taxonomy_keys"`
	TaxonomyLabels   []string          `json:"taxonomy_labels"`
	Categories       []TaxonomyValue   `json:"categories"`
	Platforms        []TaxonomyValue   `json:"platforms"`
	CategoryKeys     []string          `json:"category_keys"`
	PlatformKeys     []string          `json:"platform_keys"`
	DomainKeys       []string          `json:"domain_keys"`
	TopicKeys        []string          `json:"topic_keys"`
	TechnologyKeys   []string          `json:"technology_keys"`
	ArtifactTypes    []string          `json:"artifact_types"`
	ArtifactCount    int               `json:"artifact_count"`
	HasArtifacts     bool              `json:"has_artifacts"`
	HasReport        bool              `json:"has_report"`
	HasSlides        bool              `json:"has_slides"`
	HasSourceCode    bool              `json:"has_source_code"`
	HasDataset       bool              `json:"has_dataset"`
	PublishedAt      time.Time         `json:"published_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

func BuildProjectDocument(ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID) (Document, error) {
	var document Document
	var majorID *uuid.UUID
	var majorKey *string
	var majorLabel *string
	err := pool.QueryRow(ctx, `
		SELECT projects.id, projects.revision, projects.reference_code, projects.title, projects.abstract, projects.academic_year, projects.semester,
		       program_versions.id, programs.key, program_versions.label,
		       major_versions.id, majors.key, major_versions.label,
		       course_versions.id, courses.key, course_versions.label,
		       projects.published_at, projects.updated_at
		FROM projects
		JOIN program_versions ON program_versions.id=projects.program_version_id
		JOIN programs ON programs.id=program_versions.program_id
		LEFT JOIN major_versions ON major_versions.id=projects.major_version_id
		LEFT JOIN majors ON majors.id=major_versions.major_id
		JOIN course_versions ON course_versions.id=projects.course_version_id
		JOIN courses ON courses.id=course_versions.course_id
		WHERE projects.id=$1 AND projects.status='published'`, projectID).Scan(
		&document.ID, &document.Revision, &document.ReferenceCode, &document.Title, &document.Abstract, &document.AcademicYear, &document.Semester,
		&document.Program.ID, &document.Program.Key, &document.Program.Label,
		&majorID, &majorKey, &majorLabel,
		&document.Course.ID, &document.Course.Key, &document.Course.Label,
		&document.PublishedAt, &document.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrProjectNotPublished
	}
	if err != nil {
		return Document{}, err
	}
	document.TitleSort = strings.ToLower(strings.TrimSpace(document.Title))
	document.ProgramKey = document.Program.Key
	document.CourseKey = document.Course.Key
	document.SemesterOrder = semesterOrder(document.Semester)
	if majorID != nil && majorKey != nil && majorLabel != nil {
		document.Major = &CatalogReference{ID: *majorID, Key: *majorKey, Label: *majorLabel}
		document.MajorKey = *majorKey
	}
	if err := loadDocumentAliases(ctx, pool, &document); err != nil {
		return Document{}, err
	}
	if err := loadDocumentPeople(ctx, pool, &document); err != nil {
		return Document{}, err
	}
	if err := loadDocumentTaxonomy(ctx, pool, &document); err != nil {
		return Document{}, err
	}
	if err := loadDocumentArtifacts(ctx, pool, &document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func loadDocumentAliases(ctx context.Context, pool *pgxpool.Pool, document *Document) error {
	rows, err := pool.Query(ctx, "SELECT value FROM project_title_aliases WHERE project_id=$1 ORDER BY position", document.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	document.TitleAliases = []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return err
		}
		document.TitleAliases = append(document.TitleAliases, value)
	}
	return rows.Err()
}

func loadDocumentPeople(ctx context.Context, pool *pgxpool.Pool, document *Document) error {
	rows, err := pool.Query(ctx, `SELECT people.id, people.display_name, people.student_id, project_participations.role, project_participations.position FROM project_participations JOIN people ON people.id=project_participations.person_id WHERE project_participations.project_id=$1 ORDER BY project_participations.role, project_participations.position`, document.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	document.People = []Participation{}
	document.PersonIDs = []string{}
	document.PersonNames = []string{}
	document.StudentIDs = []string{}
	document.StudentNames = []string{}
	document.AdvisorPersonIDs = []string{}
	document.AdvisorNames = []string{}
	document.CoAdvisorNames = []string{}
	document.CommitteeNames = []string{}
	for rows.Next() {
		var participation Participation
		if err := rows.Scan(&participation.Person.ID, &participation.Person.DisplayName, &participation.Person.StudentID, &participation.Role, &participation.SortOrder); err != nil {
			return err
		}
		document.People = append(document.People, participation)
		document.PersonIDs = append(document.PersonIDs, participation.Person.ID.String())
		document.PersonNames = append(document.PersonNames, participation.Person.DisplayName)
		switch participation.Role {
		case "student":
			document.StudentNames = append(document.StudentNames, participation.Person.DisplayName)
			if participation.Person.StudentID != nil {
				document.StudentIDs = append(document.StudentIDs, *participation.Person.StudentID)
			}
		case "advisor":
			document.AdvisorPersonIDs = append(document.AdvisorPersonIDs, participation.Person.ID.String())
			document.AdvisorNames = append(document.AdvisorNames, participation.Person.DisplayName)
		case "co_advisor":
			document.CoAdvisorNames = append(document.CoAdvisorNames, participation.Person.DisplayName)
		case "committee_member":
			document.CommitteeNames = append(document.CommitteeNames, participation.Person.DisplayName)
		}
	}
	return rows.Err()
}

func loadDocumentTaxonomy(ctx context.Context, pool *pgxpool.Pool, document *Document) error {
	rows, err := pool.Query(ctx, `SELECT taxonomy_values.id, taxonomy_values.dimension, taxonomy_values.key, taxonomy_values.labels, taxonomy_values.description, taxonomy_values.sort_order FROM project_taxonomy_values JOIN taxonomy_values ON taxonomy_values.id=project_taxonomy_values.taxonomy_value_id WHERE project_taxonomy_values.project_id=$1 ORDER BY project_taxonomy_values.dimension, project_taxonomy_values.position`, document.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	document.Taxonomy = []TaxonomyValue{}
	document.TaxonomyKeys = []string{}
	document.TaxonomyLabels = []string{}
	document.Categories = []TaxonomyValue{}
	document.Platforms = []TaxonomyValue{}
	document.CategoryKeys = []string{}
	document.PlatformKeys = []string{}
	document.DomainKeys = []string{}
	document.TopicKeys = []string{}
	document.TechnologyKeys = []string{}
	for rows.Next() {
		var value TaxonomyValue
		var labels []byte
		if err := rows.Scan(&value.ID, &value.Dimension, &value.Key, &labels, &value.Description, &value.SortOrder); err != nil {
			return err
		}
		if err := json.Unmarshal(labels, &value.Labels); err != nil {
			return err
		}
		appendDocumentTaxonomy(document, value)
	}
	return rows.Err()
}

func appendDocumentTaxonomy(document *Document, value TaxonomyValue) {
	document.Taxonomy = append(document.Taxonomy, value)
	if value.Dimension != "topic" {
		document.TaxonomyKeys = append(document.TaxonomyKeys, value.Key)
		for _, label := range value.Labels {
			document.TaxonomyLabels = append(document.TaxonomyLabels, label)
		}
	}
	switch value.Dimension {
	case "category":
		document.Categories = append(document.Categories, value)
		document.CategoryKeys = append(document.CategoryKeys, value.Key)
	case "platform":
		document.Platforms = append(document.Platforms, value)
		document.PlatformKeys = append(document.PlatformKeys, value.Key)
	case "domain":
		document.DomainKeys = append(document.DomainKeys, value.Key)
	case "topic":
		document.TopicKeys = append(document.TopicKeys, value.Key)
	case "technology":
		document.TechnologyKeys = append(document.TechnologyKeys, value.Key)
	}
}

func loadDocumentArtifacts(ctx context.Context, pool *pgxpool.Pool, document *Document) error {
	rows, err := pool.Query(ctx, "SELECT type FROM artifacts WHERE project_id=$1 AND status='active' ORDER BY type, id", document.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	document.ArtifactTypes = []string{}
	seenTypes := map[string]bool{}
	for rows.Next() {
		var artifactType string
		if err := rows.Scan(&artifactType); err != nil {
			return err
		}
		document.ArtifactCount++
		if seenTypes[artifactType] {
			continue
		}
		seenTypes[artifactType] = true
		document.ArtifactTypes = append(document.ArtifactTypes, artifactType)
		switch artifactType {
		case "report":
			document.HasReport = true
		case "slides":
			document.HasSlides = true
		case "source_code":
			document.HasSourceCode = true
		case "dataset":
			document.HasDataset = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	document.HasArtifacts = document.ArtifactCount > 0
	return nil
}

func semesterOrder(semester string) int {
	switch semester {
	case "first":
		return 1
	case "second":
		return 2
	case "summer":
		return 3
	default:
		return 0
	}
}

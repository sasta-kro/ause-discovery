package search

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSearchUnavailable = errors.New("search is unavailable")
	ErrInvalidFilter     = errors.New("search filter is invalid")
	ErrInvalidCursor     = errors.New("search cursor is invalid")
)

type Query struct {
	Text           string
	AcademicYear   *int
	Semester       *string
	ProgramKeys    []string
	MajorKeys      []string
	CourseKeys     []string
	PersonIDs      []uuid.UUID
	StudentID      *string
	AdvisorIDs     []uuid.UUID
	CategoryKeys   []string
	PlatformKeys   []string
	DomainKeys     []string
	TopicKeys      []string
	TechnologyKeys []string
	ArtifactTypes  []string
	HasArtifacts   *bool
	HasReport      *bool
	HasSlides      *bool
	HasSourceCode  *bool
	HasDataset     *bool
	Sort           string
	Cursor         string
	Limit          int
}

type Highlight struct {
	Field string
	Value string
}

type ResultItem struct {
	Document
	Highlights []Highlight
}

type Facet struct {
	Key   string
	Count int
	Label *string
}

type Facets struct {
	Programs, Majors, Courses, AcademicYears, Semesters, People, Advisors, Categories, Platforms, Domains, Topics, Technologies []Facet
}

type Result struct {
	Items      []ResultItem
	Total      int
	Facets     Facets
	Limit      int
	NextCursor *string
}

type Service struct {
	Pool     *pgxpool.Pool
	Index    Index
	IndexUID string
}

var sevenDigitIdentifier = regexp.MustCompile(`^[0-9]{7}$`)

var searchFacetAttributes = []string{"program_key", "major_key", "course_key", "academic_year", "semester", "person_ids", "advisor_person_ids", "category_keys", "platform_keys", "domain_keys", "topic_keys", "technology_keys"}

func (service Service) Search(ctx context.Context, query Query) (Result, error) {
	if err := service.ValidateQuery(ctx, query); err != nil {
		return Result{}, err
	}
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	offset := 0
	if query.Cursor != "" {
		decodedOffset, err := decodeCursor(query, query.Cursor)
		if err != nil {
			return Result{}, err
		}
		offset = decodedOffset
	}
	resolvedOrder := ResolveOrder(query.Sort, query.Text)
	indexQuery := IndexQuery{Query: strings.TrimSpace(query.Text), Filters: buildFilters(query), Sort: resolvedOrder.Sort, Offset: offset, Limit: query.Limit, Facets: searchFacetAttributes}

	var indexResult IndexResult
	var err error
	if sevenDigitIdentifier.MatchString(indexQuery.Query) {
		exactQuery := indexQuery
		exactQuery.Filters = append(append([]string{}, indexQuery.Filters...), `(student_ids = `+quoteFilter(indexQuery.Query)+` OR reference_code = `+quoteFilter(indexQuery.Query)+`)`)
		indexResult, err = service.Index.Search(ctx, service.indexUID(), exactQuery)
		if err == nil && indexResult.Total == 0 {
			indexResult, err = service.Index.Search(ctx, service.indexUID(), indexQuery)
		}
	} else {
		indexResult, err = service.Index.Search(ctx, service.indexUID(), indexQuery)
	}
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrSearchUnavailable, err)
	}

	facets := mapFacets(indexResult.FacetDistribution)
	if err := labelPersonFacets(ctx, service.Pool, &facets); err != nil {
		return Result{}, fmt.Errorf("%w: label Person facets: %v", ErrSearchUnavailable, err)
	}
	result := Result{Total: indexResult.Total, Limit: query.Limit, Facets: facets, Items: []ResultItem{}}
	for _, document := range indexResult.Hits {
		result.Items = append(result.Items, ResultItem{Document: document, Highlights: documentHighlights(document, indexQuery.Query)})
	}
	if offset+len(indexResult.Hits) < indexResult.Total {
		nextCursor, err := encodeCursor(query, offset+len(indexResult.Hits))
		if err != nil {
			return Result{}, err
		}
		result.NextCursor = &nextCursor
	}
	return result, nil
}

func (service Service) ValidateQuery(ctx context.Context, query Query) error {
	if len(query.Text) > 500 {
		return fmt.Errorf("%w: query exceeds 500 characters", ErrInvalidFilter)
	}
	if query.AcademicYear != nil && (*query.AcademicYear < 1900 || *query.AcademicYear > 9999) {
		return fmt.Errorf("%w: academic year", ErrInvalidFilter)
	}
	if query.Semester != nil && *query.Semester != "first" && *query.Semester != "second" && *query.Semester != "summer" {
		return fmt.Errorf("%w: semester", ErrInvalidFilter)
	}
	if query.StudentID != nil && !sevenDigitIdentifier.MatchString(*query.StudentID) {
		return fmt.Errorf("%w: Student ID", ErrInvalidFilter)
	}
	if query.Sort != "" && query.Sort != "relevance" && query.Sort != "newest" && query.Sort != "oldest" && query.Sort != "title" {
		return fmt.Errorf("%w: sort", ErrInvalidFilter)
	}
	for _, artifactType := range query.ArtifactTypes {
		if !validArtifactType(artifactType) {
			return fmt.Errorf("%w: Artifact type", ErrInvalidFilter)
		}
	}
	for _, values := range [][]string{query.ProgramKeys, query.MajorKeys, query.CourseKeys, query.CategoryKeys, query.PlatformKeys, query.DomainKeys, query.TopicKeys, query.TechnologyKeys, query.ArtifactTypes} {
		if len(values) > 20 {
			return fmt.Errorf("%w: too many facet values", ErrInvalidFilter)
		}
	}
	if len(query.PersonIDs) > 20 || len(query.AdvisorIDs) > 20 {
		return fmt.Errorf("%w: too many Person values", ErrInvalidFilter)
	}
	for _, personID := range append(append([]uuid.UUID{}, query.PersonIDs...), query.AdvisorIDs...) {
		var exists bool
		if err := service.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM people WHERE id=$1)", personID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("%w: unknown Person", ErrInvalidFilter)
		}
	}
	checks := []struct {
		table     string
		dimension string
		values    []string
	}{
		{table: "programs", values: query.ProgramKeys},
		{table: "majors", values: query.MajorKeys},
		{table: "courses", values: query.CourseKeys},
		{table: "taxonomy_values", dimension: "category", values: query.CategoryKeys},
		{table: "taxonomy_values", dimension: "platform", values: query.PlatformKeys},
		{table: "taxonomy_values", dimension: "domain", values: query.DomainKeys},
		{table: "taxonomy_values", dimension: "topic", values: query.TopicKeys},
		{table: "taxonomy_values", dimension: "technology", values: query.TechnologyKeys},
	}
	for _, check := range checks {
		for _, value := range check.values {
			var exists bool
			statement := "SELECT EXISTS(SELECT 1 FROM " + check.table + " WHERE key=$1)"
			arguments := []any{value}
			if check.dimension != "" {
				statement = "SELECT EXISTS(SELECT 1 FROM taxonomy_values WHERE key=$1 AND dimension=$2)"
				arguments = append(arguments, check.dimension)
			}
			if err := service.Pool.QueryRow(ctx, statement, arguments...).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("%w: unknown key %s", ErrInvalidFilter, value)
			}
		}
	}
	return nil
}

func (service Service) indexUID() string {
	if strings.TrimSpace(service.IndexUID) == "" {
		return "projects"
	}
	return service.IndexUID
}

func buildFilters(query Query) []string {
	filters := []string{}
	if query.AcademicYear != nil {
		filters = append(filters, "academic_year = "+strconv.Itoa(*query.AcademicYear))
	}
	if query.Semester != nil {
		filters = append(filters, "semester = "+quoteFilter(*query.Semester))
	}
	filters = appendStringFilter(filters, "program_key", query.ProgramKeys)
	filters = appendStringFilter(filters, "major_key", query.MajorKeys)
	filters = appendStringFilter(filters, "course_key", query.CourseKeys)
	filters = appendUUIDFilter(filters, "person_ids", query.PersonIDs)
	if query.StudentID != nil {
		filters = append(filters, "student_ids = "+quoteFilter(*query.StudentID))
	}
	filters = appendUUIDFilter(filters, "advisor_person_ids", query.AdvisorIDs)
	filters = appendStringFilter(filters, "category_keys", query.CategoryKeys)
	filters = appendStringFilter(filters, "platform_keys", query.PlatformKeys)
	filters = appendStringFilter(filters, "domain_keys", query.DomainKeys)
	filters = appendStringFilter(filters, "topic_keys", query.TopicKeys)
	filters = appendStringFilter(filters, "technology_keys", query.TechnologyKeys)
	filters = appendStringFilter(filters, "artifact_types", query.ArtifactTypes)
	filters = appendBooleanFilter(filters, "has_artifacts", query.HasArtifacts)
	filters = appendBooleanFilter(filters, "has_report", query.HasReport)
	filters = appendBooleanFilter(filters, "has_slides", query.HasSlides)
	filters = appendBooleanFilter(filters, "has_source_code", query.HasSourceCode)
	filters = appendBooleanFilter(filters, "has_dataset", query.HasDataset)
	return filters
}

func appendStringFilter(filters []string, attribute string, values []string) []string {
	if len(values) == 0 {
		return filters
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, quoteFilter(value))
	}
	return append(filters, attribute+" IN ["+strings.Join(quoted, ",")+"]")
}

func appendUUIDFilter(filters []string, attribute string, values []uuid.UUID) []string {
	if len(values) == 0 {
		return filters
	}
	converted := make([]string, 0, len(values))
	for _, value := range values {
		converted = append(converted, value.String())
	}
	return appendStringFilter(filters, attribute, converted)
}

func appendBooleanFilter(filters []string, attribute string, value *bool) []string {
	if value == nil {
		return filters
	}
	return append(filters, attribute+" = "+strconv.FormatBool(*value))
}

func quoteFilter(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// Resolved ordering modes. Academic chronology is academic_year first and
// the internal semester order second (Summer, then Second, then First
// semester for newest). Relevance mode never sends query-time sort so every
// lexical ranking rule stays ahead of the academic tie-breakers configured
// as custom ranking rules.
const (
	OrderAcademicNewest = "academic_newest"
	OrderRelevance      = "relevance"
	OrderAcademicOldest = "academic_oldest"
	OrderTitle          = "title"
)

var (
	academicNewestSort = []string{"academic_year:desc", "semester_order:desc", "title_sort:asc", "id:asc"}
	academicOldestSort = []string{"academic_year:asc", "semester_order:asc", "title_sort:asc", "id:asc"}
	titleSort          = []string{"title_sort:asc", "academic_year:desc", "semester_order:desc", "id:asc"}
)

// ResolvedOrder names one effective ordering and carries its query-time sort
// list, empty when the ranking rules alone determine the order. The name is
// the canonical ordering identity used for cursor binding.
type ResolvedOrder struct {
	Name string
	Sort []string
}

// ResolveOrder centralizes ordering decisions for one search. Text is
// normalized first: whitespace-only text is empty. Explicit Newest, Oldest,
// and Title selections are authoritative regardless of text. An omitted sort
// and an explicit relevance sort resolve identically, and any empty-text
// relevance request resolves to academic newest because no textual ranking
// exists to apply.
func ResolveOrder(sortToken, text string) ResolvedOrder {
	sortToken = strings.TrimSpace(sortToken)
	switch sortToken {
	case "oldest":
		return ResolvedOrder{Name: OrderAcademicOldest, Sort: academicOldestSort}
	case "title":
		return ResolvedOrder{Name: OrderTitle, Sort: titleSort}
	case "newest":
		return ResolvedOrder{Name: OrderAcademicNewest, Sort: academicNewestSort}
	}
	// Omitted or explicit relevance: with no text there is no lexical
	// ranking to apply, so the request behaves as academic newest.
	if strings.TrimSpace(text) == "" {
		return ResolvedOrder{Name: OrderAcademicNewest, Sort: academicNewestSort}
	}
	return ResolvedOrder{Name: OrderRelevance, Sort: nil}
}

type cursorPayload struct {
	Offset    int    `json:"offset"`
	QueryHash string `json:"query_hash"`
}

func encodeCursor(query Query, offset int) (string, error) {
	hash, err := queryHash(query)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(cursorPayload{Offset: offset, QueryHash: hash})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeCursor(query Query, cursor string) (int, error) {
	encoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, ErrInvalidCursor
	}
	var payload cursorPayload
	if err := json.Unmarshal(encoded, &payload); err != nil || payload.Offset < 0 {
		return 0, ErrInvalidCursor
	}
	hash, err := queryHash(query)
	if err != nil || payload.QueryHash != hash {
		return 0, ErrInvalidCursor
	}
	return payload.Offset, nil
}

func queryHash(query Query) (string, error) {
	query.Cursor = ""
	// The hash binds the resolved ordering rather than the raw sort token,
	// so semantically equivalent omitted and explicit relevance states hash
	// identically, and any ordering or schema change invalidates older
	// cursors instead of paging them against new ranking behavior.
	query.Sort = ResolveOrder(query.Sort, query.Text).Name
	payload := struct {
		SchemaVersion int   `json:"schema_version"`
		Query         Query `json:"query"`
	}{SchemaVersion: int(SchemaVersion), Query: query}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:8]), nil
}

func mapFacets(distribution map[string]map[string]int) Facets {
	return Facets{
		Programs: facetValues(distribution["program_key"]), Majors: facetValues(distribution["major_key"]), Courses: facetValues(distribution["course_key"]),
		AcademicYears: facetValues(distribution["academic_year"]), Semesters: facetValues(distribution["semester"]), People: facetValues(distribution["person_ids"]), Advisors: facetValues(distribution["advisor_person_ids"]), Categories: facetValues(distribution["category_keys"]),
		Platforms: facetValues(distribution["platform_keys"]), Domains: facetValues(distribution["domain_keys"]), Topics: facetValues(distribution["topic_keys"]), Technologies: facetValues(distribution["technology_keys"]),
	}
}

func labelPersonFacets(ctx context.Context, pool *pgxpool.Pool, facets *Facets) error {
	personIDs := make([]uuid.UUID, 0, len(facets.People)+len(facets.Advisors))
	seen := map[uuid.UUID]struct{}{}
	for _, values := range [][]Facet{facets.People, facets.Advisors} {
		for _, value := range values {
			personID, err := uuid.Parse(value.Key)
			if err != nil {
				continue
			}
			if _, exists := seen[personID]; exists {
				continue
			}
			seen[personID] = struct{}{}
			personIDs = append(personIDs, personID)
		}
	}
	if len(personIDs) == 0 {
		return nil
	}
	if pool == nil {
		return errors.New("database pool is unavailable")
	}
	rows, err := pool.Query(ctx, "SELECT id, display_name FROM people WHERE id = ANY($1::uuid[])", personIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	labels := map[string]string{}
	for rows.Next() {
		var personID uuid.UUID
		var displayName string
		if err := rows.Scan(&personID, &displayName); err != nil {
			return err
		}
		labels[personID.String()] = displayName
	}
	if err := rows.Err(); err != nil {
		return err
	}
	facets.People = applyFacetLabels(facets.People, labels)
	facets.Advisors = applyFacetLabels(facets.Advisors, labels)
	return nil
}

func applyFacetLabels(values []Facet, labels map[string]string) []Facet {
	for index := range values {
		if label, found := labels[values[index].Key]; found {
			values[index].Label = &label
		}
	}
	sort.SliceStable(values, func(left, right int) bool {
		leftLabel := values[left].Key
		rightLabel := values[right].Key
		if values[left].Label != nil {
			leftLabel = *values[left].Label
		}
		if values[right].Label != nil {
			rightLabel = *values[right].Label
		}
		if leftLabel == rightLabel {
			return values[left].Key < values[right].Key
		}
		return strings.ToLower(leftLabel) < strings.ToLower(rightLabel)
	})
	return values
}

func facetValues(values map[string]int) []Facet {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]Facet, 0, len(keys))
	for _, key := range keys {
		result = append(result, Facet{Key: key, Count: values[key]})
	}
	return result
}

func documentHighlights(document Document, query string) []Highlight {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []Highlight{}
	}
	highlights := []Highlight{}
	appendHighlight := func(field, value string) {
		if value != "" && strings.Contains(strings.ToLower(value), query) && len(highlights) < 20 {
			highlights = append(highlights, Highlight{Field: field, Value: boundedExcerpt(value, query, 400)})
		}
	}
	appendHighlight("reference_code", stringValue(document.ReferenceCode))
	appendHighlight("title", document.Title)
	appendHighlight("abstract", document.Abstract)
	for _, name := range document.PersonNames {
		appendHighlight("person_name", name)
	}
	for _, label := range document.TaxonomyLabels {
		appendHighlight("taxonomy", label)
	}
	return highlights
}

func boundedExcerpt(value, lowerQuery string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	index := strings.Index(strings.ToLower(value), lowerQuery)
	start := index - maximum/3
	if start < 0 {
		start = 0
	}
	end := start + maximum
	if end > len(value) {
		end = len(value)
		start = end - maximum
	}
	return value[start:end]
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func validArtifactType(value string) bool {
	switch value {
	case "report", "slides", "source_code", "proposal", "poster", "dataset", "demo_video", "other":
		return true
	default:
		return false
	}
}

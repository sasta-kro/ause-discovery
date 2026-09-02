package catalog

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

var stableKeyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

var allowedTaxonomyDimensions = map[string]struct{}{
	"category":   {},
	"platform":   {},
	"domain":     {},
	"topic":      {},
	"technology": {},
}

type Source struct {
	Academic AcademicCatalog
	Taxonomy TaxonomyCatalog
}

type AcademicCatalog struct {
	Programs []Program `yaml:"programs"`
	Majors   []Major   `yaml:"majors"`
	Courses  []Course  `yaml:"courses"`
}

type Program struct {
	ID       string           `yaml:"id"`
	Key      string           `yaml:"key"`
	Versions []ProgramVersion `yaml:"versions"`
}

type ProgramVersion struct {
	ID            string `yaml:"id"`
	Label         string `yaml:"label"`
	ValidFromYear int    `yaml:"valid_from_year"`
	ValidToYear   *int   `yaml:"valid_to_year"`
	MajorRequired bool   `yaml:"major_required"`
}

type Major struct {
	ID       string         `yaml:"id"`
	Key      string         `yaml:"key"`
	Versions []MajorVersion `yaml:"versions"`
}

type MajorVersion struct {
	ID            string `yaml:"id"`
	Label         string `yaml:"label"`
	ValidFromYear int    `yaml:"valid_from_year"`
	ValidToYear   *int   `yaml:"valid_to_year"`
}

type Course struct {
	ID       string          `yaml:"id"`
	Key      string          `yaml:"key"`
	Versions []CourseVersion `yaml:"versions"`
}

type CourseVersion struct {
	ID                string   `yaml:"id"`
	Label             string   `yaml:"label"`
	Code              string   `yaml:"code"`
	ValidFromYear     int      `yaml:"valid_from_year"`
	ValidToYear       *int     `yaml:"valid_to_year"`
	ProgramVersionIDs []string `yaml:"program_version_ids"`
}

type TaxonomyCatalog struct {
	Values []TaxonomyValue `yaml:"values"`
}

type TaxonomyValue struct {
	ID          string            `yaml:"id"`
	Dimension   string            `yaml:"dimension"`
	Key         string            `yaml:"key"`
	Labels      map[string]string `yaml:"labels"`
	Description *string           `yaml:"description"`
	SortOrder   int               `yaml:"sort_order"`
	RetiredAt   *time.Time        `yaml:"retired_at"`
}

func Load(academicPath, taxonomyPath string) (Source, error) {
	academicContents, err := os.ReadFile(academicPath)
	if err != nil {
		return Source{}, fmt.Errorf("read academic catalog %q: %w", academicPath, err)
	}
	taxonomyContents, err := os.ReadFile(taxonomyPath)
	if err != nil {
		return Source{}, fmt.Errorf("read taxonomy catalog %q: %w", taxonomyPath, err)
	}

	var academic AcademicCatalog
	if err := decodeStrictYAML(academicContents, &academic); err != nil {
		return Source{}, fmt.Errorf("decode academic catalog %q: %w", academicPath, err)
	}
	var taxonomy TaxonomyCatalog
	if err := decodeStrictYAML(taxonomyContents, &taxonomy); err != nil {
		return Source{}, fmt.Errorf("decode taxonomy catalog %q: %w", taxonomyPath, err)
	}

	source := Source{Academic: academic, Taxonomy: taxonomy}
	if err := source.Validate(); err != nil {
		return Source{}, err
	}
	return source, nil
}

func (source Source) Validate() error {
	seenIDs := map[string]string{}
	programVersionIDs := map[string]struct{}{}

	if err := validatePrograms(source.Academic.Programs, seenIDs, programVersionIDs); err != nil {
		return err
	}
	if err := validateMajors(source.Academic.Majors, seenIDs); err != nil {
		return err
	}
	if err := validateCourses(source.Academic.Courses, seenIDs, programVersionIDs); err != nil {
		return err
	}
	if err := validateTaxonomy(source.Taxonomy.Values, seenIDs); err != nil {
		return err
	}
	return nil
}

func decodeStrictYAML(contents []byte, destination any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	return nil
}

func validatePrograms(programs []Program, seenIDs map[string]string, programVersionIDs map[string]struct{}) error {
	seenKeys := map[string]struct{}{}
	for index, program := range programs {
		location := fmt.Sprintf("programs[%d]", index)
		if err := validateIdentity(location, program.ID, program.Key, seenIDs, seenKeys); err != nil {
			return err
		}
		versions := make([]versionPeriod, 0, len(program.Versions))
		for versionIndex, version := range program.Versions {
			versionLocation := fmt.Sprintf("%s.versions[%d]", location, versionIndex)
			if err := validateVersion(versionLocation, version.ID, version.Label, version.ValidFromYear, version.ValidToYear, seenIDs); err != nil {
				return err
			}
			programVersionIDs[version.ID] = struct{}{}
			versions = append(versions, versionPeriod{location: versionLocation, from: version.ValidFromYear, to: version.ValidToYear})
		}
		if err := validateNoOverlap(versions); err != nil {
			return err
		}
	}
	return nil
}

func validateMajors(majors []Major, seenIDs map[string]string) error {
	seenKeys := map[string]struct{}{}
	for index, major := range majors {
		location := fmt.Sprintf("majors[%d]", index)
		if err := validateIdentity(location, major.ID, major.Key, seenIDs, seenKeys); err != nil {
			return err
		}
		versions := make([]versionPeriod, 0, len(major.Versions))
		for versionIndex, version := range major.Versions {
			versionLocation := fmt.Sprintf("%s.versions[%d]", location, versionIndex)
			if err := validateVersion(versionLocation, version.ID, version.Label, version.ValidFromYear, version.ValidToYear, seenIDs); err != nil {
				return err
			}
			versions = append(versions, versionPeriod{location: versionLocation, from: version.ValidFromYear, to: version.ValidToYear})
		}
		if err := validateNoOverlap(versions); err != nil {
			return err
		}
	}
	return nil
}

func validateCourses(courses []Course, seenIDs map[string]string, programVersionIDs map[string]struct{}) error {
	seenKeys := map[string]struct{}{}
	for index, course := range courses {
		location := fmt.Sprintf("courses[%d]", index)
		if err := validateIdentity(location, course.ID, course.Key, seenIDs, seenKeys); err != nil {
			return err
		}
		versions := make([]versionPeriod, 0, len(course.Versions))
		for versionIndex, version := range course.Versions {
			versionLocation := fmt.Sprintf("%s.versions[%d]", location, versionIndex)
			if err := validateVersion(versionLocation, version.ID, version.Label, version.ValidFromYear, version.ValidToYear, seenIDs); err != nil {
				return err
			}
			if strings.TrimSpace(version.Code) == "" {
				return fmt.Errorf("%s.code is required", versionLocation)
			}
			seenProgramVersions := map[string]struct{}{}
			for referenceIndex, programVersionID := range version.ProgramVersionIDs {
				if _, exists := programVersionIDs[programVersionID]; !exists {
					return fmt.Errorf("%s.program_version_ids[%d] references unknown program version %q", versionLocation, referenceIndex, programVersionID)
				}
				if _, exists := seenProgramVersions[programVersionID]; exists {
					return fmt.Errorf("%s.program_version_ids contains duplicate %q", versionLocation, programVersionID)
				}
				seenProgramVersions[programVersionID] = struct{}{}
			}
			versions = append(versions, versionPeriod{location: versionLocation, from: version.ValidFromYear, to: version.ValidToYear})
		}
		if err := validateNoOverlap(versions); err != nil {
			return err
		}
	}
	return nil
}

func validateTaxonomy(values []TaxonomyValue, seenIDs map[string]string) error {
	seenDimensionKeys := map[string]struct{}{}
	for index, value := range values {
		location := fmt.Sprintf("values[%d]", index)
		if err := validateUUID(location+".id", value.ID, seenIDs); err != nil {
			return err
		}
		if _, exists := allowedTaxonomyDimensions[value.Dimension]; !exists {
			return fmt.Errorf("%s.dimension %q is invalid", location, value.Dimension)
		}
		if !stableKeyPattern.MatchString(value.Key) {
			return fmt.Errorf("%s.key %q must use lower-case snake-case", location, value.Key)
		}
		dimensionKey := value.Dimension + ":" + value.Key
		if _, exists := seenDimensionKeys[dimensionKey]; exists {
			return fmt.Errorf("%s.key %q is duplicated within dimension %q", location, value.Key, value.Dimension)
		}
		seenDimensionKeys[dimensionKey] = struct{}{}
		if value.Labels == nil || strings.TrimSpace(value.Labels["en"]) == "" {
			return fmt.Errorf("%s.labels.en is required", location)
		}
	}
	return nil
}

func validateIdentity(location, id, key string, seenIDs map[string]string, seenKeys map[string]struct{}) error {
	if err := validateUUID(location+".id", id, seenIDs); err != nil {
		return err
	}
	if !stableKeyPattern.MatchString(key) {
		return fmt.Errorf("%s.key %q must use lower-case snake-case", location, key)
	}
	if _, exists := seenKeys[key]; exists {
		return fmt.Errorf("%s.key %q is duplicated", location, key)
	}
	seenKeys[key] = struct{}{}
	return nil
}

func validateVersion(location, id, label string, validFromYear int, validToYear *int, seenIDs map[string]string) error {
	if err := validateUUID(location+".id", id, seenIDs); err != nil {
		return err
	}
	if strings.TrimSpace(label) == "" {
		return fmt.Errorf("%s.label is required", location)
	}
	if validFromYear < 1 {
		return fmt.Errorf("%s.valid_from_year must be positive", location)
	}
	if validToYear != nil && *validToYear < validFromYear {
		return fmt.Errorf("%s.valid_to_year must not precede valid_from_year", location)
	}
	return nil
}

func validateUUID(location, id string, seenIDs map[string]string) error {
	parsedID, err := uuid.Parse(id)
	if err != nil || parsedID.String() != id {
		return fmt.Errorf("%s %q is not a canonical UUID", location, id)
	}
	if previousLocation, exists := seenIDs[id]; exists {
		return fmt.Errorf("%s duplicates UUID used by %s", location, previousLocation)
	}
	seenIDs[id] = location
	return nil
}

type versionPeriod struct {
	location string
	from     int
	to       *int
}

func validateNoOverlap(periods []versionPeriod) error {
	sort.Slice(periods, func(left, right int) bool {
		return periods[left].from < periods[right].from
	})
	for index := 1; index < len(periods); index++ {
		previous := periods[index-1]
		current := periods[index]
		previousEnd := math.MaxInt
		if previous.to != nil {
			previousEnd = *previous.to
		}
		if current.from <= previousEnd {
			return fmt.Errorf("%s validity overlaps %s", current.location, previous.location)
		}
	}
	return nil
}

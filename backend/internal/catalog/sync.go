package catalog

import (
	"context"
	"encoding/json"
	"fmt"

	"ause-discovery.local/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Synchronizer struct {
	DatabasePool *pgxpool.Pool
}

func (synchronizer Synchronizer) Sync(ctx context.Context, source Source) error {
	if synchronizer.DatabasePool == nil {
		return fmt.Errorf("catalog synchronization requires a database pool")
	}
	if err := source.Validate(); err != nil {
		return err
	}

	return database.InTransaction(ctx, synchronizer.DatabasePool, func(transaction pgx.Tx) error {
		if err := syncPrograms(ctx, transaction, source.Academic.Programs); err != nil {
			return err
		}
		if err := syncMajors(ctx, transaction, source.Academic.Majors); err != nil {
			return err
		}
		if err := syncCourses(ctx, transaction, source.Academic.Courses); err != nil {
			return err
		}
		return syncTaxonomy(ctx, transaction, source.Taxonomy.Values)
	})
}

func syncPrograms(ctx context.Context, transaction pgx.Tx, programs []Program) error {
	for _, program := range programs {
		if err := ensureIdentity(ctx, transaction, "programs", program.ID, program.Key); err != nil {
			return err
		}
		for _, version := range program.Versions {
			if err := ensureProgramVersion(ctx, transaction, program.ID, version); err != nil {
				return err
			}
		}
	}
	return nil
}

func syncMajors(ctx context.Context, transaction pgx.Tx, majors []Major) error {
	for _, major := range majors {
		if err := ensureIdentity(ctx, transaction, "majors", major.ID, major.Key); err != nil {
			return err
		}
		for _, version := range major.Versions {
			if err := ensureMajorVersion(ctx, transaction, major.ID, version); err != nil {
				return err
			}
		}
	}
	return nil
}

func syncCourses(ctx context.Context, transaction pgx.Tx, courses []Course) error {
	for _, course := range courses {
		if err := ensureIdentity(ctx, transaction, "courses", course.ID, course.Key); err != nil {
			return err
		}
		for _, version := range course.Versions {
			if err := ensureCourseVersion(ctx, transaction, course.ID, version); err != nil {
				return err
			}
			for _, programVersionID := range version.ProgramVersionIDs {
				if _, err := transaction.Exec(ctx, `
					INSERT INTO course_program_versions (course_version_id, program_version_id)
					VALUES ($1, $2)
					ON CONFLICT DO NOTHING`, version.ID, programVersionID); err != nil {
					return fmt.Errorf("link course version %s to program version %s: %w", version.ID, programVersionID, err)
				}
			}
		}
	}
	return nil
}

func syncTaxonomy(ctx context.Context, transaction pgx.Tx, values []TaxonomyValue) error {
	for _, value := range values {
		labels, err := json.Marshal(value.Labels)
		if err != nil {
			return fmt.Errorf("encode taxonomy labels for %s:%s: %w", value.Dimension, value.Key, err)
		}
		if _, err := transaction.Exec(ctx, `
			INSERT INTO taxonomy_values (id, dimension, key, labels, description, sort_order, retired_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO NOTHING`, value.ID, value.Dimension, value.Key, string(labels), value.Description, value.SortOrder, value.RetiredAt); err != nil {
			return fmt.Errorf("insert taxonomy value %s:%s: %w", value.Dimension, value.Key, err)
		}
		var storedDimension, storedKey string
		if err := transaction.QueryRow(ctx, `SELECT dimension, key FROM taxonomy_values WHERE id = $1`, value.ID).Scan(&storedDimension, &storedKey); err != nil {
			return fmt.Errorf("read taxonomy value %s: %w", value.ID, err)
		}
		if storedDimension != value.Dimension || storedKey != value.Key {
			return fmt.Errorf("taxonomy value %s has a conflicting stable identity", value.ID)
		}

		if _, err := transaction.Exec(ctx, `
			UPDATE taxonomy_values
			SET labels = $3, description = $4, sort_order = $5,
				retired_at = COALESCE($6, retired_at), updated_at = now()
			WHERE id = $1 AND dimension = $2 AND key = $7
				AND (labels IS DISTINCT FROM $3::jsonb
					OR description IS DISTINCT FROM $4
					OR sort_order IS DISTINCT FROM $5
					OR ($6::timestamptz IS NOT NULL AND retired_at IS DISTINCT FROM $6))`, value.ID, value.Dimension, string(labels), value.Description, value.SortOrder, value.RetiredAt, value.Key); err != nil {
			return fmt.Errorf("update taxonomy value %s:%s: %w", value.Dimension, value.Key, err)
		}
	}
	return nil
}

func ensureIdentity(ctx context.Context, transaction pgx.Tx, tableName, id, key string) error {
	statement := fmt.Sprintf("INSERT INTO %s (id, key) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING", tableName)
	if _, err := transaction.Exec(ctx, statement, id, key); err != nil {
		return fmt.Errorf("insert %s %s: %w", tableName, key, err)
	}
	statement = fmt.Sprintf("SELECT key FROM %s WHERE id = $1", tableName)
	var storedKey string
	err := transaction.QueryRow(ctx, statement, id).Scan(&storedKey)
	if err != nil {
		return fmt.Errorf("verify %s %s: %w", tableName, key, err)
	}
	if storedKey != key {
		return fmt.Errorf("%s %s has a conflicting stable identity", tableName, id)
	}
	return nil
}

func ensureProgramVersion(ctx context.Context, transaction pgx.Tx, programID string, version ProgramVersion) error {
	if _, err := transaction.Exec(ctx, `
		INSERT INTO program_versions (id, program_id, label, valid_from_year, valid_to_year, major_required)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING`, version.ID, programID, version.Label, version.ValidFromYear, version.ValidToYear, version.MajorRequired); err != nil {
		return fmt.Errorf("insert program version %s: %w", version.ID, err)
	}
	var storedProgramID string
	if err := transaction.QueryRow(ctx, `SELECT program_id FROM program_versions WHERE id = $1`, version.ID).Scan(&storedProgramID); err != nil {
		return fmt.Errorf("read program version %s: %w", version.ID, err)
	}
	if storedProgramID != programID {
		return fmt.Errorf("program version %s has a conflicting parent", version.ID)
	}
	if _, err := transaction.Exec(ctx, `
		UPDATE program_versions
		SET label = $3, valid_from_year = $4, valid_to_year = $5, major_required = $6, updated_at = now()
		WHERE id = $1 AND program_id = $2
			AND (label, valid_from_year, valid_to_year, major_required) IS DISTINCT FROM ($3, $4, $5, $6)`, version.ID, programID, version.Label, version.ValidFromYear, version.ValidToYear, version.MajorRequired); err != nil {
		return fmt.Errorf("update program version %s: %w", version.ID, err)
	}
	return nil
}

func ensureMajorVersion(ctx context.Context, transaction pgx.Tx, majorID string, version MajorVersion) error {
	if _, err := transaction.Exec(ctx, `
		INSERT INTO major_versions (id, major_id, label, valid_from_year, valid_to_year)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING`, version.ID, majorID, version.Label, version.ValidFromYear, version.ValidToYear); err != nil {
		return fmt.Errorf("insert major version %s: %w", version.ID, err)
	}
	var storedMajorID string
	if err := transaction.QueryRow(ctx, `SELECT major_id FROM major_versions WHERE id = $1`, version.ID).Scan(&storedMajorID); err != nil {
		return fmt.Errorf("read major version %s: %w", version.ID, err)
	}
	if storedMajorID != majorID {
		return fmt.Errorf("major version %s has a conflicting parent", version.ID)
	}
	if _, err := transaction.Exec(ctx, `
		UPDATE major_versions
		SET label = $3, valid_from_year = $4, valid_to_year = $5, updated_at = now()
		WHERE id = $1 AND major_id = $2
			AND (label, valid_from_year, valid_to_year) IS DISTINCT FROM ($3, $4, $5)`, version.ID, majorID, version.Label, version.ValidFromYear, version.ValidToYear); err != nil {
		return fmt.Errorf("update major version %s: %w", version.ID, err)
	}
	return nil
}

func ensureCourseVersion(ctx context.Context, transaction pgx.Tx, courseID string, version CourseVersion) error {
	if _, err := transaction.Exec(ctx, `
		INSERT INTO course_versions (id, course_id, label, code, valid_from_year, valid_to_year)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING`, version.ID, courseID, version.Label, version.Code, version.ValidFromYear, version.ValidToYear); err != nil {
		return fmt.Errorf("insert course version %s: %w", version.ID, err)
	}
	var storedCourseID string
	if err := transaction.QueryRow(ctx, `SELECT course_id FROM course_versions WHERE id = $1`, version.ID).Scan(&storedCourseID); err != nil {
		return fmt.Errorf("read course version %s: %w", version.ID, err)
	}
	if storedCourseID != courseID {
		return fmt.Errorf("course version %s has a conflicting parent", version.ID)
	}
	if _, err := transaction.Exec(ctx, `
		UPDATE course_versions
		SET label = $3, code = $4, valid_from_year = $5, valid_to_year = $6, updated_at = now()
		WHERE id = $1 AND course_id = $2
			AND (label, code, valid_from_year, valid_to_year) IS DISTINCT FROM ($3, $4, $5, $6)`, version.ID, courseID, version.Label, version.Code, version.ValidFromYear, version.ValidToYear); err != nil {
		return fmt.Errorf("update course version %s: %w", version.ID, err)
	}
	return nil
}

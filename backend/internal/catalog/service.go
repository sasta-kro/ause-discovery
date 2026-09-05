package catalog

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Version struct {
	ID            uuid.UUID
	Key           string
	Label         string
	ValidFromYear int
	ValidToYear   *int
}

type ActiveTaxonomyValue struct {
	ID          uuid.UUID
	Dimension   string
	Key         string
	Labels      map[string]string
	Description *string
	SortOrder   int
}

type Values struct {
	Programs []Version
	Majors   []Version
	Courses  []Version
	Taxonomy []ActiveTaxonomyValue
}

type Service struct{ Pool *pgxpool.Pool }

func (service Service) List(ctx context.Context) (Values, error) {
	programs, err := service.listVersions(ctx, `SELECT v.id, i.key, v.label, v.valid_from_year, v.valid_to_year FROM program_versions v JOIN programs i ON i.id = v.program_id ORDER BY v.valid_from_year DESC, i.key, v.id`)
	if err != nil {
		return Values{}, err
	}
	majors, err := service.listVersions(ctx, `SELECT v.id, i.key, v.label, v.valid_from_year, v.valid_to_year FROM major_versions v JOIN majors i ON i.id = v.major_id ORDER BY v.valid_from_year DESC, i.key, v.id`)
	if err != nil {
		return Values{}, err
	}
	courses, err := service.listVersions(ctx, `SELECT v.id, i.key, v.label, v.valid_from_year, v.valid_to_year FROM course_versions v JOIN courses i ON i.id = v.course_id ORDER BY v.valid_from_year DESC, i.key, v.id`)
	if err != nil {
		return Values{}, err
	}
	rows, err := service.Pool.Query(ctx, `SELECT id, dimension, key, labels, description, sort_order FROM taxonomy_values WHERE retired_at IS NULL ORDER BY dimension, sort_order, key, id`)
	if err != nil {
		return Values{}, err
	}
	defer rows.Close()
	taxonomy := []ActiveTaxonomyValue{}
	for rows.Next() {
		var value ActiveTaxonomyValue
		var labels []byte
		if err := rows.Scan(&value.ID, &value.Dimension, &value.Key, &labels, &value.Description, &value.SortOrder); err != nil {
			return Values{}, err
		}
		if err := json.Unmarshal(labels, &value.Labels); err != nil {
			return Values{}, err
		}
		taxonomy = append(taxonomy, value)
	}
	if err := rows.Err(); err != nil {
		return Values{}, err
	}
	return Values{Programs: programs, Majors: majors, Courses: courses, Taxonomy: taxonomy}, nil
}

func (service Service) listVersions(ctx context.Context, query string) ([]Version, error) {
	rows, err := service.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Version{}
	for rows.Next() {
		var value Version
		if err := rows.Scan(&value.ID, &value.Key, &value.Label, &value.ValidFromYear, &value.ValidToYear); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

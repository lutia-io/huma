package pipeline

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/user"
)

type store interface {
	Insert(ctx context.Context, pipeline *pipelineDefinition) (string, error)
	Update(ctx context.Context, pipeline *pipelineDefinition) error
	GetByID(ctx context.Context, id string) (*pipelineDefinition, error)
	GetBySlug(ctx context.Context, networkID, slug, organizationID string) (*pipelineDefinition, error)
	List(ctx context.Context, params listParams) (*listResult, error)

	InsertPending(ctx context.Context, p *Pipeline) (string, error)
	GetPipelineByID(ctx context.Context, id string) (*Pipeline, error)
	ListPipelines(ctx context.Context, params runListParams) (*runListResult, error)
	GetPipelineNodeByID(ctx context.Context, id string) (*PipelineNode, error)
	ListPipelineNodesByPipelineID(ctx context.Context, pipelineID string) ([]*PipelineNode, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, pipeline *pipelineDefinition) (string, error) {
	const sql = `
		INSERT INTO public.pipeline_definitions (
			name,
			slug,
			description,
			active,
			internal,
			definition,
			network_id,
			organization_id,
			user_id,
			created_by,
			updated_by,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			now(), now()
		)
		RETURNING id`

	defJSON, err := json.Marshal(pipeline.Definition)
	if err != nil {
		return "", err
	}

	err = store.db.QueryRow(ctx, sql,
		pipeline.Name,
		pipeline.Slug,
		pipeline.Description,
		pipeline.Active,
		pipeline.Internal,
		defJSON,
		pipeline.NetworkID,
		pipeline.OrganizationID,
		pipeline.UserID,
		pipeline.CreatedBy.ID,
		pipeline.UpdatedBy.ID,
	).Scan(&pipeline.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return "", apperror.NewConflictError("Pipeline definition already exists", err)
			case "23503":
				return "", apperror.NewBadRequestError("Invalid network or organization", err)
			}
		}
		return "", err
	}
	return pipeline.ID, nil
}

func (store *postgresStore) Update(ctx context.Context, pipeline *pipelineDefinition) error {
	defJSON, err := json.Marshal(pipeline.Definition)
	if err != nil {
		return err
	}

	const sql = `
		UPDATE public.pipeline_definitions
		SET name = $2,
			slug = $3,
			description = $4,
			active = $5,
			definition = $6,
			updated_by = $7,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql,
		pipeline.ID,
		pipeline.Name,
		pipeline.Slug,
		pipeline.Description,
		pipeline.Active,
		defJSON,
		pipeline.UpdatedBy.ID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("Pipeline definition already exists", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("Pipeline definition not found", nil)
	}
	return nil
}

const pipelineSelectColumns = `
	pd.id, pd.name, pd.slug, pd.description, pd.active, pd.internal, pd.definition, pd.network_id, pd.organization_id, pd.user_id,
	pd.created_at, pd.updated_at, pd.deleted_at,
	` + user.SelectSQL

var pipelineListSelectColumns = pipelineSelectColumns

func scanPipelineDefinition(row pgx.Row, pipeline *pipelineDefinition) error {
	var defJSON []byte
	if err := row.Scan(
		&pipeline.ID,
		&pipeline.Name,
		&pipeline.Slug,
		&pipeline.Description,
		&pipeline.Active,
		&pipeline.Internal,
		&defJSON,
		&pipeline.NetworkID,
		&pipeline.OrganizationID,
		&pipeline.UserID,
		&pipeline.CreatedAt,
		&pipeline.UpdatedAt,
		&pipeline.DeletedAt,
		&pipeline.CreatedBy.ID,
		&pipeline.CreatedBy.FirstName,
		&pipeline.CreatedBy.LastName,
		&pipeline.CreatedBy.Email,
		&pipeline.UpdatedBy.ID,
		&pipeline.UpdatedBy.FirstName,
		&pipeline.UpdatedBy.LastName,
		&pipeline.UpdatedBy.Email,
	); err != nil {
		return err
	}
	if len(defJSON) == 0 {
		return nil
	}
	return json.Unmarshal(defJSON, &pipeline.Definition)
}

func collectPipelineDefinitions(rows pgx.Rows) ([]*pipelineDefinition, error) {
	defer rows.Close()

	pipelines := make([]*pipelineDefinition, 0)
	for rows.Next() {
		pipeline := &pipelineDefinition{}
		if err := scanPipelineDefinition(rows, pipeline); err != nil {
			return nil, err
		}
		pipelines = append(pipelines, pipeline)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pipelines, nil
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*pipelineDefinition, error) {
	sql := `
		SELECT` + pipelineSelectColumns + `
		FROM public.pipeline_definitions pd` + user.JoinSQL("pd") + `
		WHERE pd.id = $1 AND pd.deleted_at IS NULL`

	pipeline := &pipelineDefinition{}
	err := scanPipelineDefinition(store.db.QueryRow(ctx, sql, id), pipeline)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Pipeline definition not found", err)
		}
		return nil, err
	}
	return pipeline, nil
}

func (store *postgresStore) GetBySlug(ctx context.Context, networkID, slug, organizationID string) (*pipelineDefinition, error) {
	sql := `
		SELECT` + pipelineSelectColumns + `
		FROM public.pipeline_definitions pd` + user.JoinSQL("pd") + `
		WHERE pd.network_id = $1
			AND pd.slug = $2
			AND pd.deleted_at IS NULL
			AND (pd.organization_id IS NULL OR pd.organization_id = NULLIF($3, '')::uuid)
		ORDER BY pd.organization_id NULLS LAST
		LIMIT 1`

	pipeline := &pipelineDefinition{}
	err := scanPipelineDefinition(store.db.QueryRow(ctx, sql, networkID, slug, organizationID), pipeline)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Pipeline definition not found", err)
		}
		return nil, err
	}
	return pipeline, nil
}

func (store *postgresStore) List(ctx context.Context, params listParams) (*listResult, error) {
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)

	var total int
	if err := store.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := store.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, err
	}
	items, err := collectPipelineDefinitions(rows)
	if err != nil {
		return nil, err
	}

	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = total
	}

	return &listResult{
		Items:    items,
		Total:    total,
		Page:     params.Page,
		PageSize: pageSize,
	}, nil
}

package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
)

type store interface {
	Insert(ctx context.Context, node *NodeDefinition) (string, error)
	Update(ctx context.Context, node *NodeDefinition) error
	GetByID(ctx context.Context, id string) (*NodeDefinition, error)
	GetByIDs(ctx context.Context, ids []string) ([]*NodeDefinition, error)
	List(ctx context.Context, params listParams) (*listResult, error)
}

type postgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) store {
	return &postgresStore{db: pool}
}

func (store *postgresStore) Insert(ctx context.Context, node *NodeDefinition) (string, error) {
	const sql = `
		INSERT INTO public.node_definitions (
			name,
			slug,
			active,
			internal,
			type,
			definition,
			network_id,
			user_id,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			now(), now()
		)
		RETURNING id`

	defJSON, err := json.Marshal(node.Definition)
	if err != nil {
		return "", err
	}

	err = store.db.QueryRow(ctx, sql,
		node.Name,
		node.Slug,
		node.Active,
		node.Internal,
		node.Type,
		defJSON,
		node.NetworkID,
		node.UserID,
	).Scan(&node.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", apperror.NewConflictError("Node definition already exists", err)
		}
		return "", err
	}
	return node.ID, nil
}

func (store *postgresStore) Update(ctx context.Context, node *NodeDefinition) error {
	defJSON, err := json.Marshal(node.Definition)
	if err != nil {
		return err
	}

	const sql = `
		UPDATE public.node_definitions
		SET name = $2,
			slug = $3,
			active = $4,
			type = $5,
			definition = $6,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := store.db.Exec(ctx, sql,
		node.ID,
		node.Name,
		node.Slug,
		node.Active,
		node.Type,
		defJSON,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.NewConflictError("Node definition already exists", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NewNotFoundError("Node definition not found", nil)
	}
	return nil
}

const nodeSelectColumns = `
	id, name, slug, active, internal, type, definition, network_id, user_id,
	created_at, updated_at, deleted_at`

const nodeListSelectColumns = `
	nd.id, nd.name, nd.slug, nd.active, nd.internal, nd.type, nd.definition, nd.network_id, nd.user_id,
	nd.created_at, nd.updated_at, nd.deleted_at`

func scanNodeDefinition(row pgx.Row, node *NodeDefinition) error {
	var defJSON []byte
	if err := row.Scan(
		&node.ID,
		&node.Name,
		&node.Slug,
		&node.Active,
		&node.Internal,
		&node.Type,
		&defJSON,
		&node.NetworkID,
		&node.UserID,
		&node.CreatedAt,
		&node.UpdatedAt,
		&node.DeletedAt,
	); err != nil {
		return err
	}
	def, err := ParseDefinition(node.Type, defJSON)
	if err != nil {
		return err
	}
	node.Definition = def
	return nil
}

func collectNodeDefinitions(rows pgx.Rows) ([]*NodeDefinition, error) {
	defer rows.Close()

	nodes := make([]*NodeDefinition, 0)
	for rows.Next() {
		n := &NodeDefinition{}
		if err := scanNodeDefinition(rows, n); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (store *postgresStore) GetByID(ctx context.Context, id string) (*NodeDefinition, error) {
	const sql = `
		SELECT` + nodeSelectColumns + `
		FROM public.node_definitions
		WHERE id = $1 AND deleted_at IS NULL`

	node := &NodeDefinition{}
	err := scanNodeDefinition(store.db.QueryRow(ctx, sql, id), node)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NewNotFoundError("Node definition not found", err)
		}
		return nil, err
	}
	return node, nil
}

func (store *postgresStore) GetByIDs(ctx context.Context, ids []string) ([]*NodeDefinition, error) {
	if len(ids) == 0 {
		return []*NodeDefinition{}, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	sql := `
		SELECT` + nodeSelectColumns + `
		FROM public.node_definitions
		WHERE deleted_at IS NULL AND id IN (` + strings.Join(placeholders, ", ") + `)`

	rows, err := store.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return collectNodeDefinitions(rows)
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
	items, err := collectNodeDefinitions(rows)
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

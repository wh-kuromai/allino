package objects

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

//
// Phase3
//

func (s *SQLObjects) GetRoot(
	ctx context.Context,
) (Node, error) {

	query := `
SELECT
	id,
	name,
	parent_id,
	thread_parent_id,
	entry_type,
	priority,
	created_at,
	updated_at,
	metadata,
	body,
	num_children
FROM node
WHERE parent_id IS NULL
LIMIT 1
`

	row := s.db.QueryRowContext(ctx, query)

	n, err := scanNode(row)
	if err != nil {
		return Node{}, err
	}

	return *n, nil
}

func (s *SQLObjects) GetNodes(
	ctx context.Context,
	nodeId []uint64,
) ([]Node, error) {

	if len(nodeId) == 0 {
		return []Node{}, nil
	}

	query := fmt.Sprintf(`
SELECT
	id,
	name,
	parent_id,
	thread_parent_id,
	entry_type,
	priority,
	created_at,
	updated_at,
	metadata,
	body,
	num_children
FROM node
WHERE id IN (%s)
`,
		buildInClause(len(nodeId)),
	)

	args := make([]interface{}, 0, len(nodeId))
	for _, v := range nodeId {
		args = append(args, v)
	}

	rows, err := s.db.QueryContext(
		ctx,
		query,
		args...,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Node

	for rows.Next() {

		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, *n)
	}

	return result, rows.Err()
}

//
// FindParents
//
// input:
//   [leafA, leafB]
//
// output:
//   all ancestors
//
// usecase:
//   ACL
//   organization path reconstruction
//

func (s *SQLObjects) FindParents(
	ctx context.Context,
	nodeId []uint64,
) ([]Node, error) {

	if len(nodeId) == 0 {
		return []Node{}, nil
	}

	query := fmt.Sprintf(`
WITH RECURSIVE ancestors AS (

	-- seed
	SELECT
		id,
		name,
		parent_id,
		thread_parent_id,
		entry_type,
		priority,
		created_at,
		updated_at,
		metadata,
		num_children,
		0 AS depth
	FROM node
	WHERE id IN (%s)

	UNION

	-- parents
	SELECT
		p.id,
		p.name,
		p.parent_id,
		p.thread_parent_id,
		p.entry_type,
		p.priority,
		p.created_at,
		p.updated_at,
		p.metadata,
		p.num_children
		a.depth + 1
	FROM node p
	INNER JOIN ancestors a
		ON a.parent_id = p.id
)

SELECT DISTINCT
	id,
	name,
	parent_id,
	thread_parent_id,
	entry_type,
	priority,
	created_at,
	updated_at,
	metadata,
	body,
	num_children
FROM ancestors
ORDER BY depth ASC
`,
		buildInClause(len(nodeId)),
	)

	args := make([]interface{}, 0, len(nodeId))
	for _, v := range nodeId {
		args = append(args, v)
	}

	rows, err := s.db.QueryContext(
		ctx,
		query,
		args...,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Node

	for rows.Next() {

		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, *n)
	}

	return result, rows.Err()
}

//
// helpers
//

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanNode(
	s scanner,
) (*Node, error) {

	n := &Node{}

	var parentId sql.NullInt64
	var threadParentId sql.NullInt64

	err := s.Scan(
		&n.Id,
		&n.Name,
		&parentId,
		&threadParentId,
		&n.NodeType,
		&n.Priority,
		&n.CreatedAt,
		&n.UpdatedAt,
		&n.Metadata,
		&n.Body,
		&n.NumChildren,
	)

	if err != nil {
		return nil, err
	}

	if parentId.Valid {
		n.ParentId = parentId.Int64
	}

	if threadParentId.Valid {
		n.ThreadParentId = threadParentId.Int64
	}

	return n, nil
}

func buildInClause(
	n int,
) string {

	if n <= 0 {
		return ""
	}

	return strings.TrimRight(
		strings.Repeat("?,", n),
		",",
	)
}

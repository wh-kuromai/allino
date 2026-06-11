package objects

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type SQLObjects struct {
	db *sql.DB
}

//
// Node CRUD
//

func (s *SQLObjects) CreateNode(
	ctx context.Context,
	node *Node,
) error {

	now := time.Now().UTC()

	node.CreatedAt = now
	node.UpdatedAt = now

	query := `
INSERT INTO node (
	name,
	parent_id,
	thread_parent_id,
	entry_type,
	priority,
	created_at,
	updated_at,
	metadata,
	num_children
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id
`

	err := s.db.QueryRowContext(
		ctx,
		query,
		node.Name,
		nullInt64(node.ParentId),
		nullInt64(node.ThreadParentId),
		node.NodeType,
		node.Priority,
		node.CreatedAt,
		node.UpdatedAt,
		node.Metadata,
		node.NumChildren,
	).Scan(&node.Id)

	if err != nil {
		return err
	}

	return nil
}

func (s *SQLObjects) UpdateNode(
	ctx context.Context,
	node *Node,
) error {

	node.UpdatedAt = time.Now().UTC()

	query := `
UPDATE node
SET
	name = ?,
	parent_id = ?,
	thread_parent_id = ?,
	entry_type = ?,
	priority = ?,
	updated_at = ?,
	metadata = ?,
	body = ?,
	num_children = ?
WHERE id = ?
`

	result, err := s.db.ExecContext(
		ctx,
		query,
		node.Name,
		nullInt64(node.ParentId),
		nullInt64(node.ThreadParentId),
		node.NodeType,
		node.Priority,
		node.UpdatedAt,
		node.Metadata,
		node.Body,
		node.NumChildren,
		node.Id,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *SQLObjects) DeleteNode(
	ctx context.Context,
	nodeId int64,
) error {

	result, err := s.db.ExecContext(
		ctx,
		`
DELETE FROM node
WHERE id = ?
`,
		nodeId,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

//
// Link CRUD
//

func (s *SQLObjects) CreateLink(
	ctx context.Context,
	link *Link,
) error {

	now := time.Now().UTC()

	link.CreatedAt = now
	link.UpdatedAt = now

	query := `
INSERT INTO link (
	from_id,
	to_id,
	link_type,
	created_at,
	updated_at,
	metadata
)
VALUES (?, ?, ?, ?, ?, ?)
`

	_, err := s.db.ExecContext(
		ctx,
		query,
		link.FromId,
		link.ToId,
		link.LinkType,
		link.CreatedAt,
		link.UpdatedAt,
		link.Metadata,
	)

	return err
}

func (s *SQLObjects) UpdateLink(
	ctx context.Context,
	link *Link,
) error {

	link.UpdatedAt = time.Now().UTC()

	query := `
UPDATE link
SET
	updated_at = ?,
	metadata = ?
WHERE
	from_id = ?
	AND to_id = ?
	AND link_type = ?
`

	result, err := s.db.ExecContext(
		ctx,
		query,
		link.UpdatedAt,
		link.Metadata,
		link.FromId,
		link.ToId,
		link.LinkType,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *SQLObjects) DeleteLink(
	ctx context.Context,
	link *Link,
) error {

	query := `
DELETE FROM link
WHERE
	from_id = ?
	AND to_id = ?
	AND link_type = ?
`

	result, err := s.db.ExecContext(
		ctx,
		query,
		link.FromId,
		link.ToId,
		link.LinkType,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

//
// helpers
//

func nullInt64(v int64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

var ErrInvalidId = errors.New("invalid id")
var ErrUnauthorized = errors.New("unauthorized")

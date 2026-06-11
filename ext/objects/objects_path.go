package objects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"
)

//
// Phase2
//

// path:
//   /root/folder1/folder2
//
// NOTE:
// root node itself is assumed to have parent_id IS NULL.
//

func (s *SQLObjects) Get(
	ctx context.Context,
	p string,
) (*Node, error) {

	nodes, err := s.GetAll(ctx, p)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, sql.ErrNoRows
	}

	return nodes[len(nodes)-1], nil
}

func (s *SQLObjects) GetAll(
	ctx context.Context,
	p string,
) ([]*Node, error) {

	elems, err := getPathElement(p)
	if err != nil {
		return nil, err
	}

	if len(elems) == 0 {
		return nil, errors.New("empty path")
	}

	//
	// Recursive path traversal
	//
	// depth:
	//   0 => root
	//   1 => child
	//   2 => grandchild
	//

	query := `
WITH RECURSIVE path_tree AS (

	-- root
	SELECT
		n.id,
		n.name,
		n.parent_id,
		n.thread_parent_id,
		n.entry_type,
		n.priority,
		n.created_at,
		n.updated_at,
		n.metadata,
		n.num_children,
		0 AS depth
	FROM node n
	WHERE
		n.parent_id IS NULL
		AND n.name = ?

	UNION ALL

	-- descendants
	SELECT
		c.id,
		c.name,
		c.parent_id,
		c.thread_parent_id,
		c.entry_type,
		c.priority,
		c.created_at,
		c.updated_at,
		c.metadata,
		c.num_children,
		pt.depth + 1
	FROM node c
	INNER JOIN path_tree pt
		ON c.parent_id = pt.id
	WHERE
		c.name = ?
)

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
	depth
FROM path_tree
ORDER BY depth ASC
`

	//
	// IMPORTANT:
	// recursive query only accepts one step name.
	//
	// so:
	//   repeatedly execute narrowing query
	//

	var result []*Node

	//
	// first element = root
	//

	currentName := elems[0]

	rows, err := s.db.QueryContext(
		ctx,
		query,
		currentName,
		"", // dummy
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var currentNode *Node

	for rows.Next() {
		n := &Node{}

		var parentId sql.NullInt64
		var threadParentId sql.NullInt64
		var depth int

		err := rows.Scan(
			&n.Id,
			&n.Name,
			&parentId,
			&threadParentId,
			&n.NodeType,
			&n.Priority,
			&n.CreatedAt,
			&n.UpdatedAt,
			&n.Metadata,
			&n.NumChildren,
			&depth,
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

		currentNode = n
		result = append(result, n)
	}

	if currentNode == nil {
		return nil, sql.ErrNoRows
	}

	//
	// walk descendants
	//

	for i := 1; i < len(elems); i++ {

		name := elems[i]

		n := &Node{}

		var parentId sql.NullInt64
		var threadParentId sql.NullInt64

		err := s.db.QueryRowContext(
			ctx,
			`
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
	num_children
FROM node
WHERE
	parent_id = ?
	AND name = ?
`,
			currentNode.Id,
			name,
		).Scan(
			&n.Id,
			&n.Name,
			&parentId,
			&threadParentId,
			&n.NodeType,
			&n.Priority,
			&n.CreatedAt,
			&n.UpdatedAt,
			&n.Metadata,
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

		currentNode = n

		result = append(result, n)
	}

	return result, nil
}

func getPathElement(
	p string,
) ([]string, error) {

	if p == "" {
		return nil, errors.New("empty path")
	}

	//
	// normalize:
	//   a//b///c -> a/b/c
	//

	cleaned := path.Clean(p)

	//
	// prevent relative path
	//

	if !strings.HasPrefix(cleaned, "/") {
		return nil, fmt.Errorf(
			"path must start with '/' : %s",
			p,
		)
	}

	//
	// "/" only
	//

	if cleaned == "/" {
		return []string{}, nil
	}

	//
	// "/a/b/c"
	// -> ["a", "b", "c"]
	//

	parts := strings.Split(
		strings.TrimPrefix(cleaned, "/"),
		"/",
	)

	for _, v := range parts {
		if v == "" {
			return nil, errors.New("invalid path")
		}
	}

	return parts, nil
}

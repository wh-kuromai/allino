package objects

import (
	"context"
	"database/sql"
	"fmt"
)

//
// Phase4
//

//
// FindChild
//
// thread=false:
//   parent_id based children
//
// thread=true:
//   thread_parent_id based replies
//

func (s *SQLObjects) FindChild(
	ctx context.Context,
	nodeId []int64,
	thread bool,
	offset,
	limit int,
) ([]Node, error) {

	if len(nodeId) == 0 {
		return []Node{}, nil
	}

	column := "parent_id"

	if thread {
		column = "thread_parent_id"
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
WHERE %s IN (%s)
ORDER BY priority DESC, created_at DESC
LIMIT ?
OFFSET ?
`,
		column,
		buildInClause(len(nodeId)),
	)

	args := make([]interface{}, 0, len(nodeId)+2)

	for _, v := range nodeId {
		args = append(args, v)
	}

	args = append(args, limit)
	args = append(args, offset)

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
// FindLinkingNode
//
// target <- something
//
// example:
//   tweet <- like
//   resource <- acl
//

func (s *SQLObjects) FindLinkingNode(
	ctx context.Context,
	toNodeId []int64,
) ([]Link, error) {

	if len(toNodeId) == 0 {
		return []Link{}, nil
	}

	query := fmt.Sprintf(`
SELECT
	l.from_id,
	l.to_id,
	l.link_type,
	l.created_at,
	l.updated_at,
	l.metadata,

	n.id,
	n.name,
	n.parent_id,
	n.thread_parent_id,
	n.entry_type,
	n.priority,
	n.created_at,
	n.updated_at,
	n.metadata,
	n.num_children

FROM link l

INNER JOIN node n
	ON n.id = l.from_id

WHERE l.to_id IN (%s)

ORDER BY l.created_at DESC
`,
		buildInClause(len(toNodeId)),
	)

	args := make([]interface{}, 0, len(toNodeId))

	for _, v := range toNodeId {
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

	var result []Link

	for rows.Next() {

		link := Link{}

		var fromNode Node

		err := scanLinkAndNode(
			rows,
			&link,
			&fromNode,
		)

		if err != nil {
			return nil, err
		}

		link.FromIdENode = &fromNode

		result = append(result, link)
	}

	return result, rows.Err()
}

//
// FindLinkedNode
//
// something -> target
//
// example:
//   user -> liked posts
//   user -> joined organizations
//

func (s *SQLObjects) FindLinkedNode(
	ctx context.Context,
	fromNodeId []int64,
	linkType []string,
) ([]Link, error) {

	if len(fromNodeId) == 0 {
		return []Link{}, nil
	}

	query := `
SELECT
	l.from_id,
	l.to_id,
	l.link_type,
	l.created_at,
	l.updated_at,
	l.metadata,

	n.id,
	n.name,
	n.parent_id,
	n.thread_parent_id,
	n.entry_type,
	n.priority,
	n.created_at,
	n.updated_at,
	n.metadata,
	n.num_children

FROM link l

INNER JOIN node n
	ON n.id = l.to_id
`

	var where []string
	var args []interface{}

	//
	// from_id filter
	//

	where = append(
		where,
		fmt.Sprintf(
			"l.from_id IN (%s)",
			buildInClause(len(fromNodeId)),
		),
	)

	for _, v := range fromNodeId {
		args = append(args, v)
	}

	//
	// link_type filter
	//

	if len(linkType) > 0 {

		where = append(
			where,
			fmt.Sprintf(
				"l.link_type IN (%s)",
				buildInClause(len(linkType)),
			),
		)

		for _, v := range linkType {
			args = append(args, v)
		}
	}

	query += `
WHERE ` + joinWhere(where) + `
ORDER BY l.created_at DESC
`

	rows, err := s.db.QueryContext(
		ctx,
		query,
		args...,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Link

	for rows.Next() {

		link := Link{}

		var toNode Node

		err := scanLinkAndNode(
			rows,
			&link,
			&toNode,
		)

		if err != nil {
			return nil, err
		}

		link.ToIdENode = &toNode

		result = append(result, link)
	}

	return result, rows.Err()
}

//
// helpers
//

func scanLinkAndNode(
	s scanner,
	link *Link,
	node *Node,
) error {

	var parentId sql.NullInt64
	var threadParentId sql.NullInt64

	err := s.Scan(
		&link.FromId,
		&link.ToId,
		&link.LinkType,
		&link.CreatedAt,
		&link.UpdatedAt,
		&link.Metadata,

		&node.Id,
		&node.Name,
		&parentId,
		&threadParentId,
		&node.NodeType,
		&node.Priority,
		&node.CreatedAt,
		&node.UpdatedAt,
		&node.Metadata,
		&node.NumChildren,
	)

	if err != nil {
		return err
	}

	if parentId.Valid {
		node.ParentId = parentId.Int64
	}

	if threadParentId.Valid {
		node.ThreadParentId = threadParentId.Int64
	}

	return nil
}

func joinWhere(
	v []string,
) string {

	if len(v) == 0 {
		return "1=1"
	}

	result := v[0]

	for i := 1; i < len(v); i++ {
		result += " AND " + v[i]
	}

	return result
}

package objects

import (
	"context"
	"database/sql"
)

type NodeAndParents struct {
	Node    Node
	Parents []Node
}

//
// FindLinkedNodeAndParents
//
// example:
//
// user -(member)-> team
//
// return:
//
// [
//   {
//     Node: team,
//     Parents: [root, org, division],
//   }
// ]
//
//
//
// single SQL query version
//

func (s *SQLObjects) FindLinkedNodeAndParents(
	ctx context.Context,
	fromNodeId int64,
	linkType string,
) ([]NodeAndParents, error) {

	query := `
WITH RECURSIVE linked AS (

	-- linked target nodes
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

		n.id AS origin_id
	FROM link l
	INNER JOIN node n
		ON n.id = l.to_id
	WHERE
		l.from_id = ?
		AND l.link_type = ?

),

ancestors AS (

	-- self
	SELECT
		l.id,
		l.name,
		l.parent_id,
		l.thread_parent_id,
		l.entry_type,
		l.priority,
		l.created_at,
		l.updated_at,
		l.metadata,
		l.num_children,

		l.origin_id,

		0 AS depth
	FROM linked l

	UNION ALL

	-- parent traversal
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
		p.num_children,

		a.origin_id,

		a.depth + 1
	FROM node p
	INNER JOIN ancestors a
		ON a.parent_id = p.id
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

	origin_id,
	depth

FROM ancestors

ORDER BY
	origin_id,
	depth DESC
`

	rows, err := s.db.QueryContext(
		ctx,
		query,
		fromNodeId,
		linkType,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	//
	// grouping:
	//
	// origin_id => NodeAndParents
	//

	type holder struct {
		Node    Node
		Parents []Node
		HasSelf bool
	}

	group := map[int64]*holder{}

	for rows.Next() {

		n := Node{}

		var parentId sql.NullInt64
		var threadParentId sql.NullInt64

		var originId int64
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

			&originId,
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

		h, ok := group[originId]
		if !ok {

			h = &holder{}

			group[originId] = h
		}

		//
		// depth=0 => linked node itself
		//

		if depth == 0 {

			h.Node = n
			h.HasSelf = true

			continue
		}

		//
		// parent chain
		//

		h.Parents = append(
			h.Parents,
			n,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make(
		[]NodeAndParents,
		0,
		len(group),
	)

	for _, v := range group {

		if !v.HasSelf {
			continue
		}

		result = append(
			result,
			NodeAndParents{
				Node:    v.Node,
				Parents: v.Parents,
			},
		)
	}

	return result, nil
}

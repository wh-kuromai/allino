package objects

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/wh-kuromai/allino"
	"golang.org/x/sync/singleflight"
)

type ObjectsConfig struct {
}

var Root *AllObjects
var ObjectsExtension *allino.Extension[ObjectsConfig, any]

func init() {
	ObjectsExtension = allino.NewExtension[ObjectsConfig, any](
		"objects",
		&allino.ExtOption{
			SQLSchema: func(driver string) string {
				return `

CREATE TABLE IF NOT EXISTS node (
	id BIGINT PRIMARY KEY,
  name TEXT NOT NULL,
  parent_id BIGINT,
  thread_parent_id BIGINT,
	entry_type TEXT NOT NULL,
  priority INTEGER NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	metadata BLOB NOT NULL,
	body BLOB,
  num_children INTEGER NOT NULL DEFAULT 0
)

CREATE TABLE IF NOT EXISTS link (
	from_id BIGINT,
  to_id BIGINT, -- folder
	link_type TEXT NOT NULL, -- "", ""
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	metadata BLOB NOT NULL,

	PRIMARY KEY(from_id, to_id, link_type)
)


CREATE TRIGGER node_insert_trigger
AFTER INSERT ON node
WHEN NEW.parent_id IS NOT NULL
BEGIN
  UPDATE node
  SET num_children = num_children + 1
  WHERE id = NEW.parent_id;
END;

CREATE TRIGGER node_delete_trigger
AFTER DELETE ON node
WHEN OLD.parent_id IS NOT NULL
BEGIN
  UPDATE node
  SET num_children = num_children - 1
  WHERE id = OLD.parent_id;
END;

CREATE TRIGGER node_move_trigger
AFTER UPDATE OF parent_id ON node
BEGIN
  UPDATE node
  SET num_children = num_children - 1
  WHERE id = OLD.parent_id;

  UPDATE node
  SET num_children = num_children + 1
  WHERE id = NEW.parent_id;
END;


CREATE INDEX idx_node_parent
ON node(parent_id, priority, updated_at);

CREATE INDEX idx_node_thread_parent
ON node(parent_id, priority, thread_parent_id, updated_at);

CREATE INDEX idx_node_name
ON node(name);

CREATE INDEX idx_node_type
ON node(node_type);

CREATE INDEX idx_link_from_type
ON link(from_id, link_type);

CREATE INDEX idx_link_to
ON link(to_id);

CREATE UNIQUE INDEX idx_node_parent_name
ON node(parent_id, name);


				`
			},
			OnInit: func(s *allino.Server, virtual *allino.Request) error {
				vfs := &AllObjects{
					SQLObjects: SQLObjects{
						db: s.SQL,
					},
					server: s,
				}

				Root = vfs
				return nil
			},
			OnInjection: func(r *allino.Request, targets []*allino.InjectionTarget) error {
				ids := make([]uint64, len(targets))
				for i, t := range targets {
					ids[i] = r.Sqids().DecodeN(t.Input)
				}

				uid, _, _, err := r.User()
				if err != nil {
					return err
				}

				nodes, err := Root.GetNodes(r.Context(), ids)
				if err != nil {
					return err
				}

				acl, err := Root.ResolveACL(r, uid, nodes)
				if err != nil {
					return err
				}

				if !acl.CheckUserScope("admin") && !acl.CheckNodeScopeAll("read") {
					return ErrUnauthorized
				}

				for i, n := range nodes {
					err = json.Unmarshal(n.Metadata, targets[i].Reference)
					if err != nil {
						return err
					}
				}

				return nil
			},
		},
	)
}

type Node struct {
	Id             int64
	Name           string
	ParentId       int64
	ThreadParentId int64
	NodeType       string
	Priority       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Metadata       []byte
	Body           []byte
	NumChildren    int
}

type Link struct {
	FromId    int64
	ToId      int64
	LinkType  string
	CreatedAt time.Time
	UpdatedAt time.Time
	Metadata  []byte

	FromIdENode *Node
	ToIdENode   *Node
}

type UserMetadataSet struct {
	Scopes   []string `json:"scopes"`
	Parents  []int64  `json:"parents"`
	NodePath string   `json:"nodepath"`
	IssuedAt int64    `json:"issuedAt"`
}

type ResourceMetadataSet struct {
	ACL map[int64][]string `json:"acl"`

	// cached / need revoke check.
	NodePathCache string `json:"nodepath_cache"`
	IssuedAt      int64  `json:"issuedAt"`
}

type AllObjects struct {
	SQLObjects

	sf     singleflight.Group
	server *allino.Server
}

func (a *AllObjects) Get(ctx context.Context, path string) (node *Node, err error) {
	return nil, nil
}

type ACL struct {
	UserScopes       map[string]string
	MergedNodeScopes []map[string]string
}

func (a *ACL) CheckUserScope(scope string) bool {
	_, ok := a.UserScopes[scope]
	if ok {
		return true
	}

	return false
}

func (a *ACL) CheckNodeScopeAll(scope string) bool {
	for _, v := range a.MergedNodeScopes {
		_, ok := v[scope]
		if !ok {
			return false
		}
	}
	return true
}

func (a *AllObjects) ResolveACL(r *allino.Request, uid string, nodes []Node) (*ACL, error) {
	metacache, err := a.getUserMetaCache(r, uid)
	if err != nil {
		return nil, err
	}

	dataMetas, err := a.getNodeMetaCache(r, nodes)
	if err != nil {
		return nil, err
	}

	return a.makeACL(metacache, dataMetas)
}

func (a *AllObjects) makeACL(metacache *UserMetadataSet, dataMetas []*ResourceMetadataSet) (*ACL, error) {

	acl := &ACL{
		UserScopes:       make(map[string]string),
		MergedNodeScopes: make([]map[string]string, len(dataMetas)),
	}

	for _, v := range metacache.Scopes {
		idx := strings.Index(v, ":")
		if idx > 0 {
			acl.UserScopes[v[:idx]] = v[idx+1:]
		} else {
			acl.UserScopes[v] = v
		}
	}

	for i, v := range dataMetas {
		if v == nil {
			continue
		}

		aclok := false
		mergedScopes := map[string]string{}
		acl.MergedNodeScopes[i] = mergedScopes
		for _, p := range metacache.Parents {
			scope, ok := v.ACL[p]
			if ok {
				for _, v := range scope {
					idx := strings.Index(v, ":")
					if idx > 0 {
						mergedScopes[v[:idx]] = v[idx+1:]
					} else {
						mergedScopes[v] = v
					}
				}
			}
		}

		if !aclok {
			return nil, ErrUnauthorized
		}
	}

	return acl, nil
}

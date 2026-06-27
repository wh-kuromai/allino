package objects

import (
	"encoding/json"
	"time"

	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/ext/revoker"
)

func (a *AllObjects) getNodeMetaCache(
	r *allino.Runtime,
	nodes []Node,
) ([]*ResourceMetadataSet, error) {

	results := make([]*ResourceMetadataSet, len(nodes))

	for i, n := range nodes {

		meta, err := a.getParentMetaCached(
			r,
			uint64(n.ParentId),
		)

		if err != nil {
			return nil, err
		}

		results[i] = meta
	}

	return results, nil
}

func (a *AllObjects) getParentMetaCached(
	r *allino.Runtime,
	parentID uint64,
) (*ResourceMetadataSet, error) {

	key := "allino:objects:id:" +
		r.Sqids().EncodeN(parentID)

	// -------------------------
	// redis
	// -------------------------

	s, err := r.Redis().Get(
		r.Context(),
		key,
	).Result()

	if err == nil {

		var meta ResourceMetadataSet

		if err := json.Unmarshal(
			[]byte(s),
			&meta,
		); err != nil {
			return nil, err
		}

		revoked, _ := revoker.IsRevoked(r,
			meta.NodePathCache,
			time.Unix(meta.IssuedAt, 0),
		)

		if !revoked {
			return &meta, nil
		}
	}

	// -------------------------
	// singleflight
	// -------------------------

	v, err, _ := a.sf.Do(
		key,
		func() (interface{}, error) {

			// double check
			s, err := r.Redis().Get(
				r.Context(),
				key,
			).Result()

			if err == nil {

				var meta ResourceMetadataSet

				if err := json.Unmarshal(
					[]byte(s),
					&meta,
				); err != nil {
					return nil, err
				}

				revoked, _ := revoker.IsRevoked(r,
					meta.NodePathCache,
					time.Unix(meta.IssuedAt, 0),
				)

				if !revoked {
					return &meta, nil
				}
			}

			return a.buildParentMeta(
				r,
				parentID,
				key,
			)
		},
	)

	if err != nil {
		return nil, err
	}

	return v.(*ResourceMetadataSet), nil
}

func (a *AllObjects) buildParentMeta(
	r *allino.Runtime,
	parentID uint64,
	key string,
) (*ResourceMetadataSet, error) {

	chain, err := a.FindParents(
		r.Context(),
		[]uint64{parentID},
	)

	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()

	metas := make([]*ResourceMetadataSet, 0, len(chain))

	path := ""

	for _, n := range chain {

		path += "/" + n.Name

		if len(n.Metadata) == 0 {
			continue
		}

		var meta ResourceMetadataSet

		if err := json.Unmarshal(
			n.Metadata,
			&meta,
		); err != nil {
			return nil, err
		}

		metas = append(metas, &meta)
	}

	merged := &ResourceMetadataSet{
		ACL:           mergeACLFirstWin(metas),
		NodePathCache: path,
		IssuedAt:      now,
	}

	b, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}

	if err := r.Redis().Set(
		r.Context(),
		key,
		b,
		time.Hour,
	).Err(); err != nil {
		return nil, err
	}

	return merged, nil
}

func mergeACLFirstWin(metas []*ResourceMetadataSet) map[int64][]string {
	result := map[int64][]string{}

	for _, m := range metas {
		if m == nil {
			continue
		}

		for k, v := range m.ACL {
			// 先勝ち
			if _, exists := result[k]; exists {
				continue
			}

			result[k] = v
		}
	}

	return result
}

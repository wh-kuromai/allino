package objects

import (
	"encoding/json"
	"time"

	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/ext/revoker"
)

func (a *AllObjects) getUserMetaCache(r *allino.Runtime, uid string) (*UserMetadataSet, error) {
	usermetacache, ok, err := allino.GetRedisSessionEntry[UserMetadataSet](r, uid, "allino.objects")
	if err == nil && ok {
		revoked, _ := revoker.IsRevoked(r, usermetacache.NodePath, time.Unix(usermetacache.IssuedAt, 0))
		if !revoked {
			return usermetacache, nil
		}
	}

	userAncesters, err := a.FindParents(r.Context(), []uint64{r.Sqids().Decode(uid)[0]})
	if err != nil {
		return nil, err
	}
	usermetas, err := metadataDecodeMerge[UserMetadataSet](userAncesters)
	// usermetas[].Scope をスペース区切りで切って、先勝ちでマージする。

	scopes := mergeScopesFirstWin(usermetas)

	parents := make([]int64, len(userAncesters))
	path := ""
	for i, u := range userAncesters {
		path = "/" + u.Name + path
		parents[i] = u.Id
	}

	now := time.Now()

	ums := &UserMetadataSet{
		Scopes:   scopes,
		Parents:  parents,
		NodePath: path,
		IssuedAt: now.Unix(),
	}

	*usermetacache = *ums

	return ums, nil
}

// "a b c" + "b d" => ["a", "b", "c", "d"]
// 先に出たものを優先
func mergeScopesFirstWin(metas []*UserMetadataSet) []string {
	seen := map[string]struct{}{}
	result := []string{}

	for _, m := range metas {
		if m == nil {
			continue
		}

		for _, scope := range m.Scopes {
			if _, ok := seen[scope]; ok {
				continue
			}

			seen[scope] = struct{}{}
			result = append(result, scope)
		}
	}

	return result
}

func metadataDecodeMerge[T any](target []Node) ([]*T, error) {
	if len(target) == 0 {
		return nil, ErrInvalidId
	}

	metas := make([]*T, len(target))
	for i, t := range target {
		var tt T
		err := json.Unmarshal(t.Metadata, &tt)
		if err != nil {
			return nil, err
		}

		metas[i] = &tt
	}

	return metas, nil
}

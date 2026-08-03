package allino

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	pgadapter "github.com/casbin/casbin-pg-adapter"
	"github.com/casbin/casbin/v2"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	"github.com/go-pg/pg/v10"
)

type CasbinConfig struct {
	ModelPath   string `json:"model"`
	PolicyPath  string `json:"policy"`
	Adapter     string `json:"adapter"`
	Driver      string `json:"driver"`
	DSN         string `json:"dsn"`
	Database    string `json:"database"`
	Table       string `json:"table"`
	TenantClaim string `json:"tenant_claim"`
}

var (
	aclBraceVarRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	aclColonVarRe = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)

	ErrCasbinNotConfigured = NewCodeError(500, "casbin_not_configured", "casbin is not configured")
	ErrACLVariableMissing  = NewCodeError(400, "acl_variable_missing", "acl variable is missing")
	ErrACLForbidden        = NewCodeError(403, "acl_forbidden", "acl forbidden")
)

func (c *CasbinConfig) setup(s *Server) (*casbin.SyncedEnforcer, error) {
	if c.ModelPath == "" {
		return nil, nil
	}

	if c.Adapter == "" {
		c.Adapter = "csv"
	}
	if c.TenantClaim == "" {
		c.TenantClaim = "tenant"
	}

	modelPath := s.configPath(c.ModelPath)
	switch strings.ToLower(c.Adapter) {
	case "csv", "cvs", "file":
		if c.PolicyPath == "" {
			return casbin.NewSyncedEnforcer(modelPath)
		}
		return casbin.NewSyncedEnforcer(modelPath, fileadapter.NewAdapter(s.configPath(c.PolicyPath)))
	case "postgres", "postgresql":
		dsn := c.DSN
		if dsn == "" {
			dsn = s.Config.SQL.DSN
		}
		if dsn == "" {
			return nil, fmt.Errorf("casbin postgres adapter requires casbin.dsn or sql.dsn")
		}

		var adapter *pgadapter.Adapter
		var err error
		if c.Table != "" {
			options, err := pg.ParseURL(dsn)
			if err != nil {
				return nil, err
			}
			if c.Database != "" {
				options.Database = c.Database
			}
			adapter, err = pgadapter.NewAdapterByDB(pg.Connect(options), pgadapter.WithTableName(c.Table))
			if err != nil {
				return nil, err
			}
		} else {
			adapterArgs := []string{}
			if c.Database != "" {
				adapterArgs = append(adapterArgs, c.Database)
			}
			adapter, err = pgadapter.NewAdapter(dsn, adapterArgs...)
			if err != nil {
				return nil, err
			}
		}
		return casbin.NewSyncedEnforcer(modelPath, adapter)
	default:
		return nil, fmt.Errorf("unsupported casbin adapter: %s", c.Adapter)
	}
}

func (s *Server) configPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if s.Config.ConfigDir != "" {
		return filepath.Join(s.Config.ConfigDir, path)
	}
	if s.Config.AbsWorkDir != "" {
		return filepath.Join(s.Config.AbsWorkDir, path)
	}
	return path
}

func (r *Runtime) Enforcer() *casbin.SyncedEnforcer {
	if r == nil || r.server == nil {
		return nil
	}
	return r.server.Casbin
}

func (r *Runtime) enforceACL(opt *Option, input any) error {
	if opt == nil || opt.ACLResource == "" {
		return nil
	}
	enforcer := r.Enforcer()
	if enforcer == nil {
		return ErrCasbinNotConfigured
	}

	uid, _, _, err := r.User()
	if err != nil {
		return err
	}
	tenant, err := r.Claim(r.config.Casbin.TenantClaim)
	if err != nil {
		return err
	}
	if tenant == "" {
		return ErrACLForbidden
	}

	vars := aclVars(input)
	resource, err := expandACLTemplate(opt.ACLResource, vars)
	if err != nil {
		return err
	}
	action, err := expandACLTemplate(opt.ACLAction, vars)
	if err != nil {
		return err
	}
	if action == "" {
		action = "access"
	}

	allowed, err := enforcer.Enforce(tenant, uid, resource, action)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrACLForbidden
	}
	return nil
}

func expandACLTemplate(tmpl string, vars map[string]string) (string, error) {
	var missing string
	out := aclBraceVarRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		name := match[1 : len(match)-1]
		if value, ok := vars[name]; ok {
			return value
		}
		missing = name
		return match
	})
	out = aclColonVarRe.ReplaceAllStringFunc(out, func(match string) string {
		name := match[1:]
		if value, ok := vars[name]; ok {
			return value
		}
		missing = name
		return match
	})
	if missing != "" {
		return "", ErrACLVariableMissing
	}
	return out, nil
}

func aclVars(input any) map[string]string {
	vars := map[string]string{}
	collectACLVals(reflect.ValueOf(input), vars)
	return vars
}

func collectACLVals(v reflect.Value, vars map[string]string) {
	if !v.IsValid() {
		return
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous {
			continue
		}

		fv := v.Field(i)
		_, aclTagged := sf.Tag.Lookup("acl")
		if sf.Anonymous || (!aclTagged && fieldBaseKind(fv) == reflect.Struct && fieldBaseType(fv) != tTime) {
			collectACLVals(fv, vars)
		}

		name, ok := sf.Tag.Lookup("acl")
		if !ok {
			continue
		}
		if name == "" {
			name = sf.Name
		}
		value, ok := aclValueString(fv)
		if !ok {
			continue
		}
		vars[name] = value
	}
}

func fieldBaseKind(v reflect.Value) reflect.Kind {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return v.Kind()
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return reflect.Invalid
	}
	return v.Kind()
}

func fieldBaseType(v reflect.Value) reflect.Type {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return v.Type()
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return nil
	}
	return v.Type()
}

func aclValueString(v reflect.Value) (string, bool) {
	if !v.IsValid() {
		return "", false
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "", false
		}
		v = v.Elem()
	}
	if !v.CanInterface() {
		return "", false
	}
	return fmt.Sprint(v.Interface()), true
}

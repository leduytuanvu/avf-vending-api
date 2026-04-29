package httpserver

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var (
	rbacsDirect = regexp.MustCompile(`auth\.Require(AnyPermission|Permission|AnyRole)\(`)
	// rbac:handlers-only declares shared HTTP helpers wired only from RBAC-parent groups.
	rbacsHandlersMarker = regexp.MustCompile(`rbac:handlers-only`)
	// rbac:inherited-mount documents routes whose permission gates live in mount parents (typically server.go).
	rbacsInherited = regexp.MustCompile(`rbac:inherited-mount`)
)

// Registers every admin HTTP router source expected to declare explicit access control markers.
// Prefer route-local RequireAnyPermission when possible; use rbac:inherited-mount when server.go wraps mounts.
func TestRBAC_adminMountSourcesDeclareAccessControl(t *testing.T) {
	t.Parallel()
	root := "."
	dirEntries := []string{
		"auth_admin_http.go",
		"artifacts_http.go",
		"activation_http.go",
		"ota_admin_http.go",
		"reporting_http.go",
		"server.go",
	}
	matches, err := filepath.Glob(filepath.Join(root, "admin*_http.go"))
	if err != nil {
		t.Fatalf("glob admin*_http.go: %v", err)
	}
	for _, m := range matches {
		dirEntries = append(dirEntries, filepath.Base(m))
	}
	seen := make(map[string]struct{})
	var files []string
	for _, name := range dirEntries {
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		files = append(files, name)
	}

	for _, fname := range files {
		path := filepath.Join(root, fname)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", fname, err)
		}
		s := string(b)
		hasDirect := rbacsDirect.MatchString(s)
		hasInherited := rbacsInherited.MatchString(s)
		hasHandlersMarker := rbacsHandlersMarker.MatchString(s)
		base := filepath.Base(fname)

		if base == "admin_catalog_mutations_http.go" {
			if !(hasHandlersMarker || hasDirect) {
				t.Fatalf("%s: expected rbac:handlers-only or inline auth.Require* middleware (helpers wired from catalog mounts)", fname)
			}
			continue
		}
		if hasHandlersMarker {
			continue
		}
		if !hasDirect && !hasInherited {
			t.Fatalf("%s: expected RequireAnyPermission, RequirePermission, RequireAnyRole, or // rbac:inherited-mount", fname)
		}
	}
}

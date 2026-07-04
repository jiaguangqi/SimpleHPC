package service

import "testing"

func TestBuildNavigationFlattensLDAPUserMenus(t *testing.T) {
	permissions := map[string]struct{}{
		"menu.dashboard.view":      {},
		"menu.compute.queue.view":  {},
		"menu.data.files.view":     {},
		"menu.jobs.templates.view": {},
		"menu.jobs.list.view":      {},
		"menu.jobs.vnc.view":       {},
		"menu.terminal.view":       {},
	}
	items := BuildNavigation("ldap", permissions, DefaultMenuCatalog())
	if len(items) != 7 {
		t.Fatalf("flat menu count = %d, want 7: %#v", len(items), items)
	}
	for _, item := range items {
		if item.Type == "group" || len(item.Children) > 0 {
			t.Fatalf("LDAP navigation contains group: %#v", item)
		}
	}
}

func TestBuildNavigationKeepsAdminTree(t *testing.T) {
	permissions := map[string]struct{}{"*": {}}
	items := BuildNavigation("admin", permissions, DefaultMenuCatalog())
	hasGroup := false
	hasLogGroup := false
	hasSystemGroup := false
	wantNames := []string{"仪表盘", "账户管理", "资源管理", "数据管理", "作业管理", "终端中心", "运维管理", "日志管理", "系统配置"}
	if len(items) != len(wantNames) {
		t.Fatalf("admin top-level menu count = %d, want %d: %#v", len(items), len(wantNames), items)
	}
	for index, want := range wantNames {
		if items[index].Name != want {
			t.Fatalf("admin top-level menu[%d] = %q, want %q: %#v", index, items[index].Name, want, items)
		}
	}
	for _, item := range items {
		if item.Type == "group" && len(item.Children) > 0 {
			hasGroup = true
		}
		if item.Code == "logs" {
			hasLogGroup = item.Type == "group" && len(item.Children) == 3
		}
		if item.Code == "system" {
			hasSystemGroup = item.Type == "group" && len(item.Children) == 5
		}
	}
	if !hasGroup {
		t.Fatal("admin navigation was not grouped")
	}
	if !hasLogGroup {
		t.Fatalf("日志管理 must be a top-level group with 3 children: %#v", items)
	}
	if !hasSystemGroup {
		t.Fatalf("系统配置 must be a top-level group with 5 children: %#v", items)
	}
}

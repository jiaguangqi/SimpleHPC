package service

import "sort"

type MenuItem struct {
	Code            string     `json:"code"`
	ParentCode      string     `json:"parentCode,omitempty"`
	Name            string     `json:"name"`
	Path            string     `json:"path,omitempty"`
	Icon            string     `json:"icon,omitempty"`
	Permission      string     `json:"permission"`
	RoutePermission string     `json:"routePermission,omitempty"`
	Resource        string     `json:"resource,omitempty"`
	Type            string     `json:"type"`
	SortOrder       int        `json:"sortOrder"`
	Children        []MenuItem `json:"children,omitempty"`
}

func DefaultMenuCatalog() []MenuItem {
	return []MenuItem{
		{Code: "dashboard", Name: "仪表盘", Path: "index.html", Permission: "menu.dashboard.view", RoutePermission: "route.dashboard.view", Resource: "dashboard", Type: "page", SortOrder: 10},
		{Code: "account", Name: "账户管理", Type: "group", SortOrder: 20},
		{Code: "units", ParentCode: "account", Name: "单位管理", Path: "units.html", Permission: "menu.account.units.view", RoutePermission: "route.account.units.view", Resource: "units", Type: "page", SortOrder: 21},
		{Code: "teams", ParentCode: "account", Name: "团队管理", Path: "teams.html", Permission: "menu.account.teams.view", RoutePermission: "route.account.teams.view", Resource: "teams", Type: "page", SortOrder: 22},
		{Code: "users", ParentCode: "account", Name: "用户管理", Path: "users.html", Permission: "menu.account.users.view", RoutePermission: "route.account.users.view", Resource: "users", Type: "page", SortOrder: 23},
		{Code: "admins", ParentCode: "account", Name: "管理员账号管理", Path: "admins.html", Permission: "menu.account.admins.view", RoutePermission: "route.account.admins.view", Resource: "admins", Type: "page", SortOrder: 24},
		{Code: "roles", ParentCode: "account", Name: "角色管理", Path: "roles.html", Permission: "menu.account.roles.view", RoutePermission: "route.account.roles.view", Resource: "roles", Type: "page", SortOrder: 25},
		{Code: "compute", Name: "资源管理", Type: "group", SortOrder: 30},
		{Code: "partitions", ParentCode: "compute", Name: "资源队列配置", Path: "partitions.html", Permission: "menu.compute.partitions.view", RoutePermission: "route.compute.partitions.view", Resource: "partitions", Type: "page", SortOrder: 31},
		{Code: "queue", ParentCode: "compute", Name: "队列状态", Path: "queue-status.html", Permission: "menu.compute.queue.view", RoutePermission: "route.queue.view", Resource: "queue", Type: "page", SortOrder: 32},
		{Code: "nodes", ParentCode: "compute", Name: "节点状态", Path: "nodes.html", Permission: "menu.compute.nodes.view", RoutePermission: "route.compute.nodes.view", Resource: "nodes", Type: "page", SortOrder: 33},
		{Code: "qos", ParentCode: "compute", Name: "QOS 策略", Path: "qos.html", Permission: "menu.compute.qos.view", RoutePermission: "route.compute.qos.view", Resource: "qos", Type: "page", SortOrder: 34},
		{Code: "license_status", ParentCode: "compute", Name: "应用许可状态", Path: "license-status.html", Permission: "menu.license.status.view", RoutePermission: "route.license.status.view", Resource: "license.status", Type: "page", SortOrder: 35},
		{Code: "data", Name: "数据管理", Type: "group", SortOrder: 40},
		{Code: "files", ParentCode: "data", Name: "数据目录", Path: "data.html", Permission: "menu.data.files.view", RoutePermission: "route.data.files.view", Resource: "storage_files", Type: "page", SortOrder: 41},
		{Code: "data_acl", ParentCode: "data", Name: "访问授权", Path: "data-acl.html", Permission: "menu.data.acl.view", RoutePermission: "route.data.acl.view", Resource: "storage_acl", Type: "page", SortOrder: 42},
		{Code: "projects", Name: "项目中心", Type: "group", SortOrder: 45},
		{Code: "projects_overview", ParentCode: "projects", Name: "项目总览", Path: "projects.html", Permission: "menu.projects.overview.view", RoutePermission: "route.projects.overview.view", Resource: "projects", Type: "page", SortOrder: 46},
		{Code: "jobs", Name: "作业管理", Type: "group", SortOrder: 50},
		{Code: "templates", ParentCode: "jobs", Name: "作业模板", Path: "job-templates.html", Permission: "menu.jobs.templates.view", RoutePermission: "route.jobs.templates.view", Resource: "job_templates", Type: "page", SortOrder: 51},
		{Code: "job_list", ParentCode: "jobs", Name: "作业列表", Path: "job-list.html", Permission: "menu.jobs.list.view", RoutePermission: "route.jobs.list.view", Resource: "jobs", Type: "page", SortOrder: 52},
		{Code: "vnc", ParentCode: "jobs", Name: "VNC 桌面", Path: "vnc-desktop.html", Permission: "menu.jobs.vnc.view", RoutePermission: "route.jobs.vnc.view", Resource: "vnc_sessions", Type: "page", SortOrder: 53},
		{Code: "terminal", Name: "终端中心", Path: "terminal.html", Permission: "menu.terminal.view", RoutePermission: "route.terminal.view", Resource: "terminal", Type: "page", SortOrder: 55},
		{Code: "operations", Name: "运维管理", Type: "group", SortOrder: 60},
		{Code: "monitoring", ParentCode: "operations", Name: "监控告警", Path: "monitoring.html", Permission: "menu.operations.monitoring.view", RoutePermission: "route.operations.monitoring.view", Resource: "monitoring", Type: "page", SortOrder: 61},
		{Code: "inspection", ParentCode: "operations", Name: "巡检报告", Path: "inspection.html", Permission: "menu.operations.inspection.view", RoutePermission: "route.operations.inspection.view", Resource: "inspection_reports", Type: "page", SortOrder: 62},
		{Code: "logs", Name: "日志管理", Type: "group", SortOrder: 70},
		{Code: "login_logs", ParentCode: "logs", Name: "用户登录日志", Path: "login-logs.html", Permission: "menu.logs.view", RoutePermission: "route.logs.view", Resource: "logs", Type: "page", SortOrder: 71},
		{Code: "system_logs", ParentCode: "logs", Name: "系统日志", Path: "system-logs.html", Permission: "menu.logs.view", RoutePermission: "route.logs.view", Resource: "logs", Type: "page", SortOrder: 72},
		{Code: "audit_logs", ParentCode: "logs", Name: "审计日志", Path: "audit.html", Permission: "menu.logs.view", RoutePermission: "route.logs.view", Resource: "logs", Type: "page", SortOrder: 73},
		{Code: "system", Name: "系统配置", Type: "group", SortOrder: 80},
		{Code: "settings", ParentCode: "system", Name: "平台设置", Path: "settings.html", Permission: "menu.system.platform.view", RoutePermission: "route.system.platform.view", Resource: "config.platform", Type: "page", SortOrder: 81},
		{Code: "ldap_config", ParentCode: "system", Name: "LDAP 配置", Path: "ldap.html", Permission: "menu.system.ldap.view", RoutePermission: "route.system.ldap.view", Resource: "config.ldap", Type: "page", SortOrder: 82},
		{Code: "slurm_config", ParentCode: "system", Name: "Slurm 配置", Path: "slurm.html", Permission: "menu.system.slurm.view", RoutePermission: "route.system.slurm.view", Resource: "config.slurm", Type: "page", SortOrder: 83},
		{Code: "storage_config", ParentCode: "system", Name: "存储配置", Path: "storage.html", Permission: "menu.system.storage.view", RoutePermission: "route.system.storage.view", Resource: "storage.roots", Type: "page", SortOrder: 84},
		{Code: "notify_config", ParentCode: "system", Name: "通知配置", Path: "notify.html", Permission: "menu.system.notify.view", RoutePermission: "route.system.notify.view", Resource: "config.notify", Type: "page", SortOrder: 85},
		{Code: "license_config", ParentCode: "system", Name: "应用许可配置", Path: "license-config.html", Permission: "menu.license.config.view", RoutePermission: "route.license.config.view", Resource: "license.config", Type: "page", SortOrder: 86},
	}
}

func BuildNavigation(accountType string, permissions map[string]struct{}, catalog []MenuItem) []MenuItem {
	can := func(key string) bool {
		if _, ok := permissions["*"]; ok {
			return true
		}
		_, ok := permissions[key]
		return ok
	}
	pages := make([]MenuItem, 0)
	for _, item := range catalog {
		if item.Type == "page" && can(item.Permission) {
			pages = append(pages, item)
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].SortOrder < pages[j].SortOrder })
	if accountType != "admin" {
		for index := range pages {
			pages[index].ParentCode = ""
		}
		return pages
	}
	groups := map[string]MenuItem{}
	var roots []MenuItem
	for _, item := range catalog {
		if item.Type == "group" {
			item.Children = nil
			groups[item.Code] = item
		}
	}
	for _, page := range pages {
		if group, ok := groups[page.ParentCode]; ok {
			group.Children = append(group.Children, page)
			groups[page.ParentCode] = group
		} else {
			roots = append(roots, page)
		}
	}
	for _, group := range groups {
		if len(group.Children) > 0 {
			roots = append(roots, group)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].SortOrder < roots[j].SortOrder })
	return roots
}

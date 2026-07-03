package service

import (
	"context"
	"database/sql"
	"strconv"
)

type ResourceIdentity struct {
	Owner   string
	UnitID  string
	TeamID  string
	Granted bool
}

func (p PermissionContext) Allows(resource string, item ResourceIdentity) bool {
	if p.IsClusterAdmin || p.HasScope(resource, ScopeGlobal) {
		return true
	}
	if p.HasScope(resource, ScopeUnit) && containsString(p.UnitIDs, item.UnitID) {
		return true
	}
	if p.HasScope(resource, ScopeTeam) && containsString(p.TeamIDs, item.TeamID) {
		return true
	}
	if p.HasScope(resource, ScopeSelf) && item.Owner != "" && item.Owner == p.Username {
		return true
	}
	return p.HasScope(resource, ScopeGranted) && item.Granted
}

func containsString(values []string, value string) bool {
	if value == "" {
		return false
	}
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

func (s *Services) UserResourceIdentity(ctx context.Context, username string) (ResourceIdentity, error) {
	var unitID, teamID sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `
SELECT unit_id,team_id FROM platform_users WHERE username=$1`, username).Scan(&unitID, &teamID)
	item := ResourceIdentity{Owner: username}
	if unitID.Valid {
		item.UnitID = strconv.FormatInt(unitID.Int64, 10)
	}
	if teamID.Valid {
		item.TeamID = strconv.FormatInt(teamID.Int64, 10)
	}
	return item, err
}

func (s *Services) FilterAccountUsers(ctx context.Context, authz PermissionContext, items []AccountUser) ([]AccountUser, error) {
	if authz.IsClusterAdmin || authz.HasScope("users", ScopeGlobal) {
		return items, nil
	}
	result := make([]AccountUser, 0, len(items))
	for _, item := range items {
		identity, err := s.UserResourceIdentity(ctx, item.Username)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if authz.Allows("users", identity) {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *Services) FilterAccountTeams(ctx context.Context, authz PermissionContext, items []AccountTeam) ([]AccountTeam, error) {
	if authz.IsClusterAdmin || authz.HasScope("teams", ScopeGlobal) {
		return items, nil
	}
	result := make([]AccountTeam, 0, len(items))
	for _, item := range items {
		var id int64
		var unitID sql.NullInt64
		err := s.DB.QueryRowContext(ctx, `
SELECT id,unit_id FROM teams WHERE name=$1 OR group_name=$2 LIMIT 1`,
			item.Name, item.GroupName).Scan(&id, &unitID)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		identity := ResourceIdentity{TeamID: strconv.FormatInt(id, 10)}
		if unitID.Valid {
			identity.UnitID = strconv.FormatInt(unitID.Int64, 10)
		}
		if authz.Allows("teams", identity) {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *Services) FilterAccountUnits(ctx context.Context, authz PermissionContext, items []AccountUnit) ([]AccountUnit, error) {
	if authz.IsClusterAdmin || authz.HasScope("units", ScopeGlobal) {
		return items, nil
	}
	result := make([]AccountUnit, 0, len(items))
	for _, item := range items {
		var id int64
		err := s.DB.QueryRowContext(ctx, `
SELECT id FROM units WHERE code=$1 OR name=$2 LIMIT 1`, item.Code, item.Name).Scan(&id)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if authz.Allows("units", ResourceIdentity{UnitID: strconv.FormatInt(id, 10)}) {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *Services) FilterJobTemplatesByScope(ctx context.Context, authz PermissionContext, items []JobTemplate) ([]JobTemplate, error) {
	if authz.IsClusterAdmin || authz.HasScope("job_templates", ScopeGlobal) {
		return items, nil
	}
	result := make([]JobTemplate, 0, len(items))
	for _, item := range items {
		identity, err := s.UserResourceIdentity(ctx, item.CreatedBy)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		identity.Granted = item.Authorized
		if authz.Allows("job_templates", identity) {
			result = append(result, item)
		}
	}
	return result, nil
}

func ScopeJobQueryByPermission(authz PermissionContext, query JobQuery) JobQuery {
	if authz.IsClusterAdmin || authz.HasScope("jobs", ScopeGlobal) {
		return query
	}
	query.Group = ""
	if authz.HasScope("jobs", ScopeUnit) && len(authz.UnitIDs) > 0 {
		query.Username = ""
		query.UnitIDs = append([]string(nil), authz.UnitIDs...)
		return query
	}
	if authz.HasScope("jobs", ScopeTeam) && len(authz.TeamIDs) > 0 {
		query.Username = ""
		query.TeamIDs = append([]string(nil), authz.TeamIDs...)
		return query
	}
	if authz.HasScope("jobs", ScopeSelf) {
		query.Username = authz.Username
		return query
	}
	query.Username = ""
	query.DenyAll = true
	return query
}

DELETE FROM role_permissions
WHERE created_by='migration-015-project-center';

DELETE FROM permissions
WHERE permission_key IN (
  'menu.projects.overview.view','route.projects.overview.view',
  'api.projects.list','api.projects.view','api.projects.create',
  'api.projects.update','api.projects.delete'
);

DELETE FROM menus WHERE code IN ('projects_overview','projects');

DROP TABLE IF EXISTS project_activity_logs;
DROP TABLE IF EXISTS project_job_links;
DROP TABLE IF EXISTS project_directories;
DROP TABLE IF EXISTS project_tasks;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;

DELETE FROM permissions WHERE permission_key IN (
  'route.account.units.view',
  'route.account.teams.view',
  'route.account.users.view',
  'route.account.admins.view',
  'route.account.roles.view',
  'route.compute.partitions.view',
  'route.compute.nodes.view',
  'route.compute.qos.view',
  'route.data.acl.view',
  'route.operations.monitoring.view',
  'route.operations.inspection.view',
  'route.logs.view'
);

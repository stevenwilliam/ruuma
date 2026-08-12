DELETE FROM sys_parameters WHERE key IN (
  'company.backdrop_enabled',
  'company.backdrop_file'
);

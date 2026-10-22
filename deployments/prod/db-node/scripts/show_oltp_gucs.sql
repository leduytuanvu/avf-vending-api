SELECT name, setting
FROM pg_settings
WHERE name IN (
  'shared_buffers',
  'random_page_cost',
  'jit',
  'effective_cache_size',
  'shared_preload_libraries',
  'max_connections'
)
ORDER BY 1;

-- Intentionally a no-op: jobscrapper also creates and owns data in this
-- table at runtime, so rolling back the jobhunter migration must not drop it.
SELECT 1;

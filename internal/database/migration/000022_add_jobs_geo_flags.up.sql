ALTER TABLE jobs ADD COLUMN IF NOT EXISTS geo_restriction TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS geo_sponsorship BOOLEAN;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS geo_detail TEXT;

COMMENT ON COLUMN jobs.geo_restriction IS 'Detected location/work-auth restriction label (e.g. us-only, canada-only, onsite); NULL = unknown or unrestricted';
COMMENT ON COLUMN jobs.geo_sponsorship IS 'true = JD offers visa sponsorship, false = JD explicitly refuses it, NULL = not stated';
COMMENT ON COLUMN jobs.geo_detail IS 'Raw JD phrase the restriction was detected from';

ALTER TABLE services ADD COLUMN origin_kind TEXT NOT NULL DEFAULT ''
  CHECK (origin_kind IN ('', 'host', 'container', 'ip', 'hostname'));
ALTER TABLE services ADD COLUMN origin_value TEXT NOT NULL DEFAULT '';
ALTER TABLE services ADD COLUMN origin_network TEXT NOT NULL DEFAULT '';

UPDATE services SET origin_kind = 'host', origin_value = ''
 WHERE origin_kind = '' AND (trim(origin) = '' OR trim(origin) = 'host');

UPDATE services SET origin_kind = 'ip', origin_value = trim(origin)
 WHERE origin_kind = ''
   AND (trim(origin) GLOB '[0-9]*.[0-9]*.[0-9]*.[0-9]*'
        OR trim(origin) LIKE '%:%');

UPDATE services SET origin_kind = 'hostname', origin_value = trim(origin)
 WHERE origin_kind = '' AND trim(origin) LIKE '%.%';

UPDATE services SET origin_kind = 'container', origin_value = trim(origin)
 WHERE origin_kind = '';

ALTER TABLE services DROP COLUMN origin;

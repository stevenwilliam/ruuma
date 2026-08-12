-- 0017 backdrop: the customer site's background photograph (BR-1.4.6).
--
-- The filename must keep the word "parameters" — db/embed.go DataMigrations()
-- selects reference-data migrations by that substring and test/testenv replays
-- exactly those after it TRUNCATEs. See 0016.
--
-- backdrop_file is a FILENAME, not a URL, and the API refuses anything that is
-- not a plain name with a safe extension. Two reasons:
--
--   1. The value ends up inside a CSS url(). A parameter that could contain
--      quotes, parentheses or a semicolon would let anyone with parameter
--      permission inject styles into every customer's page.
--   2. Content-Security-Policy is `img-src 'self' data: blob:`. An external
--      URL would simply be blocked, so offering one would be a setting that
--      silently does nothing.
--
-- Files live in web/public/brand/. Adding a NEW file still needs a deploy —
-- serving admin uploads from object storage is an open gap (docs/PROGRESS.md).
-- What this parameter buys today is switching between shipped images and
-- turning the photograph off, without a code change.

INSERT INTO sys_parameters (id, key, value, data_type, description, is_store_overridable) VALUES
  (uuidv7(),'company.backdrop_enabled','true','bool',
   'Show the background photograph on the customer site',false),
  (uuidv7(),'company.backdrop_file','backdrop.jpg','string',
   'Background image filename under /brand/ — letters, digits, dash, underscore, dot; .jpg .jpeg .png .webp only',false)
ON CONFLICT (key) DO NOTHING;

-- 0015 make the rate limits configurable, as docs/04 §9 always said they were.
--
-- They are read at boot, not per request: a limiter that reloaded mid-window
-- would let a burst through on the change. The descriptions say so.
INSERT INTO sys_parameters (id, key, value, data_type, description) VALUES
  (uuidv7(),'ratelimit.login_per_minute','5','int','Login attempts per email and per IP, per minute (applies on restart)'),
  (uuidv7(),'ratelimit.staff_login_per_minute','5','int','Staff login attempts per email and per IP, per minute (applies on restart)'),
  (uuidv7(),'ratelimit.otp_request_per_10min','3','int','OTP requests per phone and per IP, per 10 minutes (applies on restart)'),
  (uuidv7(),'ratelimit.otp_verify_per_10min','5','int','OTP verifications per phone and per IP, per 10 minutes (applies on restart)'),
  (uuidv7(),'ratelimit.tracking_per_minute','20','int','Order tracking lookups per customer and per IP, per minute (applies on restart)'),
  (uuidv7(),'ratelimit.order_create_per_minute','10','int','Order creations per customer and per IP, per minute (applies on restart)'),
  (uuidv7(),'ratelimit.menu_read_per_minute','120','int','Menu reads per IP, per minute (applies on restart)')
ON CONFLICT (key) DO NOTHING;

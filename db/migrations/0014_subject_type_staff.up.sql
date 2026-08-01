-- 0014 align the auth subject vocabulary with the code.
--
-- The service distinguishes a customer account from a staff account as
-- 'customer' | 'staff' (security.SubjectType); the original CHECK spelled the
-- staff side 'user' after the table name. Accept both so existing rows stay
-- valid, with 'staff' as the value written from now on.
ALTER TABLE refresh_tokens DROP CONSTRAINT IF EXISTS refresh_tokens_subject_type_check;
ALTER TABLE refresh_tokens ADD CONSTRAINT refresh_tokens_subject_type_check
  CHECK (subject_type IN ('customer','staff','user'));

ALTER TABLE verification_tokens DROP CONSTRAINT IF EXISTS verification_tokens_subject_type_check;
ALTER TABLE verification_tokens ADD CONSTRAINT verification_tokens_subject_type_check
  CHECK (subject_type IN ('customer','staff','user'));

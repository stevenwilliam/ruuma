UPDATE refresh_tokens SET subject_type = 'user' WHERE subject_type = 'staff';
UPDATE verification_tokens SET subject_type = 'user' WHERE subject_type = 'staff';

ALTER TABLE refresh_tokens DROP CONSTRAINT IF EXISTS refresh_tokens_subject_type_check;
ALTER TABLE refresh_tokens ADD CONSTRAINT refresh_tokens_subject_type_check
  CHECK (subject_type IN ('customer','user'));

ALTER TABLE verification_tokens DROP CONSTRAINT IF EXISTS verification_tokens_subject_type_check;
ALTER TABLE verification_tokens ADD CONSTRAINT verification_tokens_subject_type_check
  CHECK (subject_type IN ('customer','user'));

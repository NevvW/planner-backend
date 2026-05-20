ALTER TABLE users ADD COLUMN login text;
UPDATE users SET login = split_part(email::text,'@',1) || '_' || substr(id::text,1,8) WHERE login IS NULL;
ALTER TABLE users ALTER COLUMN login SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_login_key UNIQUE (login);

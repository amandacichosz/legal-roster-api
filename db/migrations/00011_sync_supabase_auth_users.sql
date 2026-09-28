-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.sync_auth_user_profile()
RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
	INSERT INTO public.users (id, email, name, role, status)
	VALUES (
		NEW.id,
		LOWER(COALESCE(NEW.email, '')),
		COALESCE(NEW.raw_user_meta_data->>'name', NEW.raw_user_meta_data->>'full_name', ''),
		'admin',
		'disabled'
	)
	ON CONFLICT (id) DO UPDATE
	SET email = EXCLUDED.email,
		name = CASE WHEN public.users.name = '' THEN EXCLUDED.name ELSE public.users.name END,
		updated_at = NOW();
	RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER sync_auth_user_profile_after_insert
AFTER INSERT ON auth.users
FOR EACH ROW EXECUTE FUNCTION public.sync_auth_user_profile();

INSERT INTO public.users (id, email, name, role, status)
SELECT
	id,
	LOWER(COALESCE(email, '')),
	COALESCE(raw_user_meta_data->>'name', raw_user_meta_data->>'full_name', ''),
	'admin',
	'disabled'
FROM auth.users
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TRIGGER IF EXISTS sync_auth_user_profile_after_insert ON auth.users;
DROP FUNCTION IF EXISTS public.sync_auth_user_profile();

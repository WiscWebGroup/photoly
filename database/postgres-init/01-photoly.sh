#!/bin/sh
set -eu
# psql quotes password values; they never become shell or SQL code.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
\getenv app_password POSTGRES_APP_PASSWORD
\getenv migration_password POSTGRES_MIGRATION_PASSWORD
SELECT format('CREATE ROLE photoly LOGIN PASSWORD %L', :'app_password') \gexec
SELECT format('CREATE ROLE photoly_migrator LOGIN PASSWORD %L', :'migration_password') \gexec
SQL
if [ -n "${POSTGRES_EXTRA_DATABASE:-}" ]; then
    createdb --username "$POSTGRES_USER" "$POSTGRES_EXTRA_DATABASE"
fi
for database in "$POSTGRES_DB" ${POSTGRES_EXTRA_DATABASE:-}; do
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$database" <<'SQL'
CREATE EXTENSION vector VERSION '0.8.7';
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON DATABASE :"DBNAME" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"DBNAME" TO photoly, photoly_migrator;
GRANT USAGE ON SCHEMA public TO photoly, photoly_migrator;
GRANT CREATE ON SCHEMA public TO photoly_migrator;
ALTER DEFAULT PRIVILEGES FOR ROLE photoly_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO photoly;
ALTER DEFAULT PRIVILEGES FOR ROLE photoly_migrator IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO photoly;
SQL
done

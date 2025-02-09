#!/bin/bash

set -e

echo "Waiting for PostgreSQL to be ready..."
for i in {1..10}; do
    if psql -U validator -h localhost -p 5432 -d project-sem-1 -c "\\q" &> /dev/null; then
        echo "PostgreSQL is ready!"
        break
    fi
    echo "PostgreSQL is not ready. sleep ($i/10)"
    sleep 2
done

if ! psql -U validator -h localhost -p 5432 -d project-sem-1 -c "\\q" &> /dev/null; then
    echo "Database project-sem-1 is not accessible."

    echo "Trying to connect as postgres"
    if ! psql -U postgres -h localhost -p 5432 -c "\\q" &> /dev/null; then
        echo "Error: Could not connect to PostgreSQL as postgres."
        exit 1
    fi

    echo "Creating user and db"
    psql -U postgres -h localhost -p 5432 <<EOF
    DO \$\$ BEGIN
      IF NOT EXISTS (SELECT FROM pg_catalog.pg_user WHERE usename = 'validator') THEN
        CREATE USER validator WITH PASSWORD 'val1dat0r';
      END IF;
    END \$\$;

    DO \$\$ BEGIN
      IF NOT EXISTS (SELECT FROM pg_database WHERE datname = 'project-sem-1') THEN
        CREATE DATABASE "project-sem-1" OWNER validator;
      END IF;
    END \$\$;

    GRANT ALL PRIVILEGES ON DATABASE "project-sem-1" TO validator;
EOF
else
    echo "Database project-sem-1 is accessible. No changes required."
fi

echo "Creating the table prices"
psql -U validator -h localhost -p 5432 -d project-sem-1 <<EOF
CREATE TABLE IF NOT EXISTS prices (
    product_id SERIAL PRIMARY KEY,
    id INT NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    created_at DATE NOT NULL
);
EOF

echo "Everything setted up"

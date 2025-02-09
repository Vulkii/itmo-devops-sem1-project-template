#!/bin/bash

set -e

echo "Installing postgresql and unzip"
sudo apt update
sudo apt install -y postgresql postgresql-client unzip golang

echo "Starting PostgreSQL"
sudo systemctl start postgresql
sudo systemctl enable postgresql

echo "Waiting for PostgreSQL to start"
for i in {1..10}; do
    if sudo -u postgres pg_isready -q; then
        echo "PostgreSQL is ready"
        break
    fi
    echo "PostgreSQL is not ready yet. sleep 2 sec. ($i/10)"
    sleep 2
done

echo "Creating DB and user"
sudo -u postgres psql <<EOF
CREATE DATABASE "project-sem-1";
CREATE USER validator WITH ENCRYPTED PASSWORD 'val1dat0r';
GRANT ALL PRIVILEGES ON DATABASE "project-sem-1" TO validator;
EOF

echo "Creating the table"
sudo -u postgres psql -d project-sem-1 <<EOF
CREATE TABLE IF NOT EXISTS prices (
    id SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL,
    created_at DATE NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    price INTEGER NOT NULL
);
EOF

echo "Everything setted up"

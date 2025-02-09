#!/bin/bash

set -e

echo "Installing postgresql and unzip"
sudo apt update
sudo apt install -y postgresql postgresql-client unzip

echo "Starting PostgreSQL"
sudo systemctl start postgresql
sudo systemctl enable postgresql

echo "Waiting PostgreSQL to start"
sleep 5

echo "Creating DB and user"
sudo -u postgres psql <<EOF
CREATE DATABASE project-sem-1;
CREATE USER validator WITH ENCRYPTED PASSWORD 'val1dat0r';
GRANT ALL PRIVILEGES ON DATABASE project-sem-1 TO validator;
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

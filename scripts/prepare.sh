#!/bin/bash

set -e

echo "Installing postgresql and unzip"
sudo apt update
sudo apt install -y unzip golang

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

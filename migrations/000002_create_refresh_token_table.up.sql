CREATE EXTENSION IF NOT EXISTS postgres_fdw;

-- Local users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(50) UNIQUE NOT NULL,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    password TEXT NOT NULL,
    image TEXT DEFAULT 'image',
    role role NOT NULL DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

-- Server1
CREATE SERVER server2_fdw FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host '3.120.39.160', port '5432', dbname 'google_docs');
CREATE USER MAPPING FOR postgres SERVER server2_fdw OPTIONS (user 'postgres', password '1234');

CREATE FOREIGN TABLE users_server2 (
    id UUID DEFAULT gen_random_uuid(),
    email VARCHAR(50) NOT NULL,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    password TEXT NOT NULL,
    image TEXT DEFAULT 'image',
    role role NOT NULL DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
) SERVER server1_fdw OPTIONS (schema_name 'public', table_name 'users');

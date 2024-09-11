#!/bin/bash
set -e

# Wait for the master to be ready
until pg_isready -h postgres-master -p 5432 -U postgres; do
  echo "Waiting for master..."
  sleep 1
done

# Perform the base backup from master
pg_basebackup -h postgres-master -D /var/lib/postgresql/data -U postgres -v -P --wal-method=stream

# Start PostgreSQL in standby mode
exec postgres -c hot_standby=on

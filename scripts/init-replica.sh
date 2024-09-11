#!/bin/bash

# Wait for the master to be available
until pg_isready -h postgres-master -p 5432 -U postgres; do
  echo "Waiting for master..."
  sleep 1
done

# Perform base backup from the master
pg_basebackup -h postgres-master -D /var/lib/postgresql/data -U postgres -v -P --wal-method=stream

# Start the PostgreSQL server in hot standby mode
exec postgres -c hot_standby=on

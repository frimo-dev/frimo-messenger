#!/usr/bin/env bash
set -euo pipefail

: "${SCYLLA_PASSWORD:?SCYLLA_PASSWORD is required}"
: "${SCYLLA_KEYSPACE:?SCYLLA_KEYSPACE is required}"

# Используем hex-пароль, чтобы безопасно подставлять
# его в CQL при первоначальной настройке.
if [[ ! "$SCYLLA_PASSWORD" =~ ^[0-9a-fA-F]{48}$ ]]; then
    echo "SCYLLA_PASSWORD must be a 48-character hex string"
    exit 1
fi

# Проверяем название keyspace перед подстановкой в CQL.
if [[ ! "$SCYLLA_KEYSPACE" =~ ^[a-z][a-z0-9_]*$ ]]; then
    echo "Invalid SCYLLA_KEYSPACE"
    exit 1
fi

check_password() {
    cqlsh scylla 9042 \
        -u cassandra \
        -p "$1" \
        -e "SELECT cluster_name FROM system.local;" \
        >/dev/null 2>&1
}

echo "Waiting for ScyllaDB authentication..."

authenticated=false

for attempt in $(seq 1 90); do

    # Проверяем, установлен ли наш пароль.
    if check_password "$SCYLLA_PASSWORD"; then
        authenticated=true
        break
    fi

    # Первоначальная настройка ScyllaDB 6.2.
    if check_password "cassandra"; then
        echo "Configuring ScyllaDB password..."

        cqlsh scylla 9042 \
            -u cassandra \
            -p cassandra \
            -e "ALTER ROLE cassandra WITH PASSWORD = '$SCYLLA_PASSWORD';"
    fi

    sleep 2
done

if [[ "$authenticated" != true ]]; then
    echo "Failed to authenticate to ScyllaDB"
    exit 1
fi

echo "ScyllaDB authentication configured"

# Создаём keyspace, если его ещё нет.
echo "Ensuring keyspace '$SCYLLA_KEYSPACE' exists..."

cqlsh scylla 9042 \
    -u cassandra \
    -p "$SCYLLA_PASSWORD" \
    -e "CREATE KEYSPACE IF NOT EXISTS ${SCYLLA_KEYSPACE} WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1} AND durable_writes = true;"

echo "ScyllaDB initialization completed successfully"
#!/bin/sh
set -e

echo "Waiting for Kafka to be ready..."
sleep 10

BROKER="kafka:9092"

echo "Creating topics..."
kafka-topics --create --if-not-exists \
    --bootstrap-server "$BROKER" \
    --topic user.created \
    --partitions 1 \
    --replication-factor 1

kafka-topics --create --if-not-exists \
    --bootstrap-server "$BROKER" \
    --topic ticket.purchased \
    --partitions 1 \
    --replication-factor 1

echo "Topics created successfully."
kafka-topics --list --bootstrap-server "$BROKER"

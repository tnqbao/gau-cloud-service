#!/usr/bin/env bash
# This script loops through each file in the template folder,
# applies envsubst, and writes the output to the base folder.

TEMPLATE_DIR="template"
OUTPUT_DIR="base"

# Chỉ thay thế các biến env cụ thể, không thay thế các biến shell script
ENV_VARS='${DEPLOY_ENV} ${S3_API_DOMAIN} ${S3_WEB_DOMAIN} ${GARAGE_RPC_SECRET} ${GARAGE_ADMIN_TOKEN} ${GARAGE_ROOT_USER} ${GARAGE_ROOT_PASSWORD} ${DASHBOARD_DOMAIN}'

# Create the output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

for file in "$TEMPLATE_DIR"/*; do
    filename=$(basename "$file")
    envsubst "$ENV_VARS" < "$file" > "$OUTPUT_DIR/$filename"
    echo "Processed $file -> $OUTPUT_DIR/$filename"
done

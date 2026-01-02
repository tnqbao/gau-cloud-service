#!/bin/bash

# ===========================
# Chunked Upload Test Script
# Server-decided chunk size architecture
# ===========================

# Configuration - User can modify these
API_BASE_URL="${API_BASE_URL:-http://localhost:8080/api/v1/cloud}"
BUCKET_ID="${BUCKET_ID:-your-bucket-id-here}"
AUTH_TOKEN="${AUTH_TOKEN:-your-auth-token-here}"
CUSTOM_PATH="${1:-}"  # Optional: custom path in bucket (e.g., "folder/subfolder")
shift || true  # Remove first argument (custom_path) from $@

# Remaining arguments are file paths
FILE_PATHS=("$@")

# Default to example file if no files provided
if [ ${#FILE_PATHS[@]} -eq 0 ]; then
    FILE_PATHS=("./large-file.zip")
fi

# Constants
PREFERRED_CHUNK_SIZE=$((10 * 1024 * 1024))  # 10MB preferred (server decides final)
COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_YELLOW='\033[1;33m'
COLOR_RED='\033[0;31m'
COLOR_CYAN='\033[0;36m'
COLOR_RESET='\033[0m'

# ===========================
# Helper Functions
# ===========================

log_info() {
    echo -e "${COLOR_BLUE}[INFO]${COLOR_RESET} $1"
}

log_success() {
    echo -e "${COLOR_GREEN}[SUCCESS]${COLOR_RESET} $1"
}

log_warning() {
    echo -e "${COLOR_YELLOW}[WARNING]${COLOR_RESET} $1"
}

log_error() {
    echo -e "${COLOR_RED}[ERROR]${COLOR_RESET} $1"
}

log_progress() {
    echo -e "${COLOR_CYAN}[→]${COLOR_RESET} $1"
}

format_bytes() {
    local bytes=$1
    if [ $bytes -lt 1024 ]; then
        echo "${bytes}B"
    elif [ $bytes -lt $((1024 * 1024)) ]; then
        printf "%.2fKB" $(echo "scale=2; $bytes / 1024" | bc)
    elif [ $bytes -lt $((1024 * 1024 * 1024)) ]; then
        printf "%.2fMB" $(echo "scale=2; $bytes / 1024 / 1024" | bc)
    else
        printf "%.2fGB" $(echo "scale=2; $bytes / 1024 / 1024 / 1024" | bc)
    fi
}

# Progress bar function
show_progress() {
    local current=$1
    local total=$2
    local width=50
    local percentage=$((current * 100 / total))
    local completed=$((width * current / total))
    local remaining=$((width - completed))

    printf "\r${COLOR_CYAN}["
    printf "%${completed}s" | tr ' ' '█'
    printf "%${remaining}s" | tr ' ' '░'
    printf "] %3d%% (%d/%d)${COLOR_RESET}" $percentage $current $total
}

print_banner() {
    echo -e "${COLOR_CYAN}"
    echo "╔════════════════════════════════════════════════════════╗"
    echo "║       Production-Grade Chunked Upload Tool            ║"
    echo "║       Server-Decided Chunk Size Architecture          ║"
    echo "╚════════════════════════════════════════════════════════╝"
    echo -e "${COLOR_RESET}"
    echo ""
}

# ===========================
# Validation
# ===========================

print_banner

if [ ${#FILE_PATHS[@]} -eq 0 ]; then
    log_error "No files provided!"
    echo ""
    echo "Usage: $0 [custom_path] <file1> [file2] [file3]"
    echo ""
    echo "Examples:"
    echo "  $0 \"\" /c/Downloads/file1.exe /c/Downloads/file2.zip"
    echo "  $0 \"installers\" /c/Downloads/OBS-Studio.exe"
    echo "  $0 \"videos/2024\" video1.mp4 video2.mp4 video3.mp4"
    echo ""
    exit 1
fi

if [ "$BUCKET_ID" = "your-bucket-id-here" ]; then
    log_error "BUCKET_ID not set!"
    echo "Set via: export BUCKET_ID=\"your-bucket-uuid\""
    exit 1
fi

if [ "$AUTH_TOKEN" = "your-auth-token-here" ]; then
    log_error "AUTH_TOKEN not set!"
    echo "Set via: export AUTH_TOKEN=\"your-jwt-token\""
    exit 1
fi

log_info "Preparing to upload ${#FILE_PATHS[@]} file(s)"
[ -n "$CUSTOM_PATH" ] && log_info "Custom path: $CUSTOM_PATH"
echo ""

# ===========================
# Upload Function
# ===========================

upload_file() {
    local FILE_PATH="$1"
    local FILE_INDEX="$2"
    local TOTAL_FILES="$3"

    echo -e "${COLOR_CYAN}════════════════════════════════════════════════════════${COLOR_RESET}"
    log_info "[$FILE_INDEX/$TOTAL_FILES] Processing: $FILE_PATH"
    echo -e "${COLOR_CYAN}════════════════════════════════════════════════════════${COLOR_RESET}"

    # Check if file exists
    if [ ! -f "$FILE_PATH" ]; then
        log_error "File not found: $FILE_PATH"
        return 1
    fi

    # Get file info
    FILE_NAME=$(basename "$FILE_PATH")
    FILE_SIZE=$(stat -f%z "$FILE_PATH" 2>/dev/null || stat -c%s "$FILE_PATH" 2>/dev/null)
    CONTENT_TYPE=$(file -b --mime-type "$FILE_PATH" 2>/dev/null || echo "application/octet-stream")

    log_info "File: $FILE_NAME"
    log_info "Size: $(format_bytes $FILE_SIZE) ($FILE_SIZE bytes)"
    log_info "Type: $CONTENT_TYPE"
    echo ""

    # ===========================
    # Step 1: Initialize Upload
    # ===========================

    log_info "Step 1: Initializing upload session..."

    INIT_PAYLOAD=$(cat <<EOF
{
  "file_name": "$FILE_NAME",
  "file_size": $FILE_SIZE,
  "content_type": "$CONTENT_TYPE",
  "path": "$CUSTOM_PATH",
  "preferred_chunk_size": $PREFERRED_CHUNK_SIZE
}
EOF
)

    INIT_RESPONSE=$(curl -s -X POST \
      "$API_BASE_URL/buckets/$BUCKET_ID/chunked/init" \
      -H "Authorization: Bearer $AUTH_TOKEN" \
      -H "Content-Type: application/json" \
      -d "$INIT_PAYLOAD")

    UPLOAD_ID=$(echo "$INIT_RESPONSE" | grep -o '"upload_id":"[^"]*' | cut -d'"' -f4)
    SERVER_CHUNK_SIZE=$(echo "$INIT_RESPONSE" | grep -o '"chunk_size":[0-9]*' | cut -d':' -f2)
    TOTAL_CHUNKS=$(echo "$INIT_RESPONSE" | grep -o '"total_chunks":[0-9]*' | cut -d':' -f2)

    if [ -z "$UPLOAD_ID" ] || [ -z "$SERVER_CHUNK_SIZE" ]; then
        log_error "Failed to initialize upload"
        echo "$INIT_RESPONSE"
        return 1
    fi

    log_success "Upload initialized"
    log_info "Upload ID: ${UPLOAD_ID:0:8}...${UPLOAD_ID: -4}"
    log_info "Server chunk size: $(format_bytes $SERVER_CHUNK_SIZE) (SERVER DECIDED)"
    log_info "Total chunks: $TOTAL_CHUNKS"

    if [ "$SERVER_CHUNK_SIZE" -ne "$PREFERRED_CHUNK_SIZE" ]; then
        log_warning "Server overrode preferred chunk size: $(format_bytes $PREFERRED_CHUNK_SIZE) → $(format_bytes $SERVER_CHUNK_SIZE)"
    fi
    echo ""

    # ===========================
    # Step 2: Upload Chunks
    # ===========================

    log_info "Step 2: Uploading chunks..."

    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    CHUNK_INDEX=0
    BYTES_UPLOADED=0
    START_TIME=$(date +%s)

    while [ $BYTES_UPLOADED -lt $FILE_SIZE ]; do
        CHUNK_FILE="$TEMP_DIR/chunk_$CHUNK_INDEX.part"

        OFFSET=$((CHUNK_INDEX * SERVER_CHUNK_SIZE))

        # Use server-decided chunk size
        dd if="$FILE_PATH" of="$CHUNK_FILE" \
           bs=1 \
           skip=$OFFSET \
           count=$SERVER_CHUNK_SIZE \
           status=none 2>/dev/null

        CHUNK_FILE_SIZE=$(stat -f%z "$CHUNK_FILE" 2>/dev/null || stat -c%s "$CHUNK_FILE" 2>/dev/null)

        # Show progress bar
        show_progress $((CHUNK_INDEX + 1)) $TOTAL_CHUNKS

        UPLOAD_RESPONSE=$(curl -s -X POST \
          "$API_BASE_URL/buckets/$BUCKET_ID/chunked/chunk?upload_id=$UPLOAD_ID&chunk_index=$CHUNK_INDEX" \
          -H "Authorization: Bearer $AUTH_TOKEN" \
          -F "chunk=@$CHUNK_FILE")

        STATUS=$(echo "$UPLOAD_RESPONSE" | grep -o '"status":"[^"]*' | cut -d'"' -f4)

        if [ "$STATUS" != "uploading" ]; then
            echo "" # New line after progress bar
            log_error "Failed to upload chunk $CHUNK_INDEX"
            echo "$UPLOAD_RESPONSE"
            rm -rf $TEMP_DIR
            return 1
        fi

        BYTES_UPLOADED=$((BYTES_UPLOADED + CHUNK_FILE_SIZE))
        rm "$CHUNK_FILE"
        CHUNK_INDEX=$((CHUNK_INDEX + 1))

        sleep 0.1
    done

    echo "" # New line after progress bar
    TOTAL_TIME=$(($(date +%s) - START_TIME))
    AVG_SPEED=$((FILE_SIZE / TOTAL_TIME))

    log_success "All chunks uploaded"
    log_info "Time: ${TOTAL_TIME}s | Avg speed: $(format_bytes $AVG_SPEED)/s"
    echo ""

    # ===========================
    # Step 3: Check Progress
    # ===========================

    log_info "Step 3: Verifying upload progress..."

    PROGRESS_RESPONSE=$(curl -s -X GET \
      "$API_BASE_URL/buckets/$BUCKET_ID/chunked/$UPLOAD_ID/progress" \
      -H "Authorization: Bearer $AUTH_TOKEN")

    UPLOADED_CHUNKS=$(echo "$PROGRESS_RESPONSE" | grep -o '"uploaded_chunks":[0-9]*' | cut -d':' -f2)

    if [ "$UPLOADED_CHUNKS" = "$TOTAL_CHUNKS" ]; then
        log_success "All chunks verified ($UPLOADED_CHUNKS/$TOTAL_CHUNKS)"
    else
        log_warning "Chunks verification: $UPLOADED_CHUNKS/$TOTAL_CHUNKS"
    fi
    echo ""

    # ===========================
    # Step 4: Complete Upload
    # ===========================

    log_info "Step 4: Completing upload..."

    COMPLETE_PAYLOAD=$(cat <<EOF
{
  "upload_id": "$UPLOAD_ID"
}
EOF
)

    COMPLETE_RESPONSE=$(curl -s -X POST \
      "$API_BASE_URL/buckets/$BUCKET_ID/chunked/complete" \
      -H "Authorization: Bearer $AUTH_TOKEN" \
      -H "Content-Type: application/json" \
      -d "$COMPLETE_PAYLOAD")

    FILE_HASH=$(echo "$COMPLETE_RESPONSE" | grep -o '"file_hash":"[^"]*' | cut -d'"' -f4)
    OBJECT_ID=$(echo "$COMPLETE_RESPONSE" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)

    if [ -z "$FILE_HASH" ]; then
        log_error "Failed to complete upload"
        echo "$COMPLETE_RESPONSE"
        rm -rf $TEMP_DIR
        return 1
    fi

    log_success "Upload completed successfully!"
    log_info "Object ID: $OBJECT_ID"
    log_info "File Hash: $FILE_HASH"
    echo ""

    rm -rf $TEMP_DIR
    return 0
}

# ===========================
# Main Execution
# ===========================

SUCCESSFUL_UPLOADS=0
FAILED_UPLOADS=0
TOTAL_FILES=${#FILE_PATHS[@]}

for i in "${!FILE_PATHS[@]}"; do
    FILE_INDEX=$((i + 1))

    if upload_file "${FILE_PATHS[$i]}" "$FILE_INDEX" "$TOTAL_FILES"; then
        SUCCESSFUL_UPLOADS=$((SUCCESSFUL_UPLOADS + 1))
    else
        FAILED_UPLOADS=$((FAILED_UPLOADS + 1))
    fi
done

# ===========================
# Final Summary
# ===========================

echo ""
echo -e "${COLOR_CYAN}════════════════════════════════════════════════════════${COLOR_RESET}"
echo -e "${COLOR_GREEN}                   UPLOAD SUMMARY${COLOR_RESET}"
echo -e "${COLOR_CYAN}════════════════════════════════════════════════════════${COLOR_RESET}"
echo ""
echo "Total files: $TOTAL_FILES"
echo -e "${COLOR_GREEN}✓ Successful: $SUCCESSFUL_UPLOADS${COLOR_RESET}"
[ $FAILED_UPLOADS -gt 0 ] && echo -e "${COLOR_RED}✗ Failed: $FAILED_UPLOADS${COLOR_RESET}"
echo ""

if [ $FAILED_UPLOADS -eq 0 ]; then
    log_success "All uploads completed successfully!"
    exit 0
else
    log_warning "Some uploads failed. Check logs above for details."
    exit 1
fi


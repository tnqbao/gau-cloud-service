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
MAX_PARALLEL_CHUNKS=5  # Number of chunks to upload simultaneously
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
    if [ "$bytes" -lt 1048576 ]; then
        echo "$((bytes / 1024))KB"
    else
        echo "$((bytes / 1024 / 1024))MB"
    fi
}

get_timestamp() {
    date '+%Y-%m-%d %H:%M:%S'
}

get_epoch() {
    date +%s
}

start_step() {
    local step_name="$1"
    local start_time=$(get_timestamp)
    local start_epoch=$(get_epoch)

    # Output to stderr so it doesn't interfere with return value
    echo -e "${COLOR_CYAN}▶ START $step_name at $start_time${COLOR_RESET}" >&2

    # Return both values separated by |
    echo "$start_time|$start_epoch"
}

end_step() {
    local step_name="$1"
    local start_epoch="$2"
    local end_time=$(get_timestamp)
    local end_epoch=$(get_epoch)
    local duration=$((end_epoch - start_epoch))

    # Output to stderr so it doesn't interfere with return value
    echo -e "${COLOR_GREEN}■ END   $step_name at $end_time (${duration}s)${COLOR_RESET}" >&2
    echo "" >&2

    # Return end_time|duration
    echo "$end_time|$duration"
}

print_banner() {
    echo -e "${COLOR_CYAN}"
    echo "╔════════════════════════════════════════════════════════╗"
    echo "║       Production-Grade Chunked Upload Tool             ║"
    echo "║       Server-Decided Chunk Size Architecture           ║"
    echo "║       Parallel Chunk Upload Support                    ║"
    echo "╚════════════════════════════════════════════════════════╝"
    echo -e "${COLOR_RESET}"
    echo ""
}


# ===========================
# Parallel Upload Function
# ===========================

upload_chunk_async() {
    local FILE_PATH="$1"
    local CHUNK_FILE="$2"
    local CHUNK_INDEX="$3"
    local UPLOAD_ID="$4"
    local BUCKET_ID="$5"
    local AUTH_TOKEN="$6"
    local API_BASE_URL="$7"
    local TOTAL_CHUNKS="$8"
    local PROGRESS_DIR="$9"

    local CHUNK_FILE_SIZE=$(stat -f%z "$CHUNK_FILE" 2>/dev/null || stat -c%s "$CHUNK_FILE" 2>/dev/null)
    local CURRENT_CHUNK=$((CHUNK_INDEX + 1))

    UPLOAD_RESPONSE=$(curl -s -X POST \
      "$API_BASE_URL/buckets/$BUCKET_ID/chunked/chunk?upload_id=$UPLOAD_ID&chunk_index=$CHUNK_INDEX" \
      -H "Authorization: Bearer $AUTH_TOKEN" \
      -F "chunk=@$CHUNK_FILE" 2>&1)

    STATUS_CODE=$(echo "$UPLOAD_RESPONSE" | grep -o '"status":[0-9]*' | cut -d':' -f2)

    if [ "$STATUS_CODE" = "200" ]; then
        # Mark as success
        echo "success" > "$PROGRESS_DIR/$CHUNK_INDEX.status"
        echo "$CHUNK_FILE_SIZE" > "$PROGRESS_DIR/$CHUNK_INDEX.size"

        # Calculate percentage
        UPLOADED_COUNT=$(ls "$PROGRESS_DIR"/*.status 2>/dev/null | wc -l)
        PERCENTAGE=$((UPLOADED_COUNT * 100 / TOTAL_CHUNKS))

        log_success "Chunk $CURRENT_CHUNK/$TOTAL_CHUNKS uploaded ($PERCENTAGE%)"
    else
        echo "failed" > "$PROGRESS_DIR/$CHUNK_INDEX.status"
        log_error "Chunk $CURRENT_CHUNK/$TOTAL_CHUNKS failed"
        echo "$UPLOAD_RESPONSE" > "$PROGRESS_DIR/$CHUNK_INDEX.error"
    fi

    rm "$CHUNK_FILE"
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

    # Initialize timing arrays
    declare -a STEP_NAMES
    declare -a STEP_START_TIMES
    declare -a STEP_END_TIMES
    declare -a STEP_DURATIONS

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

    STEP_NAME="STEP 1: INIT UPLOAD"
    STEP_TIMING=$(start_step "$STEP_NAME")
    STEP_START_EPOCH=$(echo "$STEP_TIMING" | cut -d'|' -f2)

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
    log_info "Parallel uploads: $MAX_PARALLEL_CHUNKS chunks at a time"

    if [ "$SERVER_CHUNK_SIZE" -ne "$PREFERRED_CHUNK_SIZE" ]; then
        log_warning "Server overrode preferred chunk size: $(format_bytes $PREFERRED_CHUNK_SIZE) → $(format_bytes $SERVER_CHUNK_SIZE)"
    fi

    STEP_END_TIMING=$(end_step "$STEP_NAME" "$STEP_START_EPOCH")
    STEP_NAMES+=("$STEP_NAME")
    STEP_START_TIMES+=($(echo "$STEP_TIMING" | cut -d'|' -f1))
    STEP_END_TIMES+=($(echo "$STEP_END_TIMING" | cut -d'|' -f1))
    STEP_DURATIONS+=($(echo "$STEP_END_TIMING" | cut -d'|' -f2))

    # ===========================
    # Step 2: Upload Chunks (Parallel)
    # ===========================

    STEP_NAME="STEP 2: PREPARE & UPLOAD CHUNKS"
    STEP_TIMING=$(start_step "$STEP_NAME")
    STEP_START_EPOCH=$(echo "$STEP_TIMING" | cut -d'|' -f2)

    TEMP_DIR=$(mktemp -d)
    PROGRESS_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR $PROGRESS_DIR" EXIT

    START_TIME=$(date +%s)

    log_info "Preparing chunks..."
  CHUNK_INDEX=0

  while [ $CHUNK_INDEX -lt $TOTAL_CHUNKS ]; do
    CHUNK_FILE="$TEMP_DIR/chunk_$CHUNK_INDEX.part"

    dd if="$FILE_PATH" of="$CHUNK_FILE" \
      bs="$SERVER_CHUNK_SIZE" \
      skip="$CHUNK_INDEX" \
      count=1 \
      status=none 2>/dev/null

    CHUNK_INDEX=$((CHUNK_INDEX + 1))
  done


    log_success "All chunks prepared"
    echo ""

    # Upload chunks in parallel batches
    CHUNK_INDEX=0
    ACTIVE_JOBS=0

    while [ $CHUNK_INDEX -lt $TOTAL_CHUNKS ]; do
        # Wait if we have too many parallel jobs
        while [ $ACTIVE_JOBS -ge $MAX_PARALLEL_CHUNKS ]; do
            # Wait for any background job to finish
            wait -n 2>/dev/null
            ACTIVE_JOBS=$((ACTIVE_JOBS - 1))
        done

        CHUNK_FILE="$TEMP_DIR/chunk_$CHUNK_INDEX.part"
        CURRENT_CHUNK=$((CHUNK_INDEX + 1))
        CHUNK_FILE_SIZE=$(stat -f%z "$CHUNK_FILE" 2>/dev/null || stat -c%s "$CHUNK_FILE" 2>/dev/null)

        log_info "Uploading chunk $CURRENT_CHUNK/$TOTAL_CHUNKS ($CHUNK_FILE_SIZE bytes)..."

        # Start upload in background
        upload_chunk_async \
            "$FILE_PATH" \
            "$CHUNK_FILE" \
            "$CHUNK_INDEX" \
            "$UPLOAD_ID" \
            "$BUCKET_ID" \
            "$AUTH_TOKEN" \
            "$API_BASE_URL" \
            "$TOTAL_CHUNKS" \
            "$PROGRESS_DIR" &

        ACTIVE_JOBS=$((ACTIVE_JOBS + 1))
        CHUNK_INDEX=$((CHUNK_INDEX + 1))

        # Small delay to avoid overwhelming the server
        sleep 0.02
    done

    # Wait for all remaining jobs
    wait

    echo ""

    # Check results
    FAILED_COUNT=$(grep -l "failed" "$PROGRESS_DIR"/*.status 2>/dev/null | wc -l)

    if [ "$FAILED_COUNT" -gt 0 ]; then
        log_error "$FAILED_COUNT chunks failed to upload"
        for error_file in "$PROGRESS_DIR"/*.error; do
            if [ -f "$error_file" ]; then
                echo "Error details:"
                cat "$error_file"
            fi
        done
        rm -rf "$TEMP_DIR" "$PROGRESS_DIR"
        return 1
    fi

    TOTAL_TIME=$(($(date +%s) - START_TIME))
    AVG_SPEED=$((FILE_SIZE / TOTAL_TIME))

    log_success "All chunks uploaded"
    log_info "Time: ${TOTAL_TIME}s | Avg speed: $(format_bytes $AVG_SPEED)/s"

    STEP_END_TIMING=$(end_step "$STEP_NAME" "$STEP_START_EPOCH")
    STEP_NAMES+=("$STEP_NAME")
    STEP_START_TIMES+=($(echo "$STEP_TIMING" | cut -d'|' -f1))
    STEP_END_TIMES+=($(echo "$STEP_END_TIMING" | cut -d'|' -f1))
    STEP_DURATIONS+=($(echo "$STEP_END_TIMING" | cut -d'|' -f2))

    # ===========================
    # Step 3: Check Progress
    # ===========================

    STEP_NAME="STEP 3: VERIFY PROGRESS"
    STEP_TIMING=$(start_step "$STEP_NAME")
    STEP_START_EPOCH=$(echo "$STEP_TIMING" | cut -d'|' -f2)

    PROGRESS_RESPONSE=$(curl -s -X GET \
      "$API_BASE_URL/buckets/$BUCKET_ID/chunked/$UPLOAD_ID/progress" \
      -H "Authorization: Bearer $AUTH_TOKEN")

    UPLOADED_CHUNKS=$(echo "$PROGRESS_RESPONSE" | grep -o '"uploaded_chunks":[0-9]*' | cut -d':' -f2)

    if [ "$UPLOADED_CHUNKS" = "$TOTAL_CHUNKS" ]; then
        log_success "All chunks verified ($UPLOADED_CHUNKS/$TOTAL_CHUNKS)"
    else
        log_warning "Chunks verification: $UPLOADED_CHUNKS/$TOTAL_CHUNKS"
    fi

    STEP_END_TIMING=$(end_step "$STEP_NAME" "$STEP_START_EPOCH")
    STEP_NAMES+=("$STEP_NAME")
    STEP_START_TIMES+=($(echo "$STEP_TIMING" | cut -d'|' -f1))
    STEP_END_TIMES+=($(echo "$STEP_END_TIMING" | cut -d'|' -f1))
    STEP_DURATIONS+=($(echo "$STEP_END_TIMING" | cut -d'|' -f2))

    # ===========================
    # Step 4: Complete Upload
    # ===========================

    STEP_NAME="STEP 4: COMPLETE UPLOAD"
    STEP_TIMING=$(start_step "$STEP_NAME")
    STEP_START_EPOCH=$(echo "$STEP_TIMING" | cut -d'|' -f2)

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

    STEP_END_TIMING=$(end_step "$STEP_NAME" "$STEP_START_EPOCH")
    STEP_NAMES+=("$STEP_NAME")
    STEP_START_TIMES+=($(echo "$STEP_TIMING" | cut -d'|' -f1))
    STEP_END_TIMES+=($(echo "$STEP_END_TIMING" | cut -d'|' -f1))
    STEP_DURATIONS+=($(echo "$STEP_END_TIMING" | cut -d'|' -f2))

    # ===========================
    # Display Timing Summary Table
    # ===========================

    echo ""
    echo -e "${COLOR_CYAN}📊 Bảng tổng kết${COLOR_RESET}"
    printf "%-36s| %-19s | %-19s | %8s\n" "STEP" "START" "END" "TIME(s)"
    printf "%-36s+%-21s+%-21s+%10s\n" "------------------------------------" "---------------------" "---------------------" "--------"

    TOTAL_DURATION=0
    for i in "${!STEP_NAMES[@]}"; do
        printf "%-36s| %-19s | %-19s | %8s\n" "${STEP_NAMES[$i]}" "${STEP_START_TIMES[$i]}" "${STEP_END_TIMES[$i]}" "${STEP_DURATIONS[$i]}"
        TOTAL_DURATION=$((TOTAL_DURATION + STEP_DURATIONS[$i]))
    done

    printf "%-36s+%-21s+%-21s+%10s\n" "------------------------------------" "---------------------" "---------------------" "--------"
    printf "%-36s| %-19s | %-19s | %8s\n" "TOTAL" "-" "-" "$TOTAL_DURATION"
    echo ""

    rm -rf "$TEMP_DIR" "$PROGRESS_DIR"
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


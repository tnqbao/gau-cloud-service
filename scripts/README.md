# Chunked Upload Script - User Guide

## 🎯 Overview

Production-grade bash script for uploading large files using chunked upload API with server-decided chunk size architecture.

## ✨ Features

- ✅ **Server-decided chunk size** - Server controls optimal chunk size
- ✅ **Multiple file upload** - Upload 1-N files in a single command
- ✅ **Progress bar** - Visual feedback for each file upload
- ✅ **Error handling** - Graceful failure with detailed error messages
- ✅ **Auto cleanup** - Temporary files cleaned up automatically
- ✅ **Colored output** - Easy-to-read terminal output
- ✅ **Git Bash compatible** - Works on Windows, Linux, macOS

## 📋 Prerequisites

- Bash shell (Git Bash on Windows, or native on Linux/macOS)
- `curl` command-line tool
- `bc` calculator (for bytes formatting)
- Valid API credentials (BUCKET_ID, AUTH_TOKEN)

## 🚀 Quick Start

### 1. Set environment variables:

```bash
export API_BASE_URL="http://localhost:8080/api/v1/cloud"
export BUCKET_ID="your-bucket-uuid-here"
export AUTH_TOKEN="your-jwt-token-here"
```

### 2. Make script executable:

```bash
chmod +x chunked_upload.sh
```

### 3. Upload files:

```bash
# Single file
./chunked_upload.sh "" /c/Downloads/OBS-Studio.exe

# Multiple files
./chunked_upload.sh "" file1.exe file2.zip file3.mp4

# With custom path
./chunked_upload.sh "installers/obs" /c/Downloads/OBS-Studio.exe

# Upload 3 videos to "videos/2024" folder
./chunked_upload.sh "videos/2024" video1.mp4 video2.mp4 video3.mp4
```

## 📖 Usage

### Syntax

```bash
./chunked_upload.sh [custom_path] <file1> [file2] [file3] ...
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `custom_path` | string | No | Folder path in bucket (use `""` for root) |
| `file1, file2, ...` | string | Yes | One or more file paths to upload |

### Environment Variables

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `API_BASE_URL` | `http://localhost:8080/api/v1/cloud` | No | API endpoint URL |
| `BUCKET_ID` | - | **Yes** | Target bucket UUID |
| `AUTH_TOKEN` | - | **Yes** | JWT authentication token |
| `PREFERRED_CHUNK_SIZE` | `10485760` (10MB) | No | Suggested chunk size (server decides final) |

## 📝 Examples

### Example 1: Upload single large file to root

```bash
export BUCKET_ID="abc-123-def-456"
export AUTH_TOKEN="eyJhbGciOiJIUzI1NiIs..."

./chunked_upload.sh "" /c/Downloads/large-file.zip
```

### Example 2: Upload multiple installers to "installers" folder

```bash
./chunked_upload.sh "installers" \
  /c/Downloads/OBS-Studio-32.0.4.exe \
  /c/Downloads/VSCode-Setup.exe \
  /c/Downloads/Chrome-Installer.exe
```

### Example 3: Upload videos with custom chunk size

```bash
export PREFERRED_CHUNK_SIZE=$((5 * 1024 * 1024))  # 5MB

./chunked_upload.sh "videos/2024" video1.mp4 video2.mp4
```

### Example 4: Set all variables inline

```bash
API_BASE_URL="http://api.example.com/api/v1/cloud" \
BUCKET_ID="abc-123" \
AUTH_TOKEN="eyJhbGc..." \
./chunked_upload.sh "documents" document.pdf
```

## 🎨 Output Example

```
╔════════════════════════════════════════════════════════╗
║       Production-Grade Chunked Upload Tool            ║
║       Server-Decided Chunk Size Architecture          ║
╚════════════════════════════════════════════════════════╝

[INFO] Preparing to upload 2 file(s)
[INFO] Custom path: installers

════════════════════════════════════════════════════════
[INFO] [1/2] Processing: /c/Downloads/OBS-Studio.exe
════════════════════════════════════════════════════════
[INFO] File: OBS-Studio-32.0.4.exe
[INFO] Size: 128.00MB (134217728 bytes)
[INFO] Type: application/vnd.microsoft.portable-executable

[INFO] Step 1: Initializing upload session...
[SUCCESS] Upload initialized
[INFO] Upload ID: a1b2c3d4...e5f6
[INFO] Server chunk size: 10.00MB (SERVER DECIDED)
[INFO] Total chunks: 13

[INFO] Step 2: Uploading chunks...
[████████████████████████████████████████████████] 100% (13/13)
[SUCCESS] All chunks uploaded
[INFO] Time: 45s | Avg speed: 2.84MB/s

[INFO] Step 3: Verifying upload progress...
[SUCCESS] All chunks verified (13/13)

[INFO] Step 4: Completing upload...
[SUCCESS] Upload completed successfully!
[INFO] Object ID: xyz-789-abc-012
[INFO] File Hash: sha256:a1b2c3d4...

════════════════════════════════════════════════════════
                   UPLOAD SUMMARY
════════════════════════════════════════════════════════

Total files: 2
✓ Successful: 2

[SUCCESS] All uploads completed successfully!
```

## 🔧 Server-Decided Chunk Size Architecture

### How it works:

1. **Client suggests** preferred chunk size (optional, default 10MB)
2. **Server decides** final chunk size (between 5MB-10MB)
3. **Server returns contract**: `upload_id`, `chunk_size`, `total_chunks`
4. **Client MUST use** the server-provided `chunk_size`

### Why server decides?

- ✅ Server knows its limits (Cloudflare, proxy, memory)
- ✅ Server can optimize based on file size and load
- ✅ Prevents malicious clients from using extreme chunk sizes
- ✅ Ensures MinIO ComposeObject requirements (min 5MB per part)

### Example flow:

```bash
# Client suggests 10MB
preferred_chunk_size: 10485760

# Server may decide differently:
# - Too small? Use default (5MB)
# - Too large? Cap at max (10MB)
# - Just right? Use client's preference

# Server response:
{
  "upload_id": "...",
  "chunk_size": 10485760,  ← CLIENT MUST USE THIS
  "total_chunks": 16
}
```

## 🐛 Troubleshooting

### Error: "BUCKET_ID not set"

```bash
export BUCKET_ID="your-bucket-uuid"
```

### Error: "AUTH_TOKEN not set"

```bash
export AUTH_TOKEN="your-jwt-token"
```

### Error: "File not found"

Check file path syntax for Git Bash:
- Windows: `/c/Users/name/file.zip` (not `C:\Users\...`)
- Use quotes for paths with spaces: `"/c/My Folder/file.zip"`

### Error: "Failed to initialize upload"

- Check API_BASE_URL is correct
- Verify AUTH_TOKEN is valid and not expired
- Ensure BUCKET_ID exists and you have permission

### Error: "Failed to upload chunk"

- Network issue - retry the upload
- Chunk too large - server will reject (should not happen with server-decided size)
- Auth token expired - refresh token and retry

### Progress bar not showing

Make sure your terminal supports ANSI colors and `\r` (carriage return).

## 📚 Related Documentation

- [Backend API Documentation](../README.md)
- [Chunked Upload Architecture](../../docs/chunked-upload.md)
- [MinIO ComposeObject Requirements](https://min.io/docs/minio/linux/developers/go/API.html#ComposeObject)

## 🤝 Contributing

Found a bug or have a feature request? Please open an issue or submit a pull request.

## 📄 License

MIT License - see LICENSE file for details.


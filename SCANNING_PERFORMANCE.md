# Media Library Scanning Performance Optimizations

## Overview

This document outlines the performance improvements implemented for the Viewra media library scanning system.

## Performance Issues Identified

### 1. Sequential File Processing

- **Problem**: Files were processed one by one in a single thread
- **Impact**: CPU cores were underutilized, especially on multi-core systems
- **Solution**: Implemented parallel processing with worker goroutines

### 2. Double Directory Walking

- **Problem**: First pass counted files, second pass processed them
- **Impact**: I/O operations were doubled, slowing down large directory scans
- **Solution**: Single-pass scanning with dynamic progress tracking

### 3. Hash Calculation Overhead

- **Problem**: SHA1 hash calculated for every file, even unchanged ones
- **Impact**: CPU and I/O intensive operation repeated unnecessarily
- **Solution**: Smart hash calculation based on file modification time and size

### 4. Metadata Extraction Blocking

- **Problem**: Music metadata extraction blocked file processing
- **Impact**: Network and file I/O for metadata slowed overall scanning
- **Solution**: Asynchronous metadata extraction with separate workers

### 5. Individual Database Operations

- **Problem**: Each file resulted in separate database queries
- **Impact**: Database became a bottleneck with many small transactions
- **Solution**: Batch database operations

## Performance Improvements Implemented

### 1. Parallel File Scanner (`parallel_scanner.go`)

- **Workers**: Configurable number of worker goroutines (default: CPU count)
- **Channels**: Buffered channels for efficient task distribution
- **Cancellation**: Context-based cancellation for clean shutdown
- **Memory Efficient**: Streaming file processing without loading all files in memory

### 2. Smart Hash Calculation (`fileutils.go`)

- **Conditional Hashing**: Only calculate hash if file size or modification time changed
- **Fast Hash**: Larger buffer (64KB) for better I/O performance
- **Existing File Lookup**: In-memory map for quick existing file checks

### 3. Batch Database Operations

- **Batch Size**: Configurable batch size (default: 50 files)
- **Transactions**: Batch operations within database transactions
- **Reduced Overhead**: Fewer database connections and query overhead

### 4. Asynchronous Metadata Processing

- **Separate Workers**: Dedicated goroutines for metadata extraction
- **Non-blocking**: File scanning continues while metadata is processed
- **Error Handling**: Metadata errors don't stop file processing

### 5. Configuration System (`config.go`)

- **Flexible Configuration**: Multiple performance profiles
- **Runtime Tuning**: Adjustable worker counts and batch sizes
- **System-specific**: Different configs for different hardware

## Performance Profiles

### Default Configuration

```go
ParallelScanningEnabled: true
WorkerCount: CPU_COUNT
BatchSize: 50
SmartHashEnabled: true
AsyncMetadataEnabled: true
```

### Conservative (Slower Systems)

```go
WorkerCount: 2
BatchSize: 25
MetadataWorkerCount: 1
```

### Aggressive (Powerful Systems)

```go
WorkerCount: 8
BatchSize: 100
MetadataWorkerCount: 4
```

## Expected Performance Gains

### File System Scanning

- **Multi-core Systems**: 2-4x faster with parallel processing
- **Large Libraries**: 50-70% reduction in scan time
- **SSD Storage**: Better utilization of high I/O throughput

### Hash Calculation

- **Unchanged Files**: 90%+ reduction in hash calculation time
- **Large Files**: Faster hashing with larger buffers
- **Incremental Scans**: Dramatically faster subsequent scans

### Database Operations

- **Batch Inserts**: 5-10x faster than individual operations
- **Transaction Overhead**: Reduced connection overhead
- **Memory Usage**: Lower memory footprint

### Metadata Extraction

- **Non-blocking**: File scanning no longer waits for metadata
- **Parallel Processing**: Multiple files processed simultaneously
- **Error Resilience**: Metadata errors don't stop scanning

## Usage Examples

### Enable Parallel Scanning

```go
manager := scanner.NewManager(db)
manager.SetParallelMode(true) // Default is true
```

### Use Different Performance Profile

```go
// For powerful systems
config := scanner.AggressiveScanConfig()

// For slower systems
config := scanner.ConservativeScanConfig()
```

### Monitor Progress

The parallel scanner maintains the same progress tracking interface:

- `Progress`: Percentage complete (0-100)
- `FilesFound`: Total files discovered
- `FilesProcessed`: Files completed
- `BytesProcessed`: Total bytes processed

## Backward Compatibility

- Original `FileScanner` remains available
- Manager can switch between parallel and sequential modes
- Same API interface maintained
- Existing scan job database schema unchanged

## Future Optimizations

### 1. File System Optimizations

- Directory-level parallelism
- File system-specific optimizations (ext4, NTFS, APFS)
- Memory-mapped file I/O for large files

### 2. Database Optimizations

- Prepared statements for batch operations
- Index optimizations for file lookups
- Connection pooling

### 3. Metadata Optimizations

- Metadata caching
- Partial metadata updates
- Thumbnail generation pipeline

### 4. Network Storage

- Remote file system optimizations
- Chunked transfers for network storage
- Connection pooling for remote databases

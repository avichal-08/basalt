# Basalt

A high-performance, highly concurrent, persistent in-memory key-value store written in Go.

Basalt keeps the active key-value state in memory for extreme read/write speeds, while using an append-only file (AOF) for persistence and crash recovery. It features a 32-way sharded architecture to eliminate global lock contention, zero-allocation I/O paths to prevent garbage collection pauses, and a lossless, non-blocking background compaction engine.

## Features

- High-Concurrency Sharding: 32-way sharded map using fnv32 hashing to eliminate global RWMutex contention.

- Zero-Allocation Hot Paths: Uses stack-allocated varints, sync.Pool byte buffers, and native string copying to achieve zero heap allocations during AOF writes.

- Lossless Background Compaction: Employs an atomic double-checked locking pattern and a sidecar buffer to safely rewrite gigabytes of data without dropping incoming mutations or stalling database reads.

- Crash-Resilient Recovery: Gracefully detects and truncates torn writes caused by unexpected power loss or process crashes, while maintaining strict CRC32 corruption detection for actual bit-rot.

- Cross-Platform Mmap: Native POSIX memory-mapped AOF reads for Linux/macOS and optimized CreateFileMapping for Windows via build tags.

## Architecture

```text
                    ┌─────────────────────┐
                    │      Basalt API     │
                    │    Set / Get / Del  │
                    └──────────┬──────────┘
                               │
                 ┌─────────────┴─────────────┐
                 │                           │
                 ▼                           ▼
        ┌─────────────────┐         ┌─────────────────┐
        │  In-Memory Map  │         │      AOF        │
        │ map[string]string│        │ Append-only log │
        └─────────────────┘         └────────┬────────┘
                                             │
                                             ▼
                                      Memory-mapped read
                                             │
                                             ▼
                                          Recovery

                         Background Compaction
                                  │
                                  ▼
                       Rewrite current state
                         into a new AOF
```

## Data Model
Basalt stores key-value pairs as:
```text
     string key → string value
```
Each mutation is persisted to the AOF before updating the in-memory state.

Supported operations:
```text
      SET key value
      DELETE key
      GET key
```

## AOF Record Format
Each AOF record contains:
```text
┌────────────┬──────┬────────────┬────────────┬─────────┬─────────┐
│   CRC32    │ Op   │ Key Length │ Val Length │   Key   │  Value  │
└────────────┴──────┴────────────┴────────────┴─────────┴─────────┘
   4 bytes     1B       varint       varint       bytes     bytes
```
The CRC32 checksum is calculated over the record payload and verified during
recovery.

This allows Basalt to detect corrupted records instead of silently rebuilding
state from invalid data.

## Persistence
Basalt uses an append-only file rather than rewriting the complete dataset on
every mutation.

## Write
```text
Set(key, value)
      │
      ▼
Append SET record to AOF
      │
      ▼
Update in-memory map
```
## Delete
```text
Delete(key)
      │
      ▼
Append DELETE record to AOF
      │
      ▼
Remove key from in-memory map
```
On startup, Basalt reads the AOF and reconstructs the in-memory state.

## Memory-Mapped Recovery
The AOF is memory-mapped during reads and recovery.
```text
AOF on disk
    │
    ▼
Memory mapping
    │
    ▼
Parse records
    │
    ├── CRC valid ──► Apply operation
    │
    └── CRC invalid ─► Detect corruption
```
Memory mapping allows the recovery path to access the log through the mapped
file instead of repeatedly issuing normal file reads.

## Concurrency
Basalt uses Go's sync.RWMutex to protect the in-memory map.

### Get
```text
Get
 │
 └── RLock
       │
       └── Read map
             │
             └── RUnlock
```

### Set / Delete
```text
Set / Delete
 │
 └── Lock
       │
       ├── Persist operation
       └── Modify map
             │
             └── Unlock
```
Multiple readers can access the store concurrently, while writes are
serialized.

## Compaction
Because the AOF only appends records, old mutations can become obsolete.

For example:
```text
SET user alice
SET user bob
SET user charlie
DELETE user
SET token xyz
```
Only the latest state needs to be retained.

Basalt periodically compacts the AOF by writing the current in-memory state
to a new log and replacing the old log.

```text
Old AOF
   │
   ▼
Read current in-memory state
   │
   ▼
Write compact AOF
   │
   ▼
Replace old AOF
```

This prevents the append-only log from growing indefinitely.

## Benchmarks

Benchmarked on:

```text
CPU:          Intel Core i3-1005G1 @ 1.20 GHz
OS:           Windows
Architecture: amd64
```

Direct Get benchmarks:
```text
~60–70 ns/op
0 B/op
0 allocs/op
```

The benchmark measures the in-memory read path and is intended as a
microbenchmark rather than an end-to-end storage benchmark.

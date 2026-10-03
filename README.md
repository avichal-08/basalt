# Basalt

A high-performance, highly concurrent, persistent in-memory key-value store written in Go.

Basalt keeps the active key-value state in memory for extreme read/write speeds, while using an append-only file (AOF) for persistence and crash recovery. It features a 32-way sharded architecture to eliminate global lock contention, zero-allocation I/O paths to prevent garbage collection pauses, and a non-blocking background compaction engine.

## Features

- High-Concurrency Sharding: 32-way sharded map using fnv32 hashing to eliminate global RWMutex contention.

- Zero-Allocation Hot Paths: Uses stack-allocated varints, sync.Pool byte buffers, and native string copying to achieve zero heap allocations during AOF writes.

- Non-Blocking Background Compaction: Snapshots the store into a new log while reads and writes continue, records concurrent mutations in a sidecar buffer, then replays them and swaps the log in a short exclusive step. Reads are never blocked; writes pause only for that final swap.

- Crash-Resilient Recovery: Gracefully detects and truncates torn writes caused by unexpected power loss or process crashes, while maintaining strict CRC32 corruption detection for actual bit-rot.

- Memory-Mapped Recovery: The AOF is read through a POSIX mmap on Linux/macOS (build tag `!windows`). Native Windows is intentionally unsupported; on Windows, run it under WSL.

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

The in-memory map is split into 32 shards, each guarded by its own `sync.RWMutex`.

### Get

```text
Get
 └── shard.RLock → read map → RUnlock
```

### Set / Delete

```text
Set / Delete
 └── swapMu.RLock          (excluded only during the compaction log swap)
       └── shard.Lock
             ├── append record to AOF
             ├── update in-memory map
             └── record diff for compaction (if running)
```

The log append happens under the shard lock, so the order of records in the AOF
is the same as the order in which the map was updated. Replaying the log after
a restart therefore reproduces the exact in-memory state. If the AOF write fails
the map is left unchanged.

Reads on different shards never contend. Writes to the same shard are serialized.

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

Basalt compacts the AOF every 5 minutes by writing the current state to a new
log and replacing the old one.

```text
Compact()
   │
   ├── 1. Snapshot (writers keep running)
   │      copy each shard into <path>.tmp; mutations that happen meanwhile
   │      are also appended to an in-memory buffer
   │
   └── 2. Swap (swapMu held exclusively, writes pause briefly)
          replay buffer → fsync + close tmp → close old AOF
          → rename tmp over AOF → reopen
```

Only one compaction runs at a time. A leftover `<path>.tmp` from a crashed
compaction is deleted on startup and before each run; the main log is always
the source of truth.

This prevents the append-only log from growing indefinitely.

## Benchmarks

Benchmarked on:

```text
CPU:          Intel Core i3-1005G1 @ 1.20 GHz
OS:           Linux (WSL2 on Windows)
Architecture: amd64
```

To ensure accurate measurement of the sharding and routing logic, all concurrent benchmarks distribute the load evenly across 1,024 unique keys rather than pinning a single cached key.

| Operation                 |  Performance | Allocations | Description                                                 |
| ------------------------- | -----------: | ----------: | ----------------------------------------------------------- |
| **Get (Concurrent)**      |    ~60 ns/op |      0 B/op | 100% read workload. 32-way sharded lock resolution.         |
| **Set (Memory + AOF)**    | ~2,090 ns/op |      0 B/op | 100% write workload. ~478,000 ops/sec.                      |
| **Mixed (90% R / 10% W)** |   ~434 ns/op |      0 B/op | Simulated real-world RWMutex contention with live disk I/O. |
| **Raw AOF Append**        | ~1,713 ns/op |      0 B/op | Zero-allocation sequential disk writes via `sync.Pool`.     |
| **Background Compaction** |     ~21.7 ms |           — | Non-blocking snapshot, then a short exclusive log swap.     |

These synthetic uniform microbenchmarks measure the raw compute, concurrency, and I/O efficiency of the core storage engine rather than end-to-end network client overhead.

## Known Limitations

- **Durability window:** the AOF is `fsync`ed once per second, so a power loss can lose up to about one second of acknowledged writes (a process crash alone loses none, since writes reach the OS before `Set` returns). The `fsync` error is not surfaced.
- **Corruption vs. torn writes:** a record cut short at the end of the file is treated as a torn write and truncated. A corrupted length field in the middle of the file looks the same and also truncates everything after it; only a CRC mismatch on a fully-present record is reported as corruption.
- **Memory:** the whole dataset must fit in RAM, and compaction briefly needs disk space for a second copy.
- **Interface:** a local CLI only; there is no network server yet.
- **Platform:** Linux and macOS only (POSIX mmap). Native Windows builds are intentionally unsupported; use WSL.

## Testing

```bash
go test -race ./...
```

Includes tests for concurrent writes to one key surviving a restart, compaction
under concurrent writers, torn-write recovery, CRC corruption detection, and
stale compaction temp files.

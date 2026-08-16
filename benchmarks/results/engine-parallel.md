# Engine micro-benchmark (built-in Go benchmark)

This is a **scheduling sanity benchmark**, not a Tern-vs-Fastlane comparison.
It proves the release engine's parallel-group scheduler actually overlaps
builds. Cross-tool claims stay in `../README.md` and wait for `results/` from
the harness.

## Reproduce

```sh
go test ./internal/engine/ -bench . -benchtime 10x -run ^$ -benchmem
```

Two `benchAdapter` builds each sleep 50 ms. The parallel lane runs android +
ios concurrently; the sequential lane uses two same-platform builds, which the
scheduler refuses to batch.

## Measured (2026-08-16, Windows 11, go 1.26.5, amd64, 16 logical CPUs)

| Benchmark | ns/op | B/op | allocs/op | wall for 2×50 ms builds |
|---|---|---|---|---|
| BenchmarkLane_2ParallelBuilds   | 63 316 230 | 80 267 | 297 | ~63 ms |
| BenchmarkLane_2SequentialBuilds | 130 356 530 | 115 169 | 371 | ~130 ms |

Parallel batch finishes two 50 ms builds in ~63 ms — a ~2.1× wall-clock win
over running the same two builds sequentially, as designed. These numbers are
for the mocked `benchAdapter` only; real Flutter/Gradle build time dominates
any Tern-internal overhead, so this says nothing about Fastlane.

Run on a comparable runner and update this table; keep the runner fields from
`../README.md` (runner class).
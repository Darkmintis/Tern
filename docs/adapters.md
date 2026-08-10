# Future adapters (structure only)

Tern v0 ships **Flutter only**. These packages exist so roadmap work has a home:

| Package | Phase | Status |
|---|---|---|
| [`internal/adapter/flutter`](../internal/adapter/flutter) | 1 | Active |
| [`internal/adapter/native`](../internal/adapter/native) | 2 | Scaffold — Detect off, Build refuses live runs |
| [`internal/adapter/kmp`](../internal/adapter/kmp) | 3 | Scaffold |
| [`internal/adapter/reactnative`](../internal/adapter/reactnative) | 4 | Scaffold |

When enabling a later phase: turn `Detect` back on, implement `Build`, dogfood on a real app, then document it as supported.

# poplar

A bubbletea terminal email client, single binary, built from one Go
module. Poplar is being rebuilt from the ground up against a settled
architecture: one process, a SQLite store as the source of truth,
background sync and rendering engines, and a bubbletea UI that never
touches the network or writes the store directly.

The build is in progress. The previous client is archived on branch
`legacy` (tag `poplar-legacy`) and is not the current state of this
repository.

The re-founding charter is
`docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`. A
full README, with usage and features, lands once there is a client to
document.

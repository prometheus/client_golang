This source code is a stripped down version of zstd from the https://github.com/klauspost/compress/tree/517288e9a6e1dd4dea10ad42ffe2829c58dadf51/zstd.

Motivation: https://github.com/kubernetes/kubernetes/pull/130569#discussion_r1981503174

Changes:
* Remove all but things necessary to use and create zstd.NewWriter for SpeedFastest mode.
* Use github.com/cespare/xxhash/v2 instead of vendored copy.

The goal is to remove this once stdlib will support zstd.

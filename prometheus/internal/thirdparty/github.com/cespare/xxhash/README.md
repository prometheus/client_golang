# xxhash

[![Go Reference](https://pkg.go.dev/badge/github.com/cespare/xxhash/v2.svg)](https://pkg.go.dev/github.com/cespare/xxhash/v2)
[![Test](https://github.com/cespare/xxhash/actions/workflows/test.yml/badge.svg)](https://github.com/cespare/xxhash/actions/workflows/test.yml)

xxhash is a Go implementation of the 64-bit
[xxHash](http://cyan4973.github.io/xxHash/) algorithm, XXH64. This is a
high-quality hashing algorithm that is much faster than anything in the Go
standard library.

This package provides a straightforward API:

```
func Sum64(b []byte) uint64
func Sum64String(s string) uint64
type Digest struct{ ... }
    func New() *Digest
```

The `Digest` type implements hash.Hash64. Its key methods are:

```
func (*Digest) Write([]byte) (int, error)
func (*Digest) WriteString(string) (int, error)
func (*Digest) Sum64() uint64
```

The package is written with optimized pure Go and also contains an even faster
assembly implementation for amd64. If desired, the `purego` build tag opts into
using the Go code even on amd64.

## Compatibility

This package is in a module and the latest code is in version 2 of the module.
You need a version of Go with at least "minimal module compatibility" to use
github.com/cespare/xxhash/v2:

* 1.9.7+ for Go 1.9
* 1.10.3+ for Go 1.10
* Go 1.11 or later

I recommend using the latest release of Go.

## Benchmarks

Here are some quick benchmarks comparing the pure-Go and assembly
implementations of Sum64.

| input size | purego | asm |
| --- | --- | --- |
| 4 B   |  1052.65 MB/s |  1278.48 MB/s  |
| 100 B | 6816.82 MB/s | 7881.09 MB/s  |
| 4 KB  | 11924.07 MB/s | 17323.63 MB/s |
| 10 MB | 11205.21 MB/s | 15484.85 MB/s |

These numbers were generated on Ubuntu 18.04 with an Intel i7-8700K CPU using
the following commands under Go 1.17:

```
go test -tags purego -benchtime 10s -bench 'Sum64$'
go test -benchtime 10s -bench 'Sum64$'
```

## Projects using this package

- [InfluxDB](https://github.com/influxdata/influxdb)
- [Prometheus](https://github.com/prometheus/prometheus)
- [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)
- [FreeCache](https://github.com/coocood/freecache)
- [FastCache](https://github.com/VictoriaMetrics/fastcache)

## 0.3.0 / 2015-03-03
* [CHANGE] Changed the fingerprinting for metrics. THIS WILL INVALIDATE ALL
  PERSISTED FINGERPRINTS. IF YOU COMPILE THE PROMETHEUS SERVER WITH THIS
  VERSION, YOU HAVE TO WIPE THE PREVIOUSLY CREATED STORAGE.
* [CHANGE] LabelValuesToSignature removed. (Nobody had used it, and it was
  arguably broken.)
* [CHANGE] Vendored dependencies. Those are only used by the Makefile. If
  client_golang is used as a library, the vendoring will stay out of your way.
* [BUGFIX] Remove a weakness in the fingerprinting for metrics. (This made
  the fingerprinting change above necessary.)
* [FEATURE] Added new fingerprinting functions SignatureForLabels and
  SignatureWithoutLabels to be used by the Prometheus server. These functions
  require fewer allocations than the ones currently used by the server.

## 0.2.0 / 2015-02-23
* [FEATURE] Introduce new Histagram metric type.
* [CHANGE] Ignore process collector errors for now (better error handling
  pending).
* [CHANGE] Use clear error interface for process pidFn.
* [BUGFIX] Fix Go download links for several archs and OSes.
* [ENHANCEMENT] Massively improve Gauge and Counter performance.
* [ENHANCEMENT] Catch illegal label names for summaries in histograms.
* [ENHANCEMENT] Reduce allocations during fingerprinting.
* [ENHANCEMENT] Remove cgo dependency. procfs package will only be included if
  both cgo is available and the build is for an OS with procfs.
* [CLEANUP] Clean up code style issues.
* [CLEANUP] Mark slow test as such and exclude them from travis.
* [CLEANUP] Update protobuf library package name.
* [CLEANUP] Updated vendoring of beorn7/perks.

## 0.1.0 / 2015-02-02
* [CLEANUP] Introduced semantic versioning and changelog. From now on,
  changes will be reported in this file.

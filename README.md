# barney.ci/go-store

[![GoDoc](https://godoc.org/barney.ci/go-store?status.svg)](https://godoc.org/barney.ci/go-store)
[![GitHub](https://img.shields.io/github/license/barney-ci/go-store?color=brightgreen)](LICENSE)

Pure-Go, cross-platform file locking and atomic swapping library.

go-store provides functions to streamline the following use-cases:

* Creating PID lockfiles
* Creating files that are safe to read and write from among many processes
* Atomically updating files using common marshalling libraries

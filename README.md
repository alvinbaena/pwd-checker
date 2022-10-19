# Password Checker

A set of CLI tools for downloading, creating, and searching an offline version of
the [Pwned Passwords](https://haveibeenpwned.com/Passwords) database.

Due to the complexity of efficiently loading and querying the 850+ million passwords into any sort
of database this project
uses [Golomb Coded Sets](https://giovanni.bajo.it/post/47119962313/golomb-coded-sets-smaller-than-bloom-filters)
as the storage medium. Golomb Coded Sets are a probabilistic data structure similar to bloom
filters, but with a more compact memory footprint and a slower query time.

With the usage of GCS, the Pwned Passwords v8 list (SHA1) that occupies around 40 GB uncompressed
only uses about 3 GB as a Golomb Coded Set. The set, when used for querying returns results with a
one in a hundred million false positive rate in less than 10ms.

## Server

To do

## CLI

To do

## Profiling

install graphviz
go tool pprof http://localhost:6060/debug/pprof/heap > png

## Load Tests

Grafana dashboard: 13861
k6 run -o statsd -e API_BASE_URL="http://localhost:3100" hash.js
k6 run -o statsd -e API_BASE_URL="http://localhost:3100" password.js

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

### Load passwords.js

i7 7700HQ (4c/8t)
```
          /\      |‾‾| /‾‾/   /‾‾/
     /\  /  \     |  |/  /   /  /
    /  \/    \    |     (   /   ‾‾\
   /          \   |  |\  \ |  (‾)  |
  / __________ \  |__| \__\ \_____/ .io

  execution: local
     script: password.js
     output: -

  scenarios: (100.00%) 2 scenarios, 100 max VUs, 6m30s max duration (incl. graceful stop):
           * smoke: 1 looping VUs for 1m0s (gracefulStop: 30s)
           * load: Up to 100 looping VUs for 5m0s over 3 stages (gracefulRampDown: 30s, startTime: 1m0s, gracefulStop: 30s)


running (6m00.0s), 000/100 VUs, 27324 complete and 0 interrupted iterations
smoke ✓ [======================================] 1 VUs        1m0s
load  ✓ [======================================] 000/100 VUs  5m0s

     ✓ is ok
     ✓ is pwned
     ✓ is not pwned

     checks.........................: 100.00% ✓ 163944   ✗ 0
     data_received..................: 18 MB   49 kB/s
     data_sent......................: 14 MB   40 kB/s
     http_req_blocked...............: avg=9.1µs    min=0s      med=0s       max=12.39ms p(90)=0s      p(95)=0s
     http_req_connecting............: avg=5.49µs   min=0s      med=0s       max=12.39ms p(90)=0s      p(95)=0s
     http_req_duration..............: avg=669.04ms min=9.51ms  med=681.53ms max=4.21s   p(90)=1.21s   p(95)=1.39s
       { expected_response:true }...: avg=669.04ms min=9.51ms  med=681.53ms max=4.21s   p(90)=1.21s   p(95)=1.39s
   ✓ http_req_failed................: 0.00%   ✓ 0        ✗ 81972
     http_req_receiving.............: avg=59.15µs  min=0s      med=0s       max=6.51ms  p(90)=81.78µs p(95)=520.29µs
     http_req_sending...............: avg=17.16µs  min=0s      med=0s       max=9.41ms  p(90)=0s      p(95)=0s
     http_req_tls_handshaking.......: avg=0s       min=0s      med=0s       max=0s      p(90)=0s      p(95)=0s
     http_req_waiting...............: avg=668.96ms min=9.12ms  med=681.47ms max=4.21s   p(90)=1.21s   p(95)=1.39s
     http_reqs......................: 81972   227.6835/s
     iteration_duration.............: avg=881.67ms min=21.99ms med=894ms    max=4.21s   p(90)=1.5s    p(95)=1.76s
     iterations.....................: 27324   75.8945/s
     vus............................: 1       min=1      max=100
     vus_max........................: 100     min=100    max=100
```

# Offline Pwned Passwords Checker

A set of CLI tools for downloading, creating, and searching an offline version of
the [Pwned Passwords](https://haveibeenpwned.com/Passwords) database.

Due to the complexity of efficiently loading and querying the 850+ million passwords into any sort
of database this project
uses [Golomb Coded Sets](https://giovanni.bajo.it/post/47119962313/golomb-coded-sets-smaller-than-bloom-filters)
as the storage medium. Golomb Coded Sets are a probabilistic data structure similar to bloom
filters, but with a more compact memory footprint and a somewhat slower query time.

Using GCS, the Pwned Passwords v8 list (SHA1 encoded) that occupies around 40 GB uncompressed
only uses about 3 GB as a Golomb Coded Set. The set, when used for querying returns results with a
one in a hundred million false positive rate in less than 10ms (with SSD storage).

This project also includes a simple REST API for querying the generated GCS Pwned Password database
for use in other systems that need this kind of check.

## Why?

Some use cases (like the one that inspired this project) require that no passwords (not even partial
hashes) ever, ever leave the local network. This means that the use of the
excellent [Have I Been Pwned range API](https://haveibeenpwned.com/API/v2#SearchingPwnedPasswordsByRange)
is forbidden.

It may also be used in _airgapped_ networks where a strong security posture is required for
_in-house_ applications.

## CLI

The CLI tool includes 3 different modes of operation: `download`, `create`, and `query`. Each
command needs the output of the previous one to work correctly. This means that the `create` command
requires the output file from the `download` command, and the `query` command needs the file output
from the `create` command.

Each sub-command has a `-h` (help) flag to help with their use, but a fast example on how to get up
and running "quickly" are presented here:

```shell
# Download the pwned passwords SHA1 file
# this takes a while, more threads may make this go faster
go run cmd/pwd-checker/main.go download -o "/home/user/pwned-pwds.txt"
# Create a GCS file with a 1-in-100m false positive rate
# this also may take a while, SSD storage makes this go much faster, RAM is also important
go run cmd/pwd-checker/main.go create -i "/home/user/pwned-pwds.txt" -o "/home/user/pwned-pwds-p100m.gcs" -g 1024 -p 100000000
# Run an interactive shell session to query the database.
# May also be run non interactively if omitting the -n flag
# CTRL+C to interrupt
go run cmd/pwd-checker/main.go query -n -i "/home/user/pwned-pwds-p100m.gcs"
```

### Things to know about the CLI

1. The download command uses the haveibeenpwned.com API to download the password hashes. It does not
   download the files from the website, so the file downloaded using this command has more leaked
   passwords than the ones available on the website.
2. The download may appear stuck at the end, it's not. With slower storage the writing of the file
   may take more time, even after all hashes have already been downloaded. Faster storage (SSD)
   makes this final step faster.
3. The downloaded file requires at least 40GB of storage. If your output path does not have enough
   storage the command may error out before starting.
4. Increasing the thread count when using the download command improves the download speed. 128
   threads netted me a constant 150Mbps download speed. If the download feels to slow with the
   default threads set you may increase it, but there are diminishing returns based on the amount of
   logical CPU cores, internet speed, and storage speed.
5. The create command has a minimum RAM warning. The calculation is not that precise. It will eat
   all your available RAM, but the minimum amount of memory **is** enforced. In my experience
   closing all other programs when running this command reduces the processing time by 2-3 minutes.

## Server

The `serve` command exposes a simple unauthenticated REST API to query an already generated GCS
database from the other commands of this project. The server uses and requires TLS to start.

the flags `--self-tls`, `--tls-key`, and `--tls-cert` configure the certificate to be used by the
server. If `--tls-key` and `--tls-cert` are used the value of `--self-tls` is ignored. `--self-tls`
generates a self-signed certificate on server start. It has a validity of 30 days, and regenerates
on each server restart.

To change the port use the `--port` flag, by default it uses port `3100`.

To start the server use the `serve` command:

```shell
# Start the server with a self signed certificate on port 3100
go run cmd/pwd-checker/main.go serve -i "/home/user/pwned-pwds-p100m.gcs" --self-tls
# Start the server with your own certificates on port 3100
go run cmd/pwd-checker/main.go serve -i "/home/user/pwned-pwds-p100m.gcs" --tls-key "/home/user/tls/pwned.key" --tls-cert "/home/user/tls/pwned.pem"
```

### Endpoints

The server exposes two endpoints, one to check a SHA1 hash directly, for example if you don't want
to expose user passwords over the network; and another to check a plain text password directly.

### Check Hash

```
POST /v1/check/hash

# Request
{
    "hash": "5baa61e4c9b93f3f0682250b6cf8331b7ee68fd8"
}

# Response
{
    "pwned": true
}
```

### Check password

This API also checks the password strength of the supplied password. The check is done using the
zxcvbn algorithm developed by Dropbox. This can be useful to present the user with a strength meter
on password change, or for use in other risk evaluation systems you may have in use.

```
POST /v1/check/password

# Request
{
    "password": "Password1"
}

# Response
{
    "pwned": true,
    "strength": {
        "crackTime": 0,
        "crackTimeDisplay": "instant",
        "score": 0
    }
}
```

### Things to know about the server

1. To improve performance each query opens a file handle to the referenced GCS file, this may
   require an increase of the max open files OS variables. The handle is closed as soon as the query
   is complete.
2. The server logs to stdout in JSON format.
3. The server logs the HTTP calls, also in JSON format.
4. The server caches the password check requests for one hour, with a max of 50.000 unique requests
   cached.
5. The server supports the autoconfiguration of a self-signed TLS certificate (valid for 30 days)
   with the use of the `self-tls` flag. This certificate is regenerated on each server start.

### Docker (experimental)

There is a `docker-compose.yaml` and `Dockerfile` file that builds the server to be used. Be warned
that the file reads are incredibly slow, so the start-up procedure takes some time, and requests
tend to take 500-1500ms.

Use the Docker container at your own peril...

**PD:** I will probably accept PR's for the Docker container to work correctly.

## Profiling

Every command of this project has support for GO's pprof package. To enable profiling set
the `--profile` and `--profile-port` flags to start an HTTP pprof server. By default, the profile
port used is `6060`.

## Unit Tests

The project comes with unit tests for the `gcs` amd `hibp` package only. All other packages are
untested for now.

## Load Tests

I have done some load tests using [K6](https://k6.io/). The test checks 3 different password / hash
combinations per virtual user, so 100 VUs equal 300 requests per iteration.

In the `tests/k6` folder are the k6 scripts that test the API endpoints. There is also a Grafana
dashboard, with a statsd service to see the load test results in real time.

To use the Grafana dashboard a `docker-compose.yaml` file is provided in the `test/k6/realtime`
folder. Some manual configuration is needed before the test results can be seen in the dashboard:

1. After running `docker compose up`, open [grafana](http://locahlhost:4020). User: `admin`,
   Pass: `admin`
2. Create the prometheus datasource pointing to `http://prometheus:9090`
3. Import the dashboard with id `13861`
4. Turn the API server on
5. Run the tests

### Using statsd

``` shell
# Test the hash check API
k6 run -o statsd -e API_BASE_URL="https://localhost:3100" hash.js
# Test the password check API
k6 run -o statsd -e API_BASE_URL="https://localhost:3100" password.js
```

### Not using statsd

``` shell
# Test the hash check API
k6 run -e API_BASE_URL="https://localhost:3100" hash.js
# Test the password check API
k6 run -e API_BASE_URL="https://localhost:3100" password.js
```

### Load passwords.js

This test was done on an i7 7700HQ (4c/8t) laptop with 32GB of RAM with no request caching done.

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

# Special thanks

Special thanks to [Thomas Hurst](https://github.com/Freaky) for having an example of using the GCS
algorithm in [this](https://github.com/Freaky/gcstool) repository, and
to [Giovanni Bajo](https://github.com/rasky) for writing the original GCS blog that inspired all of
this.

Also thanks to [haveibeenpwned](https://haveibeenpwned.com) for providing the hashes of leaked
passwords for free. Troy Hunt is the best.

# Password Checker

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

## CLI

The CLI tool includes 3 different modes of operation: `download`, `create`, and `query`. Each
command needs the output of the previous one to work correctly. This means that the `create` command
requires the output file from the `download` command, and the `query` command needs the file output
from the `create` command.

Each sub-command has a `-h` (help) flag to help with their use, but a fast example on how to get up
and running quickly are presented here:

```shell
# Download the pwned passwords SHA1 file
# this takes a while, more threads may make this go faster
go run cmd/pwd-chechker/cmd.go download -o "/home/user/pwned-pwds.txt"
# Create a GCS file with a 1-in-100m false positive rate
# this also may take a while, SSD storage makes this go much faster, also RAM is important
go run cmd/pwd-checker/cmd.go create -i "/home/user/pwned-pwds.txt" -o "/home/user/pwned-pwds-p100m.gcs" -g 1024 -p 100000000
# Run an interactive shell session to query the database.
# May also be run non interactively if omitting the -n flag
# CTRL+C to interrupt
go run cmd/pwd-checker/cmd.go query -n -i "D:\Work\pwned-go-p100m.gcs"
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
4. The create command has a minimum RAM warning. The calculation is not that precise. It will eat
   all your available RAM, but the minimum amount of memory **is** enforced. In my experience
   closing all other programs when running this command reduces the processing time by 2-3 minutes.

## Server

The server exposes a simple unauthenticated REST API to query an already generated GCS database from
the CLI tool of this project.

To start the server you need to set the following two environment variables:

- `PORT`: The port the server will listen on
- `GCS_FILE`: Absolute path of the GCS file used to check for passwords.

Then just run the server.

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

### Docker?

There is a Dockerfile, but local volumes have not worked for me. So use at your own peril.

## Load Tests

I have done some load tests using [K6](https://k6.io/). The test checks 3 different password / hash
combinations per virtual user, so 100 VUs equal 300 requests per iteration.

In the `tests/k6` folder are the k6 scripts that test the API endpoints. There is also a Grafana
dashboard, with a statsd service to see the load test results in real time.

To use the Grafana dashboard a `docker-compose.yaml` file is provided in the `test/k6/realtime`
folder. Some manual configuration is needed before the test results can be seen in the dashboard:

1. After running `docker compose up`, open [grafana](http://locahlhost:4020). User: `admin`,
   Pass: `admin`
2. Import the dashboard with id `13861`
3. Turn the API server on
4. Run the tests

### Using statsd

``` shell
# Test the hash check API
k6 run -o statsd -e API_BASE_URL="http://localhost:3100" hash.js
# Test the password check API
k6 run -o statsd -e API_BASE_URL="http://localhost:3100" password.js
```

### Not using statsd

``` shell
# Test the hash check API
k6 run -e API_BASE_URL="http://localhost:3100" hash.js
# Test the password check API
k6 run -e API_BASE_URL="http://localhost:3100" password.js
```

### Load passwords.js

This test was done on an i7 7700HQ (4c/8t) laptop with 32GB of RAM.

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

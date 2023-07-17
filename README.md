# xk6-nebula

This is a [k6](https://github.com/k6io/k6) extension using the [xk6](https://github.com/k6io/xk6) system.

Used to test [NebulaGraph](https://github.com/vesoft-inc/nebula).

## Dependency

* k6  v0.33.0
* xk6 v0.4.1
* Golang 1.16+

## Version match

k6-plugin now support NebulaGraph above v3.0.0.

## Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

* [Go toolchain](https://go101.org/article/go-toolchain.html)
* Git

Then:

1. Download `xk6`:

  ```bash
  go install github.com/k6io/xk6/cmd/xk6@v0.4.1
  ```

2. Build the binary:

```bash
# build with the latest version.
make 

# build with local source code
make build-dev

# or build with specified version
xk6 build --with github.com/vesoft-inc/k6-plugin@{version}
# e.g. build v0.0.8
xk6 build --with github.com/vesoft-inc/k6-plugin@v0.0.8
# e.g. build master
xk6 build --with github.com/vesoft-inc/k6-plugin@master
```

## Example

```javascript
import nebulaPool from 'k6/x/nebulagraph';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

var latencyTrend = new Trend('latency', true);
var responseTrend = new Trend('responseTime', true);

// option configuration, please refer more details in this doc.
var graph_option = {
 address: "192.168.8.6:10010",
 space: "sf1",
 csv_path: "person.csv",
 csv_delimiter: "|",
 csv_with_header: true
};

nebulaPool.setOption(graph_option);
var pool = nebulaPool.init();
// initial session for every vu
var session = pool.getSession()

String.prototype.format = function () {
  var formatted = this;
  var data = arguments[0]

  formatted = formatted.replace(/\{(\d+)\}/g, function (match, key) {
    return data[key]
  })
  return formatted
};

export default function (data) {
  // get csv data from csv file
  let d = session.getData()
  // {0} means the first column data in the csv file
  let ngql = 'go 2 steps from {0} over KNOWS'.format(d)
  let response = session.execute(ngql)
  check(response, {
    "IsSucceed": (r) => r.isSucceed() === true
  });
  // add trend
latencyTrend.add(response.getLatency()/1000);
responseTrend.add(response.getResponseTime()/1000);
};

export function teardown() {
  pool.close()
}

```

## Result

```bash
# -u means how many virtual users, i.e the concurrent users
# -d means the duration that test running, e.g. `3s` means 3 seconds, `5m` means 5 minutes.
>./k6 run nebula-test.js -u 3 -d 3s                                                      


          /\      |‾‾| /‾‾/   /‾‾/
     /\  /  \     |  |/  /   /  /
    /  \/    \    |     (   /   ‾‾\
   /          \   |  |\  \ |  (‾)  |
  / __________ \  |__| \__\ \_____/ .io

testing option: {"pool_policy":"connection","output":"output.csv","output_channel_size":10000,"address":"192.168.8.6:10010","timeout_us":0,"idletime_us":0,"max_size":400,"min_size":0,"username":"root","password":"nebula","space":"sf1","csv_path":"person.csv","csv_delimiter":"|","csv_with_header":true,"csv_channel_size":10000,"csv_data_limit":500000,"retry_times":0,"retry_interval_us":0,"retry_timeout_us":0,"ssl_ca_pem_path":"","ssl_client_pem_path":"","ssl_client_key_path":""}
  execution: local
     script: nebula-test.js
     output: engine

  scenarios: (100.00%) 1 scenario, 3 max VUs, 33s max duration (incl. graceful stop):
           * default: 3 looping VUs for 3s (gracefulStop: 30s)


     ✓ IsSucceed

     █ teardown

     checks...............: 100.00% ✓ 3529        ✗ 0
     data_received........: 0 B     0 B/s
     data_sent............: 0 B     0 B/s
     iteration_duration...: avg=2.54ms min=129.28µs med=1.78ms max=34.99ms p(90)=5.34ms p(95)=6.79ms
     iterations...........: 3529    1174.135729/s
     latency..............: avg=1.98ms min=439µs    med=1.42ms max=27.77ms p(90)=4.11ms p(95)=5.12ms
     responseTime.........: avg=2.48ms min=495µs    med=1.72ms max=34.93ms p(90)=5.27ms p(95)=6.71ms
     vus..................: 3       min=3         max=3
     vus_max..............: 3       min=3         max=3


running (03.0s), 0/3 VUs, 3529 complete and 0 interrupted iterations
default ✓ [======================================] 3 VUs  3s
```

* `checks`, one check per iteration, verify `isSucceed` by default.
* `data_received` and `data_sent`, used by HTTP requests, useless for NebulaGraph.
* `iteration_duration`, time consuming for every iteration.
* `latency`, time consuming in NebulaGraph server.
* `responseTime`, time consuming in client.
* `vus`, concurrent virtual users.

In general

iteration_duration = responseTime + (time consuming for read data from csv)

responseTime = latency + (time consuming for network) + (client decode)

The `output.csv` saves data as below:

```bash
>head output.csv                                                                          

timestamp,nGQL,latency,responseTime,isSucceed,rows,firstRecord,errorMsg
1689576531,go 2 steps from 4194 over KNOWS yield dst(edge),4260,5151,true,1581,32985348838665,
1689576531,go 2 steps from 8333 over KNOWS yield dst(edge),4772,5772,true,2063,32985348833536,
1689576531,go 2 steps from 1129 over KNOWS yield dst(edge),5471,6441,true,1945,19791209302529,
1689576531,go 2 steps from 8698 over KNOWS yield dst(edge),3453,4143,true,1530,28587302322946,
1689576531,go 2 steps from 8853 over KNOWS yield dst(edge),4361,5368,true,2516,28587302324992,
1689576531,go 2 steps from 2199023256684 over KNOWS yield dst(edge),2259,2762,true,967,32985348833796,
1689576531,go 2 steps from 2199023262818 over KNOWS yield dst(edge),638,732,true,0,,
1689576531,go 2 steps from 10027 over KNOWS yield dst(edge),5182,6701,true,3288,30786325580290,
1689576531,go 2 steps from 2199023261211 over KNOWS yield dst(edge),2131,2498,true,739,32985348833794,
```

## Plugin Option

Pool options

---
| Key | Type | Default | Description |
|---|---|---|---|
|pool_policy|string|connection|'connection' or 'session', using which pool to test |
|address |string||NebulaGraph address, e.g. '192.168.8.6:9669,192.168.8.7:9669'|
|timeout_us|int|0|client connetion timeout, 0 means no timeout|
|idletime_us|int|0|client connection idle timeout, 0 means no timeout|
|max_size|int|400|max client connections in pool|
|min_size|int|0|min client connections in pool|
|username|string|root|NebulaGraph username|
|password|string|nebula|NebulaGraph password|
|space|string||NebulaGraph space|

Output options

---
| Key | Type | Default | Description |
|---|---|---|---|
|output|string||output file path|
|output_channel_size|int|10000| size of output channel|

CSV options

---
| Key | Type | Default | Description |
|---|---|---|---|
|csv_path|string||csv file path|
|csv_delimiter|string|,|delimiter of csv file|
|csv_with_header|bool|false|if ture, would ignore the first record|
|csv_channel_size|int|10000|size of csv reader channel|
|csv_data_limit|int|500000|would load [x] rows in memory, and then send to channel in loop|

Retry options

---
| Key | Type | Default | Description |
|---|---|---|---|
|retry_times|int|0|max retry times|
|retry_interval_us|int|0|interval duration for next retry|
|retry_timeout_us|int|0|retry timeout|

SSL options

---
| Key | Type | Default | Description |
|---|---|---|---|
|ssl_ca_pem_path|string||if it is not blank, would use SSL connection. ca pem path|
|ssl_client_pem_path|string||client pem path|
|ssl_client_key_path|string||client key path|

## Batch insert

It can also use `k6` for batch insert testing.

```bash
# create schema
cd example
nebula-console -addr 192.168.8.61 -port 9669 -u root -p nebula -f schema.ngql

# run testing
../k6 run nebula-test-insert.js -vu 10 -d 30s 

# by default, the batch size is 100, you can change it in `nebula-test-insert.js`
sed -i 's/let batchSize.*/let batchSize = 300/g' nebula-test-insert.js
../k6 run nebula-test-insert.js -vu 10 -d 30s 

```

## Test stages

It can specify the target number of VUs by k6 stages in options. e.g.

```js
import nebulaPool from 'k6/x/nebulagraph';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { sleep } from 'k6';

export let options = {
  stages: [
    { duration: '3m', target: 10 },
    { duration: '5m', target: 10 },
    { duration: '10m', target: 35 },
    { duration: '3m', target: 0 },
  ],
};

var latencyTrend = new Trend('latency');
var responseTrend = new Trend('responseTime');

```

The options means ramping up from 1 to 10 vus in 3 minutes, then running test with 10 vus in 5 minutes.

And then ramping up from 10 vus to 35 vus in 10 minutes.

Then ramping down from 35 vu3 to 0 in 3 minutes.

It is much useful when we test multiple scenarios.

please refer to [k6 options](https://k6.io/docs/using-k6/options/)

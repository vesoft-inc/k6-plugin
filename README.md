# xk6-nebula

This is a [k6](https://github.com/k6io/k6) extension using the [xk6](https://github.com/k6io/xk6) system.

Used to test [Nebula-Graph](https://github.com/vesoft-inc/nebula-graph).

## Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

Then:

1. Download `xk6`:

  ```bash
  go install github.com/k6io/xk6/cmd/xk6@latest
  ```

2. Build the binary:

  ```bash
  xk6 build --with github.com/HarrisChu/xk6-nebula@latest
  ```

## Example

```javascript
import nebulaPool from 'k6/x/nebulagraph';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { sleep } from 'k6';

var lantencyTrend = new Trend('latency');
var responseTrend = new Trend('responseTime');
// initial nebula connect pool
// by default the channel buffer size is 20000, you can reset it with

// var pool = nebulaPool.initWithSize("192.168.8.152:9669", 400, 4000);

var pool = nebulaPool.init("192.168.8.152:9669", 400);



// initial session for every vu
var session = pool.getSession("root", "nebula")
session.execute("USE sf1")


export function setup() {
  // config csv file
  pool.configCSV("person.csv", "|", false)
  // config output file, save every query information
  pool.configOutput("output.csv")
  sleep(1)
}

export default function (data) {
  // get csv data from csv file
  let d = session.getData()
  // d[0] means the first column data in the csv file
  let ngql = 'go 2 steps from ' + d[0] + ' over KNOWS '
  let response = session.execute(ngql)
  check(response, {
    "IsSucceed": (r) => r.isSucceed() === true
  });
  // add trend
lantencyTrend.add(response.getLatency());
responseTrend.add(response.getResponseTime());
};

export function teardown() {
  pool.close()
}

```

## Result

```bash
>./k6 run nebula-test.js -u 3 -d 3s                                                      

          /\      |‾‾| /‾‾/   /‾‾/
     /\  /  \     |  |/  /   /  /
    /  \/    \    |     (   /   ‾‾\
   /          \   |  |\  \ |  (‾)  |
  / __________ \  |__| \__\ \_____/ .io

INFO[0000] 2021/07/07 16:50:25 [INFO] begin init the nebula pool
INFO[0000] 2021/07/07 16:50:25 [INFO] connection pool is initialized successfully
INFO[0000] 2021/07/07 16:50:25 [INFO] finish init the pool
  execution: local
     script: nebula-test.js
     output: -

  scenarios: (100.00%) 1 scenario, 3 max VUs, 33s max duration (incl. graceful stop):
           * default: 3 looping VUs for 3s (gracefulStop: 30s)

INFO[0004] 2021/07/07 16:50:29 [INFO] begin close the nebula pool

running (04.1s), 0/3 VUs, 570 complete and 0 interrupted iterations
default ✓ [======================================] 3 VUs  3s
INFO[0004] 2021/07/07 16:50:29 [INFO] begin init the nebula pool
INFO[0004] 2021/07/07 16:50:29 [INFO] connection pool is initialized successfully
INFO[0004] 2021/07/07 16:50:29 [INFO] finish init the pool

     ✓ IsSucceed

     █ setup

     █ teardown

     checks...............: 100.00% ✓ 570        ✗ 0
     data_received........: 0 B     0 B/s
     data_sent............: 0 B     0 B/s
     iteration_duration...: avg=17.5ms       min=356.6µs med=11.44ms max=1s     p(90)=29.35ms p(95)=38.73ms
     iterations...........: 570     139.877575/s
     latency..............: avg=2986.831579  min=995     med=2663    max=18347  p(90)=4518.4  p(95)=5803
     responseTime.........: avg=15670.263158 min=4144    med=11326.5 max=108286 p(90)=28928.9 p(95)=38367.1
     vus..................: 3       min=0        max=3
     vus_max..............: 3       min=3        max=3
```

The `output.csv` saves data as below:

```bash
>head output.csv                                                                          

timestamp,nGQL,latency,responseTime,isSucceed,rows,errorMsg
1625647825,USE sf1,7808,10775,true,0,
1625647825,USE sf1,4055,7725,true,0,
1625647825,USE sf1,3431,10231,true,0,
1625647825,USE sf1,2938,5600,true,0,
1625647825,USE sf1,2917,5410,true,0,
1625647826,go 2 steps from 933 over KNOWS ,6022,24537,true,1680,
1625647826,go 2 steps from 1129 over KNOWS ,6141,25861,true,1945,
1625647826,go 2 steps from 4194 over KNOWS ,6317,26309,true,1581,
1625647826,go 2 steps from 8698 over KNOWS ,4388,22597,true,1530,
```

## Advanced usage

By default, all vus use the same channel to read the csv data.

You can change the strategy before `getSession` function.

As each vu uses a separate channel, you can reduce channel buffer size to save memory.

```js
// initial nebula connect pool, channel buffer size is 4000
var pool = nebulaPool.initWithSize("192.168.8.61:9669", 400, 4000);

// set csv strategy, 1 means each vu has a separate csv reader.
pool.configCsvStrategy(1)

// initial session for every vu
var session = pool.getSession("root", "nebula")
```

Please refer to [nebula-test-insert.js](./example/nebula-test-insert.js) for more details.

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

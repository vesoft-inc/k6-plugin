import nebulaPool from 'k6/x/nebulagraph';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { sleep } from 'k6';

var latencyTrend = new Trend('latency', true);
var responseTrend = new Trend('responseTime', true);
var rowSize = new Trend('rowSize');
// initial nebula connect pool
var pool = nebulaPool.initWithSize("192.168.8.61:9669,192.168.8.62:9669,192.168.8.63:9669", 400, 4000);

// set csv strategy, 1 means each vu has a separate csv reader.
pool.configCsvStrategy(1)

// initial session for every vu
var session = pool.getSession("root", "nebula")
session.execute("USE ldbc")
// export let options = {
// 	stages: [
// 		{ duration: '2s', target: 20 },
// 		{ duration: '2m', target: 20 },
// 		{ duration: '1m', target: 0 },
// 	],
// };

String.prototype.format = function () {
  var formatted = this;
  var data = arguments[0]

  formatted = formatted.replace(/\{(\d+)\}/g, function (match, key) {
    return data[key]
  })
  return formatted
};

export function setup() {
  // config csv file
  pool.configCSV("person.csv", "|", false)
  // config output file, save every query information
  pool.configOutput("output.csv")
  sleep(1)
}

export default function (data) {
  // get csv data from csv file
  let ngql = 'INSERT VERTEX Person(firstName, lastName, gender, birthday, creationDate, locationIP, browserUsed) VALUES '
  let batches = []
  let batchSize = 100
  // batch size 100
  for (let i = 0; i < batchSize; i++) {
    let d = session.getData();
    let value = "{0}:(\"{1}\",\"{2}\", \"{3}\", \"{4}\", datetime(\"{5}\"), \"{6}\", \"{7}\")".format(d)
    batches.push(value)
  }
  ngql = ngql + " " + batches.join(',')
  let response = session.execute(ngql)
  check(response, {
    "IsSucceed": (r) => r.isSucceed() === true
  });
  // add trend
  latencyTrend.add(response.getLatency() / 1000);
  responseTrend.add(response.getResponseTime() / 1000);
  rowSize.add(response.getRowSize());
};

export function teardown() {
  pool.close()
}

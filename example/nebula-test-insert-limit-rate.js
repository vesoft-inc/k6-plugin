// refer https://k6.io/blog/how-to-generate-a-constant-request-rate-with-the-new-scenarios-api/
// the example options means:
// 1. each second, 1000 iterations would be made.
// 2. max concurrent vu is 300.
// 3. last 30 seconds, so it would run 1000*30 = 30000 iterations.
// 4. batchSize is 1, so it would insert one recond per iteration.
import nebulaPool from 'k6/x/nebulagraph';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { sleep } from 'k6';

var lantencyTrend = new Trend('latency');
var responseTrend = new Trend('responseTime');
// initial nebula connect pool
var pool = nebulaPool.initWithSize("192.168.8.61:9669,192.168.8.62:9669,192.168.8.63:9669", 400, 4000);

// set csv strategy, 1 means each vu has a separate csv reader.
pool.configCsvStrategy(1)

// initial session for every vu
var session = pool.getSession("root", "nebula")
session.execute("USE ldbc")

// concurrent 300, and each second, 1000 iterations would be made.
export const options = {
	scenarios: {
		constant_request_rate: {
			executor: 'constant-arrival-rate',
			rate: 1000,
			timeUnit: '1s',
			duration: '30s',
			preAllocatedVUs: 300,
			maxVUs: 300,
		},
	},
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
	let batchSize = 1
	// batch size
	for (let i = 0; i < batchSize; i++) {
		let d = session.getData();
		let values = []
		// concat the insert value
		for (let index = 1; index < 8; index++) {
			let value = '"' + d[index] + '"'
			values.push(value)
		}
		let batch = d[0] + ":(" + values.join(",") + ")"
		batches.push(batch)
	}
	ngql = ngql + batches.join(',')
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



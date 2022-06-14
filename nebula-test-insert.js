import nebulaPool from 'k6/x/nebulagraph';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { sleep } from 'k6';

var lantencyTrend = new Trend('latency');
var responseTrend = new Trend('responseTime');
// initial nebula connect pool
//var pool = nebulaPool.initWithSize("192.168.8.61:9669,192.168.8.62:9669,192.168.8.63:9669", 400, 4000);
var pool = nebulaPool.initWithSize("192.168.15.8:9669", 400, 4000);

// set csv strategy, 1 means each vu has a separate csv reader.
pool.configCsvStrategy(1)

// initial session for every vu
var session = pool.getSession("root", "nebula")
session.execute("USE sf10_harris")
// export let options = {
// 	stages: [
// 		{ duration: '2s', target: 20 },
// 		{ duration: '2m', target: 20 },
// 		{ duration: '1m', target: 0 },
// 	],
// };

export function setup() {
	// config csv file
	pool.configCSV("/data/nebula-bench/sf100/social_network/dynamic/person.csv", "|", false)
	// config output file, save every query information
	pool.configOutput("output.csv")
	sleep(1)
}

export default function (data) {
	// get csv data from csv file
	let ngql = 'INSERT VERTEX Person(firstName, lastName, gender, birthday, creationDate, locationIP, browserUsed) VALUES '
	let batches = []
	let batchSize = 300
	// batch size 100
	for (let i = 0; i < batchSize; i++) {
		let d = session.getData();
		let values = []
		// concat the insert value
		for (let index = 1; index < 8; index++) {
			let value = '"' + d[index] + '"'
			values.push(value)
		}
		values[4] = 'datetime(' + values[4] + ')'
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



// refer https://k6.io/blog/how-to-generate-a-constant-request-rate-with-the-new-scenarios-api/
// the example options means:
// 1. each second, 1000 iterations would be made.
// 2. max concurrent vu is 300.
// 3. last 30 seconds, so it would run 1000*30 = 30000 iterations.
// 4. batchSize is 1, so it would insert one record per iteration.
import nebulaPool from 'k6/x/nebulagraph';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

var latencyTrend = new Trend('latency', true);
var responseTrend = new Trend('responseTime', true);

var graph_option = {
	address: "192.168.8.6:10010",
	space: "sf1",
	csv_path: "person.csv",
	csv_delimiter: "|",
	csv_with_header: true,
	output: "output.csv"
};

nebulaPool.setOption(graph_option);
var pool = nebulaPool.init();
// initial session for every vu
var session = pool.getSession()

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

String.prototype.format = function() {
	var formatted = this;
	var data = arguments[0]

	formatted = formatted.replace(/\{(\d+)\}/g, function(match, key) {
		return data[key]
	})
	return formatted
};

export default function(data) {
	// get csv data from csv file
	let ngql = 'INSERT VERTEX Person(firstName, lastName, gender, birthday, creationDate, locationIP, browserUsed) VALUES '
	let batches = []
	let batchSize = 10
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
	latencyTrend.add(response.getLatency());
	responseTrend.add(response.getResponseTime());

};

export function teardown() {
	pool.close()
}

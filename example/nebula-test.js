import nebulaPool from 'k6/x/nebulagraph';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { sleep } from 'k6';

var latencyTrend = new Trend('latency');
var responseTrend = new Trend('responseTime');
// initial nebula connect pool
var pool = nebulaPool.init("192.168.8.152:9669", 400);
// initial session for every vu
var session = pool.getSession("root", "nebula")
session.execute("USE ldbc")

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
	latencyTrend.add(response.getLatency());
	responseTrend.add(response.getResponseTime());

};

export function teardown() {
	pool.close()
}



import nebulaPool from 'k6/x/nebulagraph';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { sleep } from 'k6';

var latencyTrend = new Trend('latency', true);
var responseTrend = new Trend('responseTime', true);

var graph_option = {
	address: "192.168.8.6:10010",
	space: "sf1",
	csv_path: "person.csv",
	csv_delimiter: "|",
	csv_with_header: true,
	output: "output.csv",
	ssl_ca_pem_path: "cert/test.ca.pem",
	ssl_client_pem_path: "cert/test.client.pem",
	ssl_client_key_path: "cert/test.client.key"
};

nebulaPool.setOption(graph_option);
var pool = nebulaPool.init();
// initial session for every vu
var session = pool.getSession()

export let options = {
	stages: [
		{ duration: '2s', target: 20 },
		{ duration: '2m', target: 20 },
		{ duration: '1m', target: 0 },
	],
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
	let d = session.getData()
	// d[0] means the first column data in the csv file
	let ngql = 'go 2 steps from {0} over KNOWS yield dst(edge)'.format(d)
	let response = session.execute(ngql)
	check(response, {
		"IsSucceed": (r) => r.isSucceed() === true
	});
	// add trend
	latencyTrend.add(response.getLatency() / 1000);
	responseTrend.add(response.getResponseTime() / 1000);

};

export function teardown() {
	pool.close()
}

import meta from 'k6/x/nebulameta';

let client = meta.open("192.168.15.31", 10510)

export default function(data) {
	client.auth("root", "nebula", "192.168.15.31:10210")	
};

export function teardown() {
	meta.close()
}

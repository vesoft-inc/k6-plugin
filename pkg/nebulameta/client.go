package nebulameta

import (
	"fmt"
	"net"
	"strconv"

	"github.com/vesoft-inc/fbthrift/thrift/lib/go/thrift"
	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
)

type module struct {
	clients []*metaClient
}

type metaClient struct {
	client *meta.MetaServiceClient
	host   string
	port   int
}

func New() *module {
	return &module{}
}

func (c *module) Open(host string, port int) (*metaClient, error) {
	client := &metaClient{
		host: host,
		port: port,
	}
	if err := client.open(); err != nil {
		return nil, err
	}
	c.clients = append(c.clients, client)
	return client, nil
}

func (c *metaClient) open() error {
	newAdd := net.JoinHostPort(c.host, strconv.Itoa(c.port))
	sock, err := thrift.NewSocket(thrift.SocketAddr(newAdd))
	if err != nil {
		return err
	}
	// Set transport
	bufferSize := 128 << 10
	bufferedTranFactory := thrift.NewBufferedTransportFactory(bufferSize)
	transport := thrift.NewHeaderTransport(bufferedTranFactory.GetTransport(sock))
	pf := thrift.NewHeaderProtocolFactory()

	c.client = meta.NewMetaServiceClientFactory(transport, pf)
	if err := transport.Open(); err != nil {
		return err
	}
	return nil
}

func (c *metaClient) Auth(username, password string, graphAddr string) error {
	req := meta.NewCreateSessionReq()
	req.User = []byte(username)
	h, p, err := net.SplitHostPort(graphAddr)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return err
	}
	req.GraphAddr = &nebula.HostAddr{
		Host: h,
		Port: int32(port),
	}
	resp, err := c.client.CreateSession(req)

	if err != nil {
		return err
	}
	if resp.GetCode() != nebula.ErrorCode_SUCCEEDED {
		return fmt.Errorf("auth failed, code: %d", resp.GetCode())
	}
	return nil
}

func (c *metaClient) Close() error {
	return c.client.Close()
}

func (m *module) Close() {
	for _, c := range m.clients {
		c.Close()
	}
}

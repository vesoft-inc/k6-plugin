package nebulagraph

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vesoft-inc/k6-plugin/pkg/common"
	"github.com/vesoft-inc/nebula-http-gateway/ccore/nebula"
	nerrors "github.com/vesoft-inc/nebula-http-gateway/ccore/nebula/errors"
	"github.com/vesoft-inc/nebula-http-gateway/ccore/nebula/types"
	"github.com/vesoft-inc/nebula-http-gateway/ccore/nebula/wrapper"
)

type (
	// GraphPool nebula connection pool
	GraphPool struct {
		DataChs           []chan common.Data
		OutoptCh          chan []string
		Version           string
		csvStrategy       csvReaderStrategy
		initialized       bool
		clients           []nebula.GraphClient
		channelBufferSize int
		hosts             []string
		mutex             sync.Mutex
		clientGetter      graphClientGetter
	}

	graphClientGetter func(endpoint, username, password string) (nebula.GraphClient, error)

	// GraphClient a wrapper for nebula client, could read data from DataCh
	GraphClient struct {
		Client   nebula.GraphClient
		Pool     *GraphPool
		DataCh   chan common.Data
		username string
		password string
	}

	// Response a wrapper for nebula resultset
	Response struct {
		*wrapper.ResultSet
		ResponseTime int32
		codeErr      nerrors.CodeError
	}

	csvReaderStrategy int

	output struct {
		timeStamp    int64
		nGQL         string
		latency      int64
		responseTime int32
		isSucceed    bool
		rows         int32
		errorMsg     string
	}
)

var _ common.IGraphClient = &GraphClient{}
var _ common.IGraphClientPool = &GraphPool{}

const (
	// AllInOne all the vus use the same DataCh
	AllInOne csvReaderStrategy = iota
	// Separate each vu has a seprate DataCh
	Separate
)

func formatOutput(o *output) []string {
	return []string{
		strconv.FormatInt(o.timeStamp, 10),
		o.nGQL,
		strconv.Itoa(int(o.latency)),
		strconv.Itoa(int(o.responseTime)),
		strconv.FormatBool(o.isSucceed),
		strconv.Itoa(int(o.rows)),
		o.errorMsg,
	}
}

var outputHeader []string = []string{
	"timestamp",
	"nGQL",
	"latency",
	"responseTime",
	"isSucceed",
	"rows",
	"errorMsg",
}

// NewNebulaGraph New for k6 initialization.
func NewNebulaGraph() *GraphPool {
	return &GraphPool{
		clientGetter: func(endpoint string, username, password string) (nebula.GraphClient, error) {
			// ccore just use the first host in list
			return nebula.NewGraphClient([]string{endpoint}, username, password)
		},
	}
}

// Init initializes nebula pool with address and concurrent, by default the buffersize is 20000
func (gp *GraphPool) Init(address string, concurrent int) (common.IGraphClientPool, error) {
	return gp.InitWithSize(address, concurrent, 20000)
}

// InitWithSize initializes nebula pool with channel buffer size
func (gp *GraphPool) InitWithSize(address string, concurrent int, chanSize int) (common.IGraphClientPool, error) {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	if gp.initialized {
		return gp, nil
	}
	var err error

	err = gp.initAndVerifyPool(address, concurrent, chanSize)
	if err != nil {
		return nil, err
	}
	gp.initialized = true
	return gp, nil
}

func (gp *GraphPool) initAndVerifyPool(address string, concurrent int, chanSize int) error {
	addrs := strings.Split(address, ",")
	for _, addr := range addrs {
		hostPort := strings.Split(addr, ":")
		if len(hostPort) != 2 {
			return fmt.Errorf("Invalid address: %s", addr)
		}
		_, err := strconv.Atoi(hostPort[1])
		if err != nil {
			return err
		}
		gp.hosts = append(gp.hosts, addr)
	}
	gp.clients = make([]nebula.GraphClient, 0)
	gp.channelBufferSize = chanSize
	gp.OutoptCh = make(chan []string, gp.channelBufferSize)
	return nil
}

// ConfigCsvStrategy sets csv reader strategy
func (gp *GraphPool) ConfigCsvStrategy(strategy int) {
	gp.csvStrategy = csvReaderStrategy(strategy)
}

// ConfigCSV makes the read csv file configuration
func (gp *GraphPool) ConfigCSV(path, delimiter string, withHeader bool) error {
	for _, dataCh := range gp.DataChs {
		reader := common.NewCsvReader(path, delimiter, withHeader, dataCh)
		if err := reader.ReadForever(); err != nil {
			return err
		}
	}
	return nil
}

// ConfigOutput makes the output file configuration, would write the execution outputs
func (gp *GraphPool) ConfigOutput(path string) error {
	writer := common.NewCsvWriter(path, ",", outputHeader, gp.OutoptCh)
	if err := writer.WriteForever(); err != nil {
		return err
	}
	return nil
}

// Close closes the nebula pool
func (gp *GraphPool) Close() error {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	if !gp.initialized {
		return nil
	}
	// gp.Log.Println("begin close the nebula pool")
	for _, s := range gp.clients {
		if s != nil {
			s.Close()
		}
	}
	gp.initialized = false
	return nil
}

// GetSession gets the session from pool
func (gp *GraphPool) GetSession(username, password string) (common.IGraphClient, error) {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	// balancer, ccore just use the first endpoint
	index := len(gp.clients) % len(gp.hosts)
	client, err := gp.clientGetter(gp.hosts[index], username, password)

	if gp.Version == "" {
		gp.Version = string(client.Version())
	}
	if err != nil {
		return nil, err
	}
	err = client.Open()
	if err != nil {
		return nil, err
	}

	gp.clients = append(gp.clients, client)
	s := &GraphClient{Client: client, Pool: gp}
	s.prepareCsvReader()

	return s, nil
}

func (gc *GraphClient) Open() error {
	return gc.Client.Open()
}
func (gc *GraphClient) Auth() error {
	_, err := gc.Client.Authenticate(gc.username, gc.password)
	return err
}
func (gc *GraphClient) Close() error {
	return gc.Client.Close()
}

func (gc *GraphClient) prepareCsvReader() error {
	np := gc.Pool

	if np.csvStrategy == AllInOne {
		if len(np.DataChs) == 0 {
			dataCh := make(chan common.Data, np.channelBufferSize)
			np.DataChs = append(np.DataChs, dataCh)
		}
		gc.DataCh = np.DataChs[0]
	} else {
		dataCh := make(chan common.Data, np.channelBufferSize)
		np.DataChs = append(np.DataChs, dataCh)
		gc.DataCh = dataCh
	}
	return nil
}

// GetData get data from csv reader
func (gc *GraphClient) GetData() (common.Data, error) {
	if gc.DataCh != nil && len(gc.DataCh) != 0 {
		if d, ok := <-gc.DataCh; ok {
			return d, nil
		}
	}
	return nil, fmt.Errorf("no Data at all")
}

// Execute executes nebula query
func (gc *GraphClient) Execute(stmt string) (common.IGraphResponse, error) {
	start := time.Now()
	resp, err := gc.Client.Execute([]byte(stmt))
	var (
		codeErr nerrors.CodeError
		ok      bool
		rows    int32
		rs      *wrapper.ResultSet
		latency int64
	)
	if err != nil {
		codeErr, ok = nerrors.AsCodeError(err)
		if !ok {
			return nil, err
		}
		rows = 0
		latency = 0
	} else {
		// no err, so the error code is ErrorCode_SUCCEEDED
		codeErr, _ = nerrors.AsCodeError(nerrors.NewCodeError(nerrors.ErrorCode_SUCCEEDED, ""))
		rs, _ = wrapper.GenResultSet(resp, gc.Client.Factory(), types.TimezoneInfo{})
		rows = int32(rs.GetRowSize())
		latency = resp.GetLatencyInUs()
	}

	responseTime := int32(time.Since(start) / 1000)
	// output
	if gc.Pool.OutoptCh != nil && len(gc.Pool.OutoptCh) != cap(gc.Pool.OutoptCh) {
		o := &output{
			timeStamp:    start.Unix(),
			nGQL:         stmt,
			latency:      latency,
			responseTime: responseTime,
			isSucceed:    codeErr.GetErrorCode() == nerrors.ErrorCode_SUCCEEDED,
			rows:         rows,
			errorMsg:     codeErr.GetErrorMsg(),
		}
		gc.Pool.OutoptCh <- formatOutput(o)

	}
	return &Response{ResultSet: rs, ResponseTime: responseTime, codeErr: codeErr}, nil
}

// GetResponseTime GetResponseTime
func (r *Response) GetResponseTime() int32 {
	return r.ResponseTime
}

// IsSucceed IsSucceed
func (r *Response) IsSucceed() bool {
	if r.codeErr != nil && r.codeErr.GetErrorCode() != nerrors.ErrorCode_SUCCEEDED {
		return false
	}
	return true
}

// GetLatency GetLatency
func (r *Response) GetLatency() int64 {
	if r.ResultSet != nil {
		return r.ResultSet.GetLatency()
	}
	return 0
}

// GetRowSize GetRowSize
func (r *Response) GetRowSize() int32 {
	return int32(r.ResultSet.GetRowSize())
}

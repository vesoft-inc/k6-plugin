package nebulagraph

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vesoft-inc/k6-plugin/pkg/common"
	graph "github.com/vesoft-inc/nebula-go/v3"
)

const EnvRetryTimes = "NEBULA_RETRY_TIMES"
const EnvRetryIntervalUs = "NEBULA_RETRY_INTERVAL_US"
const EnvRetryTimeoutUs = "NEBULA_RETRY_TIMEOUT_US"

type (
	// GraphPool nebula connection pool
	GraphPool struct {
		DataCh      chan common.Data
		OutputCh    chan []string
		initialized bool
		mutex       sync.Mutex
		csvReader   common.ICsvReader
		connPool    *graph.ConnectionPool
		sessPool    *graph.SessionPool
		clients     []common.IGraphClient
		graphOption *common.GraphOption
	}

	// GraphClient a wrapper for nebula client, could read data from DataCh
	GraphClient struct {
		Client *graph.Session
		Pool   *GraphPool
		DataCh chan common.Data
	}

	// Response a wrapper for nebula resultSet
	Response struct {
		*graph.ResultSet
		ResponseTime int32
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
		firstRecord  string
	}
)

var _ common.IGraphClient = &GraphClient{}
var _ common.IGraphClientPool = &GraphPool{}

const (
	// AllInOne read csv sequentially
	AllInOne csvReaderStrategy = iota
	// Separate read csv concurrently
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
		o.firstRecord,
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
	"firstRecord",
	"errorMsg",
}

// NewNebulaGraph New for k6 initialization.
func NewNebulaGraph() *GraphPool {
	return &GraphPool{}
}

// Init initializes nebula pool with address and concurrent, by default the bufferSize is 20000
func (gp *GraphPool) Init() (common.IGraphClientPool, error) {
	var (
		err error
	)
	if gp.initialized {
		return gp, nil
	}

	switch gp.graphOption.PoolPolicy {
	case string(common.ConnectionPool):
		err = gp.initConnectionPool()
	case string(common.SessionPool):
		err = gp.initSessionPool()
	default:
		return nil, fmt.Errorf("invalid pool policy: %s, need connection or session", gp.graphOption.PoolPolicy)
	}
	if err != nil {
		return nil, err
	}
	gp.initialized = true
	if gp.graphOption.Output != "" {
		channelBufferSize := gp.graphOption.OutputChannelSize
		gp.OutputCh = make(chan []string, channelBufferSize)
		writer := common.NewCsvWriter(gp.graphOption.Output, ",", outputHeader, gp.OutputCh)
		if err := writer.WriteForever(); err != nil {
			return nil, err
		}
	}
	if gp.graphOption.CsvPath != "" {
		gp.csvReader = common.NewCsvReader(
			gp.graphOption.CsvPath,
			gp.graphOption.CsvDelimiter,
			gp.graphOption.CsvWithHeader,
			gp.graphOption.CsvDataLimit,
		)
		gp.DataCh = make(chan common.Data, gp.graphOption.CsvChannelSize)
		if err := gp.csvReader.ReadForever(gp.DataCh); err != nil {
			return nil, err
		}
	}
	return gp, nil
}

func (gp *GraphPool) initConnectionPool() error {
	addr := gp.graphOption.Address
	hosts, err := gp.validate(addr)
	if err != nil {
		return err
	}
	gp.clients = make([]common.IGraphClient, 0, gp.graphOption.MaxSize)
	conf := graph.GetDefaultConf()
	conf.MaxConnPoolSize = gp.graphOption.MaxSize
	conf.MinConnPoolSize = gp.graphOption.MinSize
	conf.TimeOut = time.Duration(gp.graphOption.TimeoutUs) * time.Microsecond
	conf.IdleTime = time.Duration(gp.graphOption.IdleTimeUs) * time.Microsecond
	var sslConfig *tls.Config
	if gp.graphOption.SslCaPemPath != "" {
		var err error
		sslConfig, err = graph.GetDefaultSSLConfig(
			gp.graphOption.SslCaPemPath,
			gp.graphOption.SslClientPemPath,
			gp.graphOption.SslClientKeyPath)
		if err != nil {
			return err
		}
	}
	pool, err := graph.NewSslConnectionPool(hosts, conf, sslConfig, graph.DefaultLogger{})
	if err != nil {
		return err
	}
	gp.connPool = pool

	return nil
}

func (gp *GraphPool) initSessionPool() error {
	addr := gp.graphOption.Address
	hosts, err := gp.validate(addr)
	if err != nil {
		return err
	}
	var sslConfig *tls.Config
	if gp.graphOption.SslCaPemPath != "" {
		var err error
		sslConfig, err = graph.GetDefaultSSLConfig(
			gp.graphOption.SslCaPemPath,
			gp.graphOption.SslClientPemPath,
			gp.graphOption.SslClientKeyPath)
		if err != nil {
			return err
		}
	}
	conf, err := graph.NewSessionPoolConf(
		gp.graphOption.Username,
		gp.graphOption.Password,
		hosts,
		gp.graphOption.Space,
		graph.WithTimeOut(time.Duration(gp.graphOption.TimeoutUs)*time.Microsecond),
		graph.WithIdleTime(time.Duration(gp.graphOption.IdleTimeUs)*time.Microsecond),
		graph.WithMaxSize(gp.graphOption.MaxSize),
		graph.WithMinSize(gp.graphOption.MinSize),
		graph.WithSSLConfig(sslConfig),
	)
	if err != nil {
		return err
	}
	pool, err := graph.NewSessionPool(*conf, graph.DefaultLogger{})
	if err != nil {
		return err
	}
	gp.sessPool = pool

	return nil
}

func (gp *GraphPool) validate(address string) ([]graph.HostAddress, error) {
	var hosts []graph.HostAddress
	addrs := strings.Split(address, ",")
	if len(addrs) == 0 {
		return nil, fmt.Errorf("Invalid address: %s", address)
	}
	for _, addr := range addrs {
		hostAndPort := strings.Split(addr, ":")
		if len(hostAndPort) != 2 {
			return nil, fmt.Errorf("Invalid address: %s", addr)
		}
		port, err := strconv.Atoi(hostAndPort[1])
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, graph.HostAddress{
			Host: hostAndPort[0],
			Port: port,
		})
	}
	return hosts, nil
}

// Deprecated ConfigCsvStrategy sets csv reader strategy
func (gp *GraphPool) ConfigCsvStrategy(strategy int) {
	return
}

// Close closes the nebula pool
func (gp *GraphPool) Close() error {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	if !gp.initialized {
		return nil
	}
	for _, s := range gp.clients {
		if s != nil {
			s.Close()
		}
	}
	if gp.connPool != nil {
		gp.connPool.Close()
	}
	if gp.sessPool != nil {
		gp.sessPool.Close()
	}

	return nil
}

// GetSession gets the session from pool
func (gp *GraphPool) GetSession() (common.IGraphClient, error) {
	if gp.connPool != nil {
		gp.mutex.Lock()
		defer gp.mutex.Unlock()
		c, err := gp.connPool.GetSession(
			gp.graphOption.Username,
			gp.graphOption.Password,
		)
		if err != nil {
			return nil, err
		}
		_, err = c.Execute(fmt.Sprintf("USE %s", gp.graphOption.Space))
		if err != nil {
			return nil, err
		}
		s := &GraphClient{Client: c, Pool: gp, DataCh: gp.DataCh}
		gp.clients = append(gp.clients, s)
		return s, nil
	} else {
		s := &GraphClient{Client: nil, Pool: gp, DataCh: gp.DataCh}
		return s, nil
	}

}

func (gp *GraphPool) SetOption(option *common.GraphOption) error {
	if gp.graphOption != nil {
		return nil
	}
	gp.graphOption = common.MakeDefaultOption(option)
	if err := common.ValidateOption(gp.graphOption); err != nil {
		return err
	}
	bs, _ := json.Marshal(gp.graphOption)
	fmt.Printf("testing option: %s\n", bs)
	return nil
}

func (gc *GraphClient) Open() error {
	// nebula-go no need to open
	return nil
}

func (gc *GraphClient) Close() error {
	gc.Client.Release()
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

func (gc *GraphClient) executeRetry(stmt string) (*graph.ResultSet, error) {
	// retry only when execution error
	// if other errors, e.g. SemanticError, would return directly
	var (
		resp *graph.ResultSet
		err  error
	)
	start := time.Now()
	for i := 0; i < gc.Pool.graphOption.RetryTimes+1; i++ {
		if gc.Client != nil {
			resp, err = gc.Client.Execute(stmt)
		} else {
			resp, err = gc.Pool.sessPool.Execute(stmt)
		}
		if gc.Pool.graphOption.RetryIntervalUs != 0 &&
			time.Since(start).Microseconds() > int64(gc.Pool.graphOption.RetryTimeoutUs) {
			return resp, fmt.Errorf("retry timeout")
		}
		if err != nil {
			fmt.Println("execute error: ", err)
			continue
		}

		graphErr := resp.GetErrorCode()
		if graphErr == graph.ErrorCode_SUCCEEDED {
			return resp, nil
		}
		// only retry for execution error
		if graphErr != graph.ErrorCode_E_EXECUTION_ERROR {
			break
		}
		<-time.After(time.Duration(gc.Pool.graphOption.RetryIntervalUs) * time.Microsecond)
	}
	return resp, err
}

// Execute executes nebula query
func (gc *GraphClient) Execute(stmt string) (common.IGraphResponse, error) {
	start := time.Now()
	var (
		o      *output
		result common.IGraphResponse
	)
	resp, err := gc.executeRetry(stmt)
	if err != nil {
		// to summary the error, should validate the response is nil or not in js.
		o = &output{
			timeStamp:    start.Unix(),
			nGQL:         stmt,
			latency:      0,
			responseTime: 0,
			isSucceed:    false,
			rows:         0,
			errorMsg:     err.Error(),
			firstRecord:  "",
		}
		result = nil
	} else {
		o = &output{
			timeStamp:    start.Unix(),
			nGQL:         stmt,
			latency:      resp.GetLatency(),
			responseTime: int32(time.Since(start) / 1000),
			isSucceed:    resp.GetErrorCode() == graph.ErrorCode_SUCCEEDED,
			rows:         int32(resp.GetRowSize()),
			errorMsg:     resp.GetErrorMsg(),
			firstRecord:  "",
		}
		result = &Response{ResultSet: resp, ResponseTime: o.responseTime}
	}
	if gc.Pool.OutputCh == nil {
		return result, nil
	}

	if resp != nil {
		var fr []string
		columns := resp.GetColSize()
		if o.rows != 0 {
			r, err := resp.GetRowValuesByIndex(0)
			if err != nil {
				return nil, err
			}
			for i := 0; i < columns; i++ {
				v, err := r.GetValueByIndex(i)
				if err != nil {
					return nil, err
				}
				fr = append(fr, v.String())
			}
		}
		o.firstRecord = strings.Join(fr, "|")
	}

	select {
	case gc.Pool.OutputCh <- formatOutput(o):
	// abandon if the output chan is full.
	default:

	}
	return result, nil
}

// GetResponseTime GetResponseTime
func (r *Response) GetResponseTime() int32 {
	return r.ResponseTime
}

// IsSucceed IsSucceed
func (r *Response) IsSucceed() bool {
	if r.ResultSet == nil || r.ResultSet.GetErrorCode() != graph.ErrorCode_SUCCEEDED {
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
	if r.ResultSet != nil {
		return int32(r.ResultSet.GetRowSize())
	}
	return 0
}

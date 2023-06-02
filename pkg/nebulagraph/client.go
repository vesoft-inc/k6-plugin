package nebulagraph

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vesoft-inc/k6-plugin/pkg/common"
	graph "github.com/vesoft-inc/nebula-go/v3"
)

const EnvRetryTimes = "NEBULA_RETRY_TIMES"

type (
	// GraphPool nebula connection pool
	GraphPool struct {
		DataCh            chan common.Data
		OutputCh          chan []string
		Version           string
		csvStrategy       csvReaderStrategy
		initialized       bool
		pool              *graph.ConnectionPool
		clients           []common.IGraphClient
		channelBufferSize int
		hosts             []string
		mutex             sync.Mutex
		csvReader         common.ICsvReader
		retryTimes        int
	}

	// GraphClient a wrapper for nebula client, could read data from DataCh
	GraphClient struct {
		Client   *graph.Session
		Pool     *GraphPool
		DataCh   chan common.Data
		username string
		password string
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
func (gp *GraphPool) Init(address string, concurrent int) (common.IGraphClientPool, error) {
	return gp.InitWithSize(address, concurrent, 20000)
}

// InitWithSize initializes nebula pool with channel buffer size
func (gp *GraphPool) InitWithSize(address string, concurrent int, chanSize int) (common.IGraphClientPool, error) {
	var retryTimes int
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	if gp.initialized {
		return gp, nil
	}
	if os.Getenv(EnvRetryTimes) != "" {
		retryTimes, _ = strconv.Atoi(os.Getenv(EnvRetryTimes))
	}

	if retryTimes == 0 {
		retryTimes = 200
	}

	err := gp.initAndVerifyPool(address, concurrent, chanSize)
	if err != nil {
		return nil, err
	}
	gp.DataCh = make(chan common.Data, chanSize)
	gp.initialized = true
	gp.retryTimes = retryTimes

	return gp, nil
}

func (gp *GraphPool) initAndVerifyPool(address string, concurrent int, chanSize int) error {
	var hosts []graph.HostAddress
	addrs := strings.Split(address, ",")
	for _, addr := range addrs {
		hostAndPort := strings.Split(addr, ":")
		if len(hostAndPort) != 2 {
			return fmt.Errorf("Invalid address: %s", addr)
		}
		port, err := strconv.Atoi(hostAndPort[1])
		if err != nil {
			return err
		}
		hosts = append(hosts, graph.HostAddress{
			Host: hostAndPort[0],
			Port: port,
		})
	}

	gp.clients = make([]common.IGraphClient, 0, concurrent)
	conf := graph.GetDefaultConf()
	conf.MaxConnPoolSize = concurrent * 2
	pool, err := graph.NewConnectionPool(hosts, conf, graph.DefaultLogger{})
	if err != nil {
		return err
	}
	gp.pool = pool
	gp.channelBufferSize = chanSize
	gp.OutputCh = make(chan []string, gp.channelBufferSize)
	return nil
}

// Deprecated ConfigCsvStrategy sets csv reader strategy
func (gp *GraphPool) ConfigCsvStrategy(strategy int) {
	return
}

// ConfigCSV makes the read csv file configuration
func (gp *GraphPool) ConfigCSV(path, delimiter string, withHeader bool, opts ...interface{}) error {
	var (
		limit int = 500 * 10000
	)
	if gp.csvReader != nil {
		return nil
	}
	if len(opts) > 0 {
		l, ok := opts[0].(int)
		if ok {
			limit = l
		}
	}
	gp.csvReader = common.NewCsvReader(path, delimiter, withHeader, limit)

	if err := gp.csvReader.ReadForever(gp.DataCh); err != nil {
		return err
	}

	return nil
}

// ConfigOutput makes the output file configuration, would write the execution outputs
func (gp *GraphPool) ConfigOutput(path string) error {
	writer := common.NewCsvWriter(path, ",", outputHeader, gp.OutputCh)
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
	c, err := gp.pool.GetSession(username, password)
	if err != nil {
		return nil, err
	}
	s := &GraphClient{Client: c, Pool: gp, DataCh: gp.DataCh}
	gp.clients = append(gp.clients, s)
	return s, nil
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
	// retry only when leader changed
	// if other errors, e.g. SemanticError, would return directly
	var (
		resp *graph.ResultSet
		err  error
	)
	for i := 0; i < gc.Pool.retryTimes; i++ {
		resp, err = gc.Client.Execute(stmt)
		if err != nil {
			return nil, err
		}
		graphErr := resp.GetErrorCode()
		if graphErr != graph.ErrorCode_SUCCEEDED {
			if graphErr == graph.ErrorCode_E_EXECUTION_ERROR {
				<-time.After(100 * time.Millisecond)
				continue
			}
			return resp, nil
		}
		return resp, nil
	}
	// still leader changed
	fmt.Printf("retry %d times, but still error: %s, return directly\n", gc.Pool.retryTimes, resp.GetErrorMsg())
	return resp, nil
}

// Execute executes nebula query
func (gc *GraphClient) Execute(stmt string) (common.IGraphResponse, error) {
	start := time.Now()
	var (
		rows    int32
		latency int64
	)
	resp, err := gc.executeRetry(stmt)
	if err != nil {
		return nil, err
	}

	rows = int32(resp.GetRowSize())
	latency = resp.GetLatency()

	responseTime := int32(time.Since(start) / 1000)
	// output
	if gc.Pool.OutputCh != nil {
		var fr []string
		columns := resp.GetColSize()
		if rows != 0 {
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
		o := &output{
			timeStamp:    start.Unix(),
			nGQL:         stmt,
			latency:      latency,
			responseTime: responseTime,
			isSucceed:    resp.GetErrorCode() == graph.ErrorCode_SUCCEEDED,
			rows:         rows,
			errorMsg:     resp.GetErrorMsg(),
			firstRecord:  strings.Join(fr, "|"),
		}
		select {
		case gc.Pool.OutputCh <- formatOutput(o):
		// abandon if the output chan is full.
		default:
		}

	}
	return &Response{ResultSet: resp, ResponseTime: responseTime}, nil
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

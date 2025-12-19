package nebulagraph5

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vesoft-inc/k6-plugin/pkg/common"

	nebula "github.com/vesoft-inc/nebula-go/v5"
	"github.com/vesoft-inc/nebula-go/v5/pkg/types"
)

type (
	// GraphPool nebula connection pool
	GraphPool struct {
		mutex             sync.Mutex
		DataCh            chan common.Data
		OutputCh          chan []string
		Version           string
		csvStrategy       csvReaderStrategy
		initialized       bool
		pool              types.Pool
		clients           []*GraphClient
		channelBufferSize int
		Hosts             []string
		csvReader         common.ICsvReader
		graphOption       *common.GraphOption
		maxLifeTime       time.Duration
		logger            logger
	}

	logger interface {
		Infof(msg string, args ...any)
		Warnf(msg string, args ...any)
		Debugf(msg string, args ...any)
		Errorf(msg string, args ...any)
	}

	graphClientGetter  func(endpoint, username, password string, timeout time.Duration) (types.Client, error)
	GraphClientFactory struct{}

	// GraphClient a wrapper for nebula client, could read data from DataCh
	GraphClient struct {
		Session  types.Client
		Pool     *GraphPool
		DataCh   chan common.Data
		username string
		password string
		since    time.Time
	}

	// Response a wrapper for nebula resultSet
	Response struct {
		ResultSet    types.Result
		err          error
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

func NewGraphClientFactory() *GraphClientFactory {
	return &GraphClientFactory{}
}

func (gf *GraphClientFactory) GetClient() *GraphClient {
	return &GraphClient{}
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
	gp.logger.Infof("testing option: %s\n", bs)
	return nil
}

// Init initializes nebula pool with address and concurrent, by default the bufferSize is 20000
func (gp *GraphPool) Init() (common.IGraphClientPool, error) {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	if gp.initialized {
		return gp, nil
	}
	if err := gp.validate(gp.graphOption.Address); err != nil {
		return nil, err
	}
	gp.Hosts = strings.Split(gp.graphOption.Address, ",")
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

	options := []nebula.PoolOptionsFn{
		nebula.WithPoolMaxOpenConns(gp.graphOption.MaxSize * 2),
		nebula.WithPoolMinOpenConns(gp.graphOption.MinSize),
	}
	if gp.graphOption.RetryTimeoutUs <= 0 {
		gp.graphOption.RetryTimeoutUs = math.MaxInt32
	}
	options = append(options, nebula.WithPoolRequestTimeout(time.Duration(gp.graphOption.TimeoutUs)*time.Microsecond))
	if gp.graphOption.SslCaPemPath != "" {
		options = append(options, nebula.WithPoolTLS(
			gp.graphOption.SslCaPemPath,
			gp.graphOption.SslClientPemPath,
			gp.graphOption.SslClientKeyPath,
			false,
		))
	}
	options = append(options, nebula.WithPoolMaxWait(1*time.Minute))
	pool, err := nebula.NewNebulaPool(
		gp.graphOption.Address,
		gp.graphOption.Username,
		gp.graphOption.Password,
		options...,
	)
	if err != nil {
		return nil, err
	}
	gp.maxLifeTime = getMaxLifeTime(gp.graphOption.ExtraOptions)
	gp.pool = pool
	gp.clients = make([]*GraphClient, 0)
	gp.initialized = true
	return gp, nil
}

func getMaxLifeTime(extra any) time.Duration {
	if extra == nil {
		return 0
	}
	m, ok := extra.(map[string]any)
	if !ok {
		return 0
	}
	if v, ok := m["max_life_time"]; ok {
		if f, ok := v.(int64); ok {
			return time.Duration(f) * time.Second
		}
	}
	return 0
}

func (gp *GraphPool) validate(address string) error {
	addrs := strings.Split(address, ",")
	if len(addrs) == 0 {
		return fmt.Errorf("Invalid address: %s", address)
	}
	for _, addr := range addrs {
		hostAndPort := strings.Split(addr, ":")
		if len(hostAndPort) != 2 {
			return fmt.Errorf("Invalid address: %s", addr)
		}
	}
	return nil
}

// Close closes the nebula pool
func (gp *GraphPool) Close() error {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	for _, client := range gp.clients {
		client.Close()
	}
	gp.pool.Close()
	return nil
}

// GetSession gets the session from pool
func (gp *GraphPool) GetSession() (common.IGraphClient, error) {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	if !gp.initialized {
		return nil, fmt.Errorf("GraphPool is not initialized, please call Init() first")
	}

	s := &GraphClient{Pool: gp, DataCh: gp.DataCh, since: time.Now()}
	gp.clients = append(gp.clients, s)
	return s, nil
}

func (gc *GraphClient) Open() error {
	return nil
}

func (gc *GraphClient) OpenAddress(address, username, password string, connectTimeout int) error {
	if gc.Session != nil {
		return fmt.Errorf("session already open")
	}
	connectTimeoutDuration := time.Duration(connectTimeout) * time.Second
	client, err := nebula.NewNebulaClient(address, username, password,
		nebula.WithClientConnectTimeout(connectTimeoutDuration),
	)
	if err != nil {
		return err
	}
	gc.Session = client
	return nil
}

func (gc *GraphClient) Close() error {
	if gc.Session == nil {
		return nil
	}
	gc.Session.Close()
	gc.Session = nil
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
	var (
		isSucceed  bool = true
		errMessage string
		err        error
		resp       types.Result
		rows       int32
		latency    int64
	)
	stmt = common.ProcessStmt(stmt)
	start := time.Now()
	if gc.Pool.maxLifeTime > 0 && time.Since(gc.since) > gc.Pool.maxLifeTime {
		gc.Pool.logger.Debugf("the client has been used for %v, which is longer than maxLifeTime %v, so we need to recreate it",
			time.Since(gc.since), gc.Pool.maxLifeTime)
		if gc.Session != nil {
			gc.Session.Close()
			gc.Pool.pool.PutClient(gc.Session)
			sess, err := gc.Pool.pool.GetClient()
			if err != nil {

				return nil, err
			}
			gc.Session = sess
		}
		gc.since = time.Now()
	}
	resp, err = gc.executeWithRetry(stmt)

	if err != nil {
		isSucceed = false
		errMessage = err.Error()
	} else {
		rows = int32(resp.RowSize())
		latency = resp.Summary().TotalServerTimeUs()
	}
	var fr []string
	var values []*nebula.NullValue
	var anyValues []any
	if rows != 0 {
		// print the first row of the result
		values = make([]*nebula.NullValue, 0, len(resp.Columns()))
		anyValues = make([]any, 0, len(resp.Columns()))
		for _ = range resp.Columns() {
			value := &nebula.NullValue{}
			values = append(values, value)
			anyValues = append(anyValues, value)
		}

		if err := resp.Scan(anyValues...); err != nil {
			return nil, err
		}

		for _, v := range values {
			if !v.Valid || v.Data == nil {
				fr = append(fr, "NULL")
			} else {
				fr = append(fr, v.Data.String())
			}
		}
	}
	//TODO could add a flag to just decode the first row
	if rows != 0 {
		for resp.HasNext() {
			if err := resp.Scan(anyValues...); err != nil {
				return nil, err
			}
		}
	}
	responseTime := int32(time.Since(start) / 1000)
	// output
	if gc.Pool.OutputCh != nil {
		o := &output{
			timeStamp:    start.Unix(),
			nGQL:         stmt,
			latency:      latency,
			responseTime: responseTime,
			isSucceed:    isSucceed,
			rows:         rows,
			errorMsg:     errMessage,
			firstRecord:  strings.Join(fr, "|"),
		}
		select {
		case gc.Pool.OutputCh <- formatOutput(o):
		// abandon if the output chan is full.
		default:
			gc.Pool.logger.Warnf("output channel is full, abandon the output: %v\n", o)
		}
	}
	return &Response{ResultSet: resp, ResponseTime: responseTime, err: err}, nil
}

func (gc *GraphClient) executeWithRetry(stmt string) (types.Result, error) {
	var (
		err  error
		resp types.Result
	)
	retryTimeout := time.Duration(gc.Pool.graphOption.RetryTimeoutUs) * time.Microsecond
	if retryTimeout <= 0 {
		retryTimeout = math.MaxInt64
	}
	start := time.Now()
	for i := 0; i < gc.Pool.graphOption.RetryTimes+1; i++ {
		if time.Now().Sub(start) > retryTimeout {
			return nil, fmt.Errorf("execute statement timeout: %s, timeout: %v", stmt, retryTimeout)
		}
		if i > 0 {
			gc.Pool.logger.Warnf("execute statement failed, retry %d time, error: %s\n", i, err.Error())
		}
		resp, err = gc.execute(stmt)
		if err == nil {
			return resp, nil
		} else {
			gc.Session.Close()
			gc.Pool.pool.PutClient(gc.Session)
		}
		time.Sleep(time.Duration(gc.Pool.graphOption.RetryIntervalUs) * time.Microsecond)
	}
	return nil, err
}

func (gc *GraphClient) execute(stmt string) (types.Result, error) {
	if gc.Session == nil || gc.Session.IsClosed() {
		sess, err := gc.Pool.pool.GetClient()
		if err != nil {
			return nil, err
		}
		gc.Session = sess
	}
	resp, err := gc.Session.Execute(stmt)
	if err != nil {
		return nil, fmt.Errorf("execute statement failed: %s, error: %w", stmt, err)
	}
	return resp, nil
}

// GetResponseTime GetResponseTime
func (r *Response) GetResponseTime() int32 {
	return r.ResponseTime
}

// IsSucceed IsSucceed
func (r *Response) IsSucceed() bool {
	if r.err != nil {
		return false
	}

	return true
}

func (r *Response) GetLatency() int64 {
	if r.ResultSet != nil {
		return r.ResultSet.Summary().TotalServerTimeUs()
	}
	return 0
}

// GetRowSize GetRowSize
func (r *Response) GetRowSize() int32 {
	if r.ResultSet != nil {
		return int32(r.ResultSet.RowSize())
	}
	return 0
}

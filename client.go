package nebulagraph

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	nebula "github.com/vesoft-inc/nebula-go"
	graph "github.com/vesoft-inc/nebula-go/nebula/graph"
)

type (
	HostAddress struct {
		Host string
		Port int
	}
	// NebulaPool nebula connection pool
	NebulaPool struct {
		Host              *HostAddress
		DataChs           []chan Data
		OutoptCh          chan []string
		Version           string
		csvStrategy       csvReaderStrategy
		initialized       bool
		sessions          []*nebula.GraphClient
		channelBufferSize int
		sslconfig         *sslConfig
		mutex             sync.Mutex
	}

	// NebulaSession a wrapper for nebula session, could read data from DataCh
	NebulaSession struct {
		Session *nebula.GraphClient
		Pool    *NebulaPool
		DataCh  chan Data
	}

	// Response a wrapper for nebula resultset
	Response struct {
		ResultSet    *graph.ExecutionResponse
		ResponseTime int32
	}

	csvReaderStrategy int

	sslConfig struct {
		rootCAPath     string
		certPath       string
		privateKeyPath string
	}

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

// New for k6 initialization.
func New() *NebulaPool {
	return &NebulaPool{
		initialized: false,
		Version:     version,
	}
}

// NewSSLConfig return sslConfig
func (np *NebulaPool) NewSSLConfig(rootCAPath, certPath, privateKeyPath string) {
	np.sslconfig = &sslConfig{
		rootCAPath:     rootCAPath,
		certPath:       certPath,
		privateKeyPath: privateKeyPath,
	}
}

// Init init nebula pool with address and concurrent, by default the buffersize is 20000
func (np *NebulaPool) Init(address string, concurrent int) (*NebulaPool, error) {
	return np.InitWithSize(address, concurrent, 20000)
}

// InitWithSize init nebula pool with channel buffer size
func (np *NebulaPool) InitWithSize(address string, concurrent int, size int) (*NebulaPool, error) {
	np.mutex.Lock()
	defer np.mutex.Unlock()
	np.initialized = true
	var err error

	err = np.initAndVerifyPool(address, concurrent, size)
	if err != nil {
		return nil, err
	}
	np.initialized = true
	return np, nil
}

func (np *NebulaPool) initAndVerifyPool(address string, concurrent int, size int) error {
	hostPort := strings.Split(address, ":")
	if len(hostPort) != 2 {
		return fmt.Errorf("Invalid address: %s", address)
	}
	port, err := strconv.Atoi(hostPort[1])
	if err != nil {
		return err
	}
	host := hostPort[0]
	hostAddr := &HostAddress{Host: host, Port: port}
	np.Host = hostAddr
	np.sessions = make([]*nebula.GraphClient, concurrent)
	np.channelBufferSize = size
	np.OutoptCh = make(chan []string, np.channelBufferSize)
	return nil
}

// ConfigCsvStrategy set csv reader strategy
func (np *NebulaPool) ConfigCsvStrategy(strategy int) {
	np.csvStrategy = csvReaderStrategy(strategy)
}

// ConfigCSV config the csv file to be read
func (np *NebulaPool) ConfigCSV(path, delimiter string, withHeader bool) error {
	for _, dataCh := range np.DataChs {
		reader := NewCsvReader(path, delimiter, withHeader, dataCh)
		if err := reader.ReadForever(); err != nil {
			return err
		}
	}
	return nil
}

// ConfigOutput config the output file, would write the execution outputs
func (np *NebulaPool) ConfigOutput(path string) error {
	writer := NewCsvWriter(path, ",", outputHeader, np.OutoptCh)
	if err := writer.WriteForever(); err != nil {
		return err
	}
	return nil
}

// Close close the nebula pool
func (np *NebulaPool) Close() error {
	np.mutex.Lock()
	defer np.mutex.Unlock()
	if !np.initialized {
		return nil
	}
	for _, s := range np.sessions {
		if s != nil {
			s.Disconnect()
		}
	}
	np.initialized = false
	return nil
}

// GetSession get the session from pool
func (np *NebulaPool) GetSession(user, password string) (*NebulaSession, error) {
	addr := fmt.Sprintf("%s:%d", np.Host.Host, np.Host.Port)
	client, err := nebula.NewClient(addr)
	if err != nil {
		return nil, err
	}
	if err = client.Connect(user, password); err != nil {
		return nil, err
	}

	np.mutex.Lock()
	defer np.mutex.Unlock()
	np.sessions = append(np.sessions, client)
	s := &NebulaSession{Session: client, Pool: np}
	s.prepareCsvReader()

	return s, nil
}

func (s *NebulaSession) prepareCsvReader() error {
	np := s.Pool
	if np.csvStrategy == AllInOne {
		if len(np.DataChs) == 0 {
			dataCh := make(chan Data, np.channelBufferSize)
			np.DataChs = append(np.DataChs, dataCh)
		}
		s.DataCh = np.DataChs[0]
	} else {
		dataCh := make(chan Data, np.channelBufferSize)
		np.DataChs = append(np.DataChs, dataCh)
		s.DataCh = dataCh
	}
	return nil
}

// GetData get data from csv reader
func (s *NebulaSession) GetData() (Data, error) {
	if s.DataCh != nil && len(s.DataCh) != 0 {
		if d, ok := <-s.DataCh; ok {
			return d, nil
		}
	}
	return nil, fmt.Errorf("no Data at all")
}

// Execute execute nebula query
func (s *NebulaSession) Execute(stmt string) (*Response, error) {
	start := time.Now()
	rs, err := s.Session.Execute(stmt)
	// us
	responseTime := int32(time.Since(start) / 1000)
	if err != nil {
		return nil, err
	}

	// output
	if s.Pool.OutoptCh != nil && len(s.Pool.OutoptCh) != cap(s.Pool.OutoptCh) {
		o := &output{
			timeStamp:    start.Unix(),
			nGQL:         stmt,
			latency:      int64(rs.GetLatencyInUs()),
			responseTime: responseTime,
			isSucceed:    rs.GetErrorCode() == graph.ErrorCode_SUCCEEDED,
			rows:         int32(len(rs.GetRows())),
			errorMsg:     rs.GetErrorMsg(),
		}
		s.Pool.OutoptCh <- formatOutput(o)

	}

	return &Response{ResultSet: rs, ResponseTime: responseTime}, nil
}

// GetResponseTime GetResponseTime
func (r *Response) GetResponseTime() int32 {
	return r.ResponseTime
}

func (r *Response) IsSucceed() bool{
	return r.ResultSet.GetErrorCode() == graph.ErrorCode_SUCCEEDED
}

func (r *Response) GetLatency() int32{
	return r.ResultSet.GetLatencyInUs()
}

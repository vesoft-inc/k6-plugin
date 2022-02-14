package nebulagraph

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	nebula "github.com/vesoft-inc/nebula-go/v3"
)

type (
	// NebulaPool nebula connection pool
	NebulaPool struct {
		HostList          []nebula.HostAddress
		Pool              *nebula.ConnectionPool
		Log               nebula.Logger
		DataChs           []chan Data
		OutoptCh          chan []string
		csvStrategy       csvReaderStrategy
		initialized       bool
		sessions          []*nebula.Session
		channelBufferSize int
		sslconfig         *sslConfig
		mutex             sync.Mutex
	}

	// NebulaSession a wrapper for nebula session, could read data from DataCh
	NebulaSession struct {
		Session *nebula.Session
		Pool    *NebulaPool
		DataCh  chan Data
	}

	// Response a wrapper for nebula resultset
	Response struct {
		*nebula.ResultSet
		ResponseTime int32
	}

	csvReaderStrategy int

	sslConfig struct {
		rootCAPath     string
		certPath       string
		privateKeyPath string
	}

	// Data data in csv file
	Data []string

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
		Log:         nebula.DefaultLogger{},
		initialized: false,
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
	// k6 run in concurrent thread.
	if np.initialized {
		return np, nil
	}
	np.Log.Info("begin init the nebula pool")
	var (
		sslConfig *tls.Config
		err       error
		pool      *nebula.ConnectionPool
	)

	if np.sslconfig != nil {
		sslConfig, err = nebula.GetDefaultSSLConfig(
			np.sslconfig.rootCAPath,
			np.sslconfig.certPath,
			np.sslconfig.privateKeyPath,
		)
		if err != nil {
			return nil, err
		}
		// skip insecure verification for stress testing.
		sslConfig.InsecureSkipVerify = true
	}
	err = np.initAndVerifyPool(address, concurrent, size)
	if err != nil {
		return nil, err
	}
	conf := np.getDefaultConf(concurrent)
	if sslConfig != nil {
		pool, err = nebula.NewSslConnectionPool(np.HostList, *conf, sslConfig, np.Log)

	} else {
		pool, err = nebula.NewConnectionPool(np.HostList, *conf, np.Log)
	}

	if err != nil {
		return nil, err
	}
	np.Pool = pool
	np.Log.Info("finish init the pool")
	np.initialized = true
	return np, nil
}

func (np *NebulaPool) initAndVerifyPool(address string, concurrent int, size int) error {

	addrs := strings.Split(address, ",")
	var hosts []nebula.HostAddress
	for _, addr := range addrs {
		hostPort := strings.Split(addr, ":")
		if len(hostPort) != 2 {
			return fmt.Errorf("Invalid address: %s", addr)
		}
		port, err := strconv.Atoi(hostPort[1])
		if err != nil {
			return err
		}
		host := hostPort[0]
		hostAddr := nebula.HostAddress{Host: host, Port: port}
		hosts = append(hosts, hostAddr)
		np.HostList = hosts
	}
	np.sessions = make([]*nebula.Session, concurrent)
	np.channelBufferSize = size
	np.OutoptCh = make(chan []string, np.channelBufferSize)
	return nil
}

func (np *NebulaPool) getDefaultConf(concurrent int) *nebula.PoolConfig {
	conf := nebula.PoolConfig{
		TimeOut:         0,
		IdleTime:        0,
		MaxConnPoolSize: concurrent,
		MinConnPoolSize: 1,
	}
	return &conf
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
	np.Log.Info("begin close the nebula pool")
	for _, s := range np.sessions {
		if s != nil {
			s.Release()
		}
	}
	np.initialized = false
	return nil
}

// GetSession get the session from pool
func (np *NebulaPool) GetSession(user, password string) (*NebulaSession, error) {
	session, err := np.Pool.GetSession(user, password)
	if err != nil {
		return nil, err
	}
	np.mutex.Lock()
	defer np.mutex.Unlock()
	np.sessions = append(np.sessions, session)
	s := &NebulaSession{Session: session, Pool: np}
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
			latency:      rs.GetLatency(),
			responseTime: responseTime,
			isSucceed:    rs.IsSucceed(),
			rows:         int32(rs.GetRowSize()),
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

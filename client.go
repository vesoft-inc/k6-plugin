package nebulagraph

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	nebula "github.com/vesoft-inc/nebula-go/v2"
)

type Data []string

type Output struct {
	TimeStamp    int64
	NGQL         string
	Latency      int64
	ResponseTime int32
	IsSucceed    bool
	Rows         int32
	ErrorMsg     string
}

func formatOutput(o *Output) []string {
	return []string{
		strconv.FormatInt(o.TimeStamp, 10),
		o.NGQL,
		strconv.Itoa(int(o.Latency)),
		strconv.Itoa(int(o.ResponseTime)),
		strconv.FormatBool(o.IsSucceed),
		strconv.Itoa(int(o.Rows)),
		o.ErrorMsg,
	}
}

var OutputHeader []string = []string{
	"timestamp",
	"nGQL",
	"latency",
	"responseTime",
	"isSucceed",
	"rows",
	"errorMsg",
}

type Response struct {
	*nebula.ResultSet
	ResponseTime int32
}
type CSVReaderStrategy int

const (
	AllInOne CSVReaderStrategy = iota
	Separate
)

type NebulaPool struct {
	HostList          []nebula.HostAddress
	Pool              *nebula.ConnectionPool
	Log               nebula.Logger
	DataChs           []chan Data
	OutoptCh          chan []string
	Version           string
	csvStrategy       CSVReaderStrategy
	initialized       bool
	sessions          []*nebula.Session
	channelBufferSize int
	mutex             sync.Mutex
}

type NebulaSession struct {
	Session *nebula.Session
	Pool    *NebulaPool
	DataCh  chan Data
}

func New() *NebulaPool {
	return &NebulaPool{
		Log:         nebula.DefaultLogger{},
		initialized: false,
		Version:     version,
	}
}

func (np *NebulaPool) Init(address string, concurrent int) (*NebulaPool, error) {
	return np.InitWithSize(address, concurrent, 20000)

}

func (np *NebulaPool) InitWithSize(address string, concurrent int, size int) (*NebulaPool, error) {
	if np.initialized {
		return np, nil
	}
	np.Log.Info("begin init the nebula pool")
	np.sessions = make([]*nebula.Session, concurrent)
	np.channelBufferSize = size
	np.OutoptCh = make(chan []string, np.channelBufferSize)

	addrs := strings.Split(address, ",")
	var hosts []nebula.HostAddress
	for _, addr := range addrs {
		hostPort := strings.Split(addr, ":")
		if len(hostPort) != 2 {
			return nil, fmt.Errorf("Invalid address: %s", addr)
		}
		port, err := strconv.Atoi(hostPort[1])
		if err != nil {
			return nil, err
		}
		host := hostPort[0]
		hostAddr := nebula.HostAddress{Host: host, Port: port}
		hosts = append(hosts, hostAddr)

	}
	conf := nebula.PoolConfig{
		TimeOut:         0,
		IdleTime:        0,
		MaxConnPoolSize: concurrent,
		MinConnPoolSize: 1,
	}
	pool, err := nebula.NewConnectionPool(hosts, conf, np.Log)
	if err != nil {
		return nil, err
	}

	np.Log.Info("finish init the pool")
	np.Pool = pool
	np.initialized = true
	return np, nil
}

func (np *NebulaPool) ConfigCsvStrategy(strategy int) {
	np.csvStrategy = CSVReaderStrategy(strategy)
}

func (np *NebulaPool) ConfigCSV(path, delimiter string, withHeader bool) error {
	for _, dataCh := range np.DataChs {
		reader := NewCsvReader(path, delimiter, withHeader, dataCh)
		if err := reader.ReadForever(); err != nil {
			return err
		}
	}
	return nil
}

func (np *NebulaPool) ConfigOutput(path string) error {
	writer := NewCsvWriter(path, ",", OutputHeader, np.OutoptCh)
	if err := writer.WriteForever(); err != nil {
		return err
	}
	return nil
}

func (np *NebulaPool) Close() error {
	if !np.initialized {
		return nil
	}
	np.Log.Info("begin close the nebula pool")
	for _, s := range np.sessions {
		if s != nil {
			s.Release()
		}
	}
	np.Pool.Close()
	np.initialized = false
	return nil
}

func (np *NebulaPool) GetSession(user, password string) (*NebulaSession, error) {
	session, err := np.Pool.GetSession(user, password)
	if err != nil {
		return nil, err
	}
	np.mutex.Lock()
	defer np.mutex.Unlock()
	np.sessions = append(np.sessions, session)
	s := &NebulaSession{Session: session, Pool: np}
	s.PrepareCsvReader()

	return s, nil
}

func (s *NebulaSession) PrepareCsvReader() error {
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

func (s *NebulaSession) GetData() (Data, error) {
	if s.DataCh != nil && len(s.DataCh) != 0 {
		if d, ok := <-s.DataCh; ok {
			return d, nil
		}
	}
	return nil, fmt.Errorf("no Data at all")
}

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
		o := &Output{
			TimeStamp:    start.Unix(),
			NGQL:         stmt,
			Latency:      rs.GetLatency(),
			ResponseTime: responseTime,
			IsSucceed:    rs.IsSucceed(),
			Rows:         int32(rs.GetRowSize()),
			ErrorMsg:     rs.GetErrorMsg(),
		}
		s.Pool.OutoptCh <- formatOutput(o)

	}

	return &Response{ResultSet: rs, ResponseTime: responseTime}, nil
}

func (r *Response) GetResponseTime() int32 {
	return r.ResponseTime
}

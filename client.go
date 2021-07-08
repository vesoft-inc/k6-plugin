package nebulagraph

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	nebula "github.com/vesoft-inc/nebula-go/v2"
)

type Data []string
type Output struct {
	TimeStamp    int64
	NGQL         string
	Latency      int32
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

type NebulaPool struct {
	HostList    []nebula.HostAddress
	Pool        *nebula.ConnectionPool
	Log         nebula.Logger
	DataCh      chan Data
	OutoptCh    chan []string
	initialized bool
	Sessions    []*nebula.Session
}

type NebulaSession struct {
	Session *nebula.Session
	Pool    *NebulaPool
}

func New() *NebulaPool {
	return &NebulaPool{
		Log:         nebula.DefaultLogger{},
		initialized: false,
	}
}

func (np *NebulaPool) Init(address string, concurrent int) (*NebulaPool, error) {
	if np.initialized {
		return np, nil
	}
	np.Log.Info("begin init the nebula pool")
	np.Sessions = make([]*nebula.Session, concurrent)
	np.DataCh = make(chan Data, 20000)
	np.OutoptCh = make(chan []string, 20000)
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

func (np *NebulaPool) ConfigCSV(path, delimiter string, withHeader bool) error {
	reader := NewCsvReader(path, delimiter, withHeader, np.DataCh)
	if err := reader.ReadForever(); err != nil {
		return err
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

func (np *NebulaPool) GetData() (Data, error) {
	if len(np.DataCh) != 0 {
		if d, ok := <-np.DataCh; ok {
			return d, nil
		}
	}
	return nil, fmt.Errorf("no Data at all")
}

func (np *NebulaPool) Close() error {
	if !np.initialized {
		return nil
	}
	np.Log.Info("begin close the nebula pool")
	for _, s := range np.Sessions {
		if s != nil {
			s.Release()
		}
	}
	np.Pool.Close()
	np.initialized = false
	return nil
}

func (np *NebulaPool) GetSession(user, password string) (*NebulaSession, error) {

	if session, err := np.Pool.GetSession(user, password); err != nil {
		return nil, err
	} else {
		np.Sessions = append(np.Sessions, session)
		return &NebulaSession{Session: session, Pool: np}, nil
	}
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
	if len(s.Pool.OutoptCh) != cap(s.Pool.OutoptCh) {
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

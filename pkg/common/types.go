package common

import (
	"fmt"
)

type (
	// Data data in csv file
	Data []string

	// pool policy
	PoolPolicy string

	// IClient common client
	IClient interface {
		Open() error
		Close() error
	}

	// IClientPool common client pool
	IClientPool interface {
		Close() error
	}

	// IGraphClient graph client
	IGraphClient interface {
		IClient
		GetData() (Data, error)
		Execute(stmt string) (IGraphResponse, error)
	}

	// IGraphResponse graph response, just support some functions to user.
	IGraphResponse interface {
		IsSucceed() bool
		GetLatency() int64
		GetResponseTime() int32
		GetRowSize() int32
	}

	// IGraphClientPool graph client pool.
	IGraphClientPool interface {
		IClientPool
		GetSession() (IGraphClient, error)
		// Init initialize the poop with default channel bufferSize
		Init() (IGraphClientPool, error)
		SetOption(*GraphOption) error
	}

	ICsvReader interface {
		ReadForever(dataCh chan<- Data) error
	}

	GraphOption struct {
		PoolOption   `json:",inline"`
		OutputOption `json:",inline"`
		CsvOption    `json:",inline"`
		RetryOption  `json:",inline"`
		SSLOption    `json:",inline"`
		Http2Option  `json:",inline"`
	}

	PoolOption struct {
		PoolPolicy string `json:"pool_policy"`
		Address    string `json:"address"`
		TimeoutUs  int    `json:"timeout_us"`
		IdleTimeUs int    `json:"idletime_us"`
		MaxSize    int    `json:"max_size"`
		MinSize    int    `json:"min_size"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Space      string `json:"space"`
	}

	OutputOption struct {
		Output            string `json:"output"`
		OutputChannelSize int    `json:"output_channel_size"`
	}

	SSLOption struct {
		SslCaPemPath     string `json:"ssl_ca_pem_path"`
		SslClientPemPath string `json:"ssl_client_pem_path"`
		SslClientKeyPath string `json:"ssl_client_key_path"`
	}

	CsvOption struct {
		CsvPath        string `json:"csv_path"`
		CsvDelimiter   string `json:"csv_delimiter"`
		CsvWithHeader  bool   `json:"csv_with_header"`
		CsvChannelSize int    `json:"csv_channel_size"`
		CsvDataLimit   int    `json:"csv_data_limit"`
	}
	RetryOption struct {
		RetryTimes      int `json:"retry_times"`
		RetryIntervalUs int `json:"retry_interval_us"`
		RetryTimeoutUs  int `json:"retry_timeout_us"`
	}
	Http2Option struct {
		HttpEnable bool                `json:"http_enable"`
		HttpHeader map[string][]string `json:"http_header"`
	}
)

const (
	ConnectionPool PoolPolicy = "connection"
	SessionPool    PoolPolicy = "session"
)

func MakeDefaultOption(opt *GraphOption) *GraphOption {
	if opt == nil {
		return nil
	}
	if opt.PoolPolicy == "" {
		opt.PoolPolicy = string(ConnectionPool)
	}
	if opt.OutputChannelSize == 0 {
		opt.OutputChannelSize = 10000
	}
	if opt.CsvPath != "" && opt.CsvDelimiter == "" {
		opt.CsvDelimiter = ","
	}
	if opt.CsvChannelSize == 0 {
		opt.CsvChannelSize = 10000
	}
	if opt.CsvDataLimit == 0 {
		opt.CsvDataLimit = 500000
	}
	if opt.MaxSize == 0 {
		opt.MaxSize = 400
	}
	if opt.Username == "" {
		opt.Username = "root"
	}
	if opt.Password == "" {
		opt.Password = "nebula"
	}
	return opt
}

func ValidateOption(option *GraphOption) error {
	if option == nil {
		return nil
	}
	if option.Space == "" {
		return fmt.Errorf("space is empty")
	}
	if option.Address == "" {
		return fmt.Errorf("address is empty")
	}
	if option.SslCaPemPath != "" {
		if option.SslClientPemPath == "" || option.SslClientKeyPath == "" {
			return fmt.Errorf("ssl_client_pem_path or ssl_client_key_path is empty")
		}
	}

	return nil
}

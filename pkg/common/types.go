package common

type (
	// Data data in csv file
	Data []string

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

	// IGraphResponse graph response, just support 3 functions to user.
	IGraphResponse interface {
		IsSucceed() bool
		GetLatency() int64
		GetResponseTime() int32
		GetRowSize() int32
	}

	// IGraphClientPool graph client pool.
	IGraphClientPool interface {
		IClientPool
		GetSession(username, password string) (IGraphClient, error)
		// Init initialize the poop with default channel buffersize
		Init(address string, concurrent int) (IGraphClientPool, error)
		InitWithSize(address string, concurrent int, size int) (IGraphClientPool, error)
		ConfigCSV(path, delimiter string, withHeader bool) error
		ConfigOutput(path string) error
		// ConfigCsvStrategy, csv strategy
		// 0 means all vus have the same csv reader.
		// 1 means each vu has a separate csv reader.
		ConfigCsvStrategy(strategy int)
	}
)

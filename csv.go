package nebulagraph

import (
	"encoding/csv"
	"io"
	"os"
)

type CSVReader struct {
	Path       string
	Delimiter  string
	WithHeader bool
	DataCh     chan<- Data
}

type CSVWriter struct {
	Path      string
	Header    []string
	Delimiter string
	DataCh    <-chan []string
}

func NewCsvReader(path, delimiter string, withHeader bool, dataCh chan<- Data) *CSVReader {
	return &CSVReader{
		Path:       path,
		Delimiter:  delimiter,
		WithHeader: withHeader,
		DataCh:     dataCh,
	}

}

func NewCsvWriter(path, delimiter string, header []string, dataCh <-chan []string) *CSVWriter {
	return &CSVWriter{
		Path:      path,
		Delimiter: delimiter,
		Header:    header,
		DataCh:    dataCh,
	}
}

func (c *CSVReader) ReadForever() error {
	file, err := os.Open(c.Path)
	defer file.Close()
	if err != nil {
		return err
	}
	go func() {
		file, err := os.Open(c.Path)
		defer file.Close()
		if err != nil {
			return
		}
		reader := csv.NewReader(file)
		comma := []rune(c.Delimiter)
		if len(comma) > 0 {
			reader.Comma = comma[0]
		}
		var offset int64 = 0
		if c.WithHeader {
			offset = 1
		}
		file.Seek(offset, 0)

		for {
			row, err := reader.Read()
			if err == io.EOF {
				file.Seek(offset, 0)
				row, err = reader.Read()
			}
			if err != nil {
				return
			}
			c.DataCh <- row
		}
	}()
	return nil

}

func (c *CSVWriter) WriteForever() error {
	file, err := os.OpenFile(c.Path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	defer file.Close()
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	comma := []rune(c.Delimiter)
	if len(comma) > 0 {
		w.Comma = comma[0]
	}
	w.Write(c.Header)
	w.Flush()
	go func() {
		file, err := os.OpenFile(c.Path, os.O_APPEND|os.O_RDWR, 0644)
		defer file.Close()
		if err != nil {
			return
		}
		w := csv.NewWriter(file)
		comma := []rune(c.Delimiter)
		if len(comma) > 0 {
			w.Comma = comma[0]
		}

		for {
			w.Write(<-c.DataCh)
			w.Flush()
		}

	}()
	return nil
}

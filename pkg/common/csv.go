package common

import (
	"encoding/csv"
	"io"
	"os"
)

type (
	CSVReader struct {
		Path       string
		Delimiter  string
		WithHeader bool
		limit      int
	}

	CSVWriter struct {
		Path      string
		Header    []string
		Delimiter string
		DataCh    <-chan []string
	}
)

func NewCsvReader(path, delimiter string, withHeader bool, limit int) *CSVReader {
	return &CSVReader{
		Path:       path,
		Delimiter:  delimiter,
		WithHeader: withHeader,
		limit:      limit,
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

// ReadForever read the csv in slice first, and send to the data channel forever.
func (c *CSVReader) ReadForever(dataCh chan<- Data) error {
	lines := make([]Data, 0, c.limit)
	file, err := os.Open(c.Path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	reader := csv.NewReader(file)
	comma := []rune(c.Delimiter)
	if len(comma) > 0 {
		reader.Comma = comma[0]
	}
	if c.WithHeader {
		_, err := reader.Read()
		if err != nil {
			return err
		}
	}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		lines = append(lines, row)
		if len(lines) == c.limit {
			break
		}
	}

	go func() {
		index := 0
		for {
			if index == len(lines) {
				index = 0
			}
			dataCh <- lines[index]
			index++
		}
	}()
	return nil

}

func (c *CSVWriter) WriteForever() error {
	file, err := os.OpenFile(c.Path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	defer func() {
		_ = file.Close()
	}()
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	comma := []rune(c.Delimiter)
	if len(comma) > 0 {
		w.Comma = comma[0]
	}
	if err := w.Write(c.Header); err != nil {
		return err
	}
	w.Flush()
	go func() {
		file, err := os.OpenFile(c.Path, os.O_APPEND|os.O_RDWR, 0644)
		defer func() {
			_ = file.Close()
		}()
		if err != nil {
			return
		}
		w := csv.NewWriter(file)
		comma := []rune(c.Delimiter)
		if len(comma) > 0 {
			w.Comma = comma[0]
		}

		for {
			_ = w.Write(<-c.DataCh)
			w.Flush()
		}

	}()
	return nil
}

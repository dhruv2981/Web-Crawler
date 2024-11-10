package scrape

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/slotix/dataflowkit/errs"
	"github.com/slotix/dataflowkit/storage"
	"github.com/spf13/viper"
	"github.com/tealeg/xlsx"
)

const (
	COMMENT_INFO  = "Generated by Dataflow Kit. https://dataflowkit.com"
	GZIP_COMPRESS = "gz"
)

func getFilename(resultPath string, timestamp string, info encodeInfo) (ext string) {
	switch info.compressor {
	case GZIP_COMPRESS:
		ext = GZIP_COMPRESS
	default:
		ext = info.extension
	}
	return path.Join(resultPath, info.payloadMD5+"_"+timestamp+"."+ext)
}

func newEncodeWriter(ctx context.Context, e *encoder, info encodeInfo) (io.Writer, *os.File, error) {
	var w io.Writer
	resultPath := viper.GetString("RESULTS_DIR")
	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		os.Mkdir(resultPath, 0700)
	}
	timestamp := time.Now().Format("2006-01-02_15:04")
	sFileName := getFilename(resultPath, timestamp, info)
	fo, err := os.OpenFile(sFileName, os.O_CREATE|os.O_WRONLY, 0660)
	if err != nil {
		return nil, nil, err
	}
	switch info.compressor {
	case GZIP_COMPRESS:
		gw, _ := gzip.NewWriterLevel(fo, gzip.BestSpeed)
		gw.Name = info.payloadMD5 + "_" + timestamp + "." + info.extension
		gw.Comment = COMMENT_INFO
		w = gw
	default:
		w = fo
	}
	return w, fo, nil
}

type encoder interface {
	begin() string
	stringDelimiter() string
	encodeRecord(record map[string]interface{}) string
	finilize() string
}

// CSVEncoder transforms parsed data to CSV format.
type CSVEncoder struct {
	partNames []string
	comma     string
}

// JSONEncoder transforms parsed data to JSON format.
type JSONEncoder struct {
	JSONL bool
}

// XMLEncoder transforms parsed data to XML format.
type XMLEncoder struct {
}

type XLSXEncoder struct {
	partNames []string
}

func (e JSONEncoder) encode(ctx context.Context, w io.Writer, payloadMD5 string) error {
	storageType := viper.GetString("STORAGE_TYPE")
	s := storage.NewStore(storageType)

	if !e.JSONL {
		io.WriteString(w, "[")
	}

	reader := newStorageReader(&s, payloadMD5 /* , keys */)
	writeComma := false
	for {
		select {
		default:
			block, err := reader.Read()
			if err != nil {
				if err.Error() == errs.EOF {
					if !e.JSONL {
						io.WriteString(w, "]")
					}
					s.Close()
					return nil
				} else if err.Error() == errs.NextPage {
					//next page
					// if e.paginateResults {
					// 	w.WriteString("],[")
					// }
				} else {
					logger.Error(err.Error())
					continue
				}
			}
			if !e.JSONL && writeComma {
				io.WriteString(w, ",")
			}
			blockJSON, err := json.Marshal(block)
			if err != nil {
				logger.Error(err.Error())
			}
			if !writeComma && !e.JSONL {
				writeComma = !writeComma
			}
			w.Write(blockJSON)
			if e.JSONL {
				io.WriteString(w, "\n")
			}
		case <-ctx.Done():
			s.Close()
			return &errs.Cancel{}
		}
	}
}

func (e JSONEncoder) begin() string {
	begin := ""
	if !e.JSONL {
		begin = "["
	}
	return begin
}

func (e JSONEncoder) stringDelimiter() string {
	delimiter := ","
	if e.JSONL {
		delimiter = "\n"
	}
	return delimiter
}

func (e JSONEncoder) encodeRecord(record map[string]interface{}) string {
	buffer, err := json.Marshal(record)
	if err != nil {
		logger.Error(err.Error())
		return ""
	}
	return string(buffer)
}

func (e JSONEncoder) finilize() string {
	fin := ""
	if !e.JSONL {
		fin = "]"
	}
	return fin
}

type storageResultReader struct {
	storage    *storage.Store
	payloadMD5 string
	page       int
	keys       []int
	block      int
}

func newStorageReader(store *storage.Store, md5Hash string) *storageResultReader {
	reader := &storageResultReader{
		storage:    store,
		payloadMD5: md5Hash,
		block:      0,
		page:       0,
	}
	return reader
}

func (r *storageResultReader) Read() (map[string]interface{}, error) {
	var err error
	blockMap, err := r.getValue()
	if err != nil {
		// have to try get next page value
		r.page++
		blockMap, err = r.getValue()
		// if there is no value - EOF
		// else it is possible we have to return a error
		// to separate values in file
		// errs.ErrStorageResult{Err: errs.NextPage}
		if err != nil {
			return nil, &errs.ErrStorageResult{Err: errs.EOF}
		}
	}
	for field, value := range blockMap {
		if strings.Contains(field, "details") {
			details := []map[string]interface{}{}
			detailsReader := newStorageReader(r.storage, value.(string) /* , nil */)
			for {
				detailsBlock, detailsErr := detailsReader.Read()
				if detailsErr != nil {
					if detailsErr.Error() == errs.NextPage {

					} else if detailsErr.Error() == errs.EOF {
						break
					} else {
						// in a case of details "no such file or directory" error means, that
						// detail's selector(s) has not be found in a block
						// these can happens when there are few coresponding blocks within a page
						// but only some of them contains wanted selector(s)
						// so just go ahead
						continue
					}
				}
				details = append(details, detailsBlock)
				// we are just breaking here because we got all details recursively
				// if we will continue to read storage, next iteration will returns EOF
				// just to save a time break loop manually
				//break
			}
			if len(details) > 0 {
				if len(details) == 1 {
					blockMap[field] = details[0]
				} else {
					blockMap[field] = details
				}
			}
		}
	}
	r.block++
	return blockMap, err
}

func (r *storageResultReader) getValue() (map[string]interface{}, error) {
	key := fmt.Sprintf("%s-%d-%d", r.payloadMD5, r.page, r.block)
	blockJSON, err := (*r.storage).Read(storage.Record{
		Type: storage.INTERMEDIATE,
		Key:  key,
	})

	if err != nil {
		//logger.Sugar().Errorf(fmt.Sprintf(errs.NoKey, key))
		return nil, err //&errs.ErrStorageResult{Err: fmt.Sprintf(errs.NoKey, key)}
	}
	blockMap := make(map[string]interface{})
	err = json.Unmarshal(blockJSON, &blockMap)
	if err != nil {
		return nil, err
	}
	return blockMap, nil
}

func (e CSVEncoder) formatFieldValue(block *map[string]interface{}, fieldName string) string {
	formatedString := ""
	switch v := (*block)[fieldName].(type) {
	case string:
		formatedString = v
		if strings.Contains(formatedString, `"`) {
			formatedString = strings.Replace(formatedString, `"`, `""`, -1)
		}
		if strings.Contains(formatedString, ",") || strings.Contains(formatedString, "\n") {
			formatedString = fmt.Sprintf("\"%s\"", formatedString)
		}
	case []string:
		formatedString = strings.Join(v, ";")
	case int:
		formatedString = strconv.FormatInt(int64(v), 10)
	case []int:
		formatedString = intArrayToString(v, ";")
	case []float64:
		formatedString = floatArrayToString(v, ";")
	case float64:
		formatedString = strconv.FormatFloat(v, 'f', -1, 64)
	case nil:
		formatedString = ""
	case []interface{}:
		values := make([]string, len(v))
		for i, value := range v {
			if strings.Contains(value.(string), `"`) {
				value = strings.Replace(value.(string), `"`, `""`, -1)
			}
			values[i] = fmt.Sprint(value)
		}
		formatedString = strings.Join(values, ";")
		if strings.Contains(formatedString, ",") || strings.Contains(formatedString, "\n") {
			formatedString = fmt.Sprintf("\"%s\"", formatedString)
		}
	}
	return fmt.Sprintf("%s,", formatedString)
}

func (e CSVEncoder) begin() string {
	begin := ""
	for _, headerName := range e.partNames {
		begin += fmt.Sprintf("%s,", headerName)
	}
	begin = strings.TrimSuffix(begin, ",") + "\n"
	return begin
}

func (e CSVEncoder) stringDelimiter() string {
	return ""
}

func (e CSVEncoder) encodeRecord(record map[string]interface{}) string {
	recordString := ""
	for _, fieldName := range e.partNames {
		recordString += e.formatFieldValue(&record, fieldName)
	}
	recordString = strings.TrimSuffix(recordString, ",") + "\n"
	return recordString
}

func (e CSVEncoder) finilize() string {
	return ""
}

func (e XMLEncoder) writeXML(w io.Writer, block *map[string]interface{}) {
	for field, value := range *block {
		if strings.Contains(field, "details") {
			io.WriteString(w, fmt.Sprintf("<%s>", field))
			switch details := value.(type) {
			case map[string]interface{}:
				e.writeXML(w, &details)
			case []map[string]interface{}:
				for _, detail := range details {
					e.writeXML(w, &detail)
				}
			}
			io.WriteString(w, fmt.Sprintf("</%s>", field))
		} else {
			io.WriteString(w, fmt.Sprintf("<%s>", field))
			// have to escape predefined entities to obtain valid xml
			v, ok := value.(string)
			if ok {
				xml.Escape(w, []byte(v))
			} else {
				for i, val := range value.([]interface{}) {
					v, ok = val.(string)
					if ok {
						xml.Escape(w, []byte(v))
						if i < len(value.([]interface{}))-1 {
							io.WriteString(w, ";")
						}
					}
				}
			}
			io.WriteString(w, fmt.Sprintf("</%s>", field))
		}
	}
}

func (e XMLEncoder) begin() string {
	return `<?xml version="1.0" encoding="UTF-8"?><root>`
}

func (e XMLEncoder) stringDelimiter() string {
	return ""
}

func (e XMLEncoder) encodeRecord(record map[string]interface{}) string {
	recordString := ""
	w := bufio.NewWriter(bytes.NewBufferString(recordString))
	e.writeXML(w, &record)
	return recordString
}

func (e XMLEncoder) finilize() string {
	return "</root>"
}

func intArrayToString(a []int, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
	//return strings.Trim(strings.Join(strings.Split(fmt.Sprint(a), " "), delim), "[]")
	//return strings.Trim(strings.Join(strings.Fields(fmt.Sprint(a)), delim), "[]")
}

func floatArrayToString(a []float64, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
	//return strings.Trim(strings.Join(strings.Split(fmt.Sprint(a), " "), delim), "[]")
	//return strings.Trim(strings.Join(strings.Fields(fmt.Sprint(a)), delim), "[]")
}

func (e XLSXEncoder) encode(ctx context.Context, w io.Writer, payloadMD5 string) error {
	file := xlsx.NewFile()
	sh, err := file.AddSheet("sheet")
	if err != nil {
		return err
	}
	row := sh.AddRow()
	for _, headerName := range e.partNames {
		cell := row.AddCell()
		cell.SetString(headerName)
	}

	storageType := viper.GetString("STORAGE_TYPE")
	s := storage.NewStore(storageType)
	defer s.Close()
	reader := newStorageReader(&s, payloadMD5)
	csv := CSVEncoder{partNames: e.partNames}
	for {
		select {
		default:
			block, err := reader.Read()
			if err != nil {
				if err.Error() == errs.EOF {
					return file.Write(w)
				} else if err.Error() == errs.NextPage {
					//next page
				} else {
					logger.Error(err.Error())
					//we have to continue 'cause we still have other records
					continue
				}
			}
			row = sh.AddRow()
			for _, fieldName := range e.partNames {
				cell := row.AddCell()
				sString := csv.formatFieldValue(&block, fieldName)
				cell.SetString(sString)
			}
		case <-ctx.Done():
			return &errs.Cancel{}
		}
	}
	return nil
}

func (e XLSXEncoder) begin() string {
	return ""
}

func (e XLSXEncoder) stringDelimiter() string {
	return ""
}

func (e XLSXEncoder) encodeRecord(record map[string]interface{}) string {
	recordString := ""
	return recordString
}

func (e XLSXEncoder) finilize() string {
	return ""
}
package elasticsearch

import (
	"bytes"
	"encoding/json"
	"errors"
)

type ByteSize float64

const (
	_           = iota
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
)
const newline byte = 10

type BulkEntry interface {
	Operationer
	Identifier
	Documenter
}

var BulkBodyFull = errors.New("No more operations can be added")

// BulkBody creates valid bulk data to be used by ES _bulk requests.
// http://www.elasticsearch.org/guide/en/elasticsearch/reference/current/docs-bulk.html
type BulkBody struct {
	bytes.Buffer
	max  ByteSize
	done bool
}

type indexHeader struct {
	Name string `json:"_index"`
	Type string `json:"_type"`
	Id   string `json:"_id"`
}

func NewBulkBody(max ByteSize) *BulkBody {
	return &BulkBody{max: max}
}

// Add will write new bulk operations to the buffer. Returns BulkBodyFull when maxed out.
func (bulk *BulkBody) Add(v BulkEntry) error {
	// Clear done bool on resets
	if bulk.Len() == 0 && bulk.done {
		bulk.done = false
	}
	// Don't allow more additions if we are full
	if bulk.done {
		return BulkBodyFull
	}
	if ByteSize(bulk.Len()) >= bulk.max {
		bulk.Done()
		return BulkBodyFull
	}

	// First part is a header identifying what to do
	header := indexHeader{}
	if i, err := v.Index(); err != nil {
		return err
	} else {
		header.Name = i
	}
	if t, err := v.Type(); err != nil {
		return err
	} else {
		header.Type = t
	}
	if id, err := v.Id(); err != nil {
		return err
	} else {
		header.Id = id
	}
	action, err := v.Action()
	if err != nil {
		return err
	}
	headerJson, err := json.Marshal(map[string]interface{}{action: header})
	if err != nil {
		return err
	}

	// Then is the values that should be applied
	doc, err := v.Document()
	if err != nil {
		return err
	}
	// Updates needs to be wrapped with additional options
	if action == "update" {
		doc = map[string]interface{}{
			"doc":           doc,
			"doc_as_upsert": true,
		}
	}
	valuesJson, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	// Header, values and final delimeter is separated by newlines
	entry := bytes.Join([][]byte{headerJson, valuesJson, nil}, []byte{newline})
	_, err = (*bulk).Write(entry)

	return err
}

// Done will append the final byte to mark the end of a bulk body. Should be called after all
// operations has been added.
func (bulk *BulkBody) Done() error {
	if !bulk.done {
		if err := bulk.WriteByte(newline); err != nil {
			return err
		}
		bulk.done = true
	}
	return nil
}
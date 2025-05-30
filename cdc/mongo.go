package cdc

import (
	"encoding/binary"
	"encoding/hex"
)

type ObjectId struct {
	Value string `json:"$oid"`
}

func (o *ObjectId) UnmarshalBSON(data []byte) error {
	o.Value = hex.EncodeToString(data)
	return nil
}

type Date struct {
	Value int64 `json:"$date"`
}

func (o *Date) UnmarshalBSON(data []byte) error {
	o.Value = int64(binary.LittleEndian.Uint64(data))
	return nil
}

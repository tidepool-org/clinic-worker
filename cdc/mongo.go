package cdc

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ObjectId struct {
	Value string `json:"$oid"`
}

func (o *ObjectId) UnmarshalBSON(data []byte) error {
	var id primitive.ObjectID
	err := bson.Unmarshal(data, &id)
	if err != nil {
		return err
	}
	o.Value = id.Hex()
	return nil
}

type Date struct {
	Value int64 `json:"$date"`
}

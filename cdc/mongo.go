package cdc

type ObjectId struct {
	Value string `json:"$oid"`
}

type Date struct {
	Value int64 `json:"$date"`
}

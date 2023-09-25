package cdc

type Event[Document any] struct {
	Offset            int64                        `json:"-"`
	OperationType     string                       `json:"operationType"`
	FullDocument      *Document                    `json:"fullDocument"`
	UpdateDescription *UpdateDescription[Document] `json:"updateDescription"`
}

type UpdateDescription[Document any] struct {
	UpdatedFields *Document `json:"updatedFields"`
	RemovedFields []string  `json:"removedFields"`
}

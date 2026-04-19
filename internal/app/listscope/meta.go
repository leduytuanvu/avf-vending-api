package listscope

// CollectionMeta is standard pagination for operational list endpoints.
type CollectionMeta struct {
	Limit    int32 `json:"limit"`
	Offset   int32 `json:"offset"`
	Returned int   `json:"returned"`
	Total    int64 `json:"total"`
}

package produce

type BucketMessage struct {
	Type   string `json:"type"`
	Bucket string `json:"bucket"`
	Region string `json:"region"`
	Owner  string `json:"owner"`
}

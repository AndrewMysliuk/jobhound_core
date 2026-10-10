package schema

import "fmt"

// Bucket is the scoring outcome stored on a match.
type Bucket string

const (
	BucketPassed   Bucket = "PASSED"
	BucketRejected Bucket = "REJECTED"
)

func (b Bucket) String() string { return string(b) }

func (b Bucket) Equals(s string) bool { return string(b) == s }

func (b Bucket) Pointer() *Bucket { return &b }

func (b Bucket) FromValue(s string) (Bucket, error) {
	switch Bucket(s) {
	case BucketPassed, BucketRejected:
		return Bucket(s), nil
	default:
		return "", fmt.Errorf("unknown Bucket %q: valid values are %v", s, ValuesBucket())
	}
}

func ValuesBucket() []Bucket {
	return []Bucket{BucketPassed, BucketRejected}
}

func FromStringBucket(s string) (Bucket, error) {
	var z Bucket
	return z.FromValue(s)
}

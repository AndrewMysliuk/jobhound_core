package schema

import "fmt"

// Bucket is the scoring outcome stored on a match.
type Bucket string

const (
	BucketTop      Bucket = "TOP"
	BucketReview   Bucket = "REVIEW"
	BucketRejected Bucket = "REJECTED"
)

func (b Bucket) String() string { return string(b) }

func (b Bucket) Equals(s string) bool { return string(b) == s }

func (b Bucket) Pointer() *Bucket { return &b }

func (b Bucket) FromValue(s string) (Bucket, error) {
	switch Bucket(s) {
	case BucketTop, BucketReview, BucketRejected:
		return Bucket(s), nil
	default:
		return "", fmt.Errorf("unknown Bucket %q: valid values are %v", s, ValuesBucket())
	}
}

func ValuesBucket() []Bucket {
	return []Bucket{BucketTop, BucketReview, BucketRejected}
}

func FromStringBucket(s string) (Bucket, error) {
	var z Bucket
	return z.FromValue(s)
}

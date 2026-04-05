package schema

import "fmt"

// JobBucket is the coarse passed/failed outcome recorded for stage 2 or 3 (PATCH body and persistence).
type JobBucket string

const (
	JobBucketPassed JobBucket = "passed"
	JobBucketFailed JobBucket = "failed"
)

func (b JobBucket) String() string { return string(b) }

func (b JobBucket) Equals(s string) bool { return string(b) == s }

func (b JobBucket) Pointer() *JobBucket { return &b }

// FromValue parses s into a JobBucket, returning an error that lists valid values on failure.
func (b JobBucket) FromValue(s string) (JobBucket, error) {
	switch s {
	case string(JobBucketPassed):
		return JobBucketPassed, nil
	case string(JobBucketFailed):
		return JobBucketFailed, nil
	default:
		return "", fmt.Errorf("unknown JobBucket %q: valid values are %v", s, ValuesJobBucket())
	}
}

// ValuesJobBucket returns all valid JobBucket values.
func ValuesJobBucket() []JobBucket {
	return []JobBucket{JobBucketPassed, JobBucketFailed}
}

// FromStringJobBucket parses s into a JobBucket.
func FromStringJobBucket(s string) (JobBucket, error) {
	var z JobBucket
	return z.FromValue(s)
}

// Valid reports whether b is a known bucket value.
func (b JobBucket) Valid() bool {
	_, err := b.FromValue(string(b))
	return err == nil
}

// PatchJobBucketRequest is PATCH …/stages/{2|3}/jobs/{job_id} body.
type PatchJobBucketRequest struct {
	Bucket JobBucket `json:"bucket"`
}

// PatchJobBucketResponse is the 200 JSON body for PATCH …/stages/{2|3}/jobs/{job_id}.
type PatchJobBucketResponse struct {
	JobID  string    `json:"job_id"`
	Bucket JobBucket `json:"bucket"`
}

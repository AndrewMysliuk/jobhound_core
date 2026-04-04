package schema

// JobBucket is a coarse passed/failed outcome for stage 2 or 3 (PATCH body and persistence).
type JobBucket string

const (
	JobBucketPassed JobBucket = "passed"
	JobBucketFailed JobBucket = "failed"
)

// Valid reports whether b is a known coarse bucket (009 PATCH body).
func (b JobBucket) Valid() bool {
	return b == JobBucketPassed || b == JobBucketFailed
}

// PatchJobBucketRequest is PATCH …/stages/{2|3}/jobs/{job_id} body.
type PatchJobBucketRequest struct {
	Bucket JobBucket `json:"bucket"`
}

// PatchJobBucketResponse is the optional 200 JSON body when the implementation returns a body instead of 204 No Content.
type PatchJobBucketResponse struct {
	JobID  string    `json:"job_id"`
	Bucket JobBucket `json:"bucket"`
}

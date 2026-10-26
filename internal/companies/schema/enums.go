package schema

import "fmt"

// ATS is the board provider. Only the three with a working fetcher are pollable.
type ATS string

const (
	ATSGreenhouse ATS = "GREENHOUSE"
	ATSLever      ATS = "LEVER"
	ATSAshby      ATS = "ASHBY"
	// Recognised but not pollable. Stored so the company is not re-resolved forever
	// and so the bucket size says when a fourth fetcher pays.
	ATSWorkday         ATS = "WORKDAY"
	ATSTeamtailor      ATS = "TEAMTAILOR"
	ATSWorkable        ATS = "WORKABLE"
	ATSRecruitee       ATS = "RECRUITEE"
	ATSPersonio        ATS = "PERSONIO"
	ATSRippling        ATS = "RIPPLING"
	ATSSmartRecruiters ATS = "SMARTRECRUITERS"
	ATSOther           ATS = "OTHER"
)

func (a ATS) String() string { return string(a) }

func (a ATS) Equals(str string) bool { return string(a) == str }

func (a ATS) Pointer() *ATS { return &a }

func (a ATS) FromValue(str string) (ATS, error) {
	switch ATS(str) {
	case ATSGreenhouse, ATSLever, ATSAshby, ATSWorkday, ATSTeamtailor, ATSWorkable,
		ATSRecruitee, ATSPersonio, ATSRippling, ATSSmartRecruiters, ATSOther:
		return ATS(str), nil
	default:
		return "", fmt.Errorf("unknown ATS %q: valid values are %v", str, ValuesATS())
	}
}

// Pollable reports whether a fetcher exists for this ATS in this slice.
func (a ATS) Pollable() bool {
	switch a {
	case ATSGreenhouse, ATSLever, ATSAshby:
		return true
	default:
		return false
	}
}

func ValuesATS() []ATS {
	return []ATS{
		ATSGreenhouse,
		ATSLever,
		ATSAshby,
		ATSWorkday,
		ATSTeamtailor,
		ATSWorkable,
		ATSRecruitee,
		ATSPersonio,
		ATSRippling,
		ATSSmartRecruiters,
		ATSOther,
	}
}

func FromStringATS(str string) (ATS, error) {
	var z ATS
	return z.FromValue(str)
}

// ResolutionStatus is how far ATS discovery has got for a company.
type ResolutionStatus string

const (
	ResolutionStatusPending        ResolutionStatus = "PENDING"
	ResolutionStatusResolved       ResolutionStatus = "RESOLVED"
	ResolutionStatusUnresolved     ResolutionStatus = "UNRESOLVED"
	ResolutionStatusUnsupportedATS ResolutionStatus = "UNSUPPORTED_ATS"
)

func (s ResolutionStatus) String() string { return string(s) }

func (s ResolutionStatus) Equals(str string) bool { return string(s) == str }

func (s ResolutionStatus) Pointer() *ResolutionStatus { return &s }

func (s ResolutionStatus) FromValue(str string) (ResolutionStatus, error) {
	switch ResolutionStatus(str) {
	case ResolutionStatusPending, ResolutionStatusResolved, ResolutionStatusUnresolved, ResolutionStatusUnsupportedATS:
		return ResolutionStatus(str), nil
	default:
		return "", fmt.Errorf("unknown ResolutionStatus %q: valid values are %v", str, ValuesResolutionStatus())
	}
}

func ValuesResolutionStatus() []ResolutionStatus {
	return []ResolutionStatus{
		ResolutionStatusPending,
		ResolutionStatusResolved,
		ResolutionStatusUnresolved,
		ResolutionStatusUnsupportedATS,
	}
}

func FromStringResolutionStatus(str string) (ResolutionStatus, error) {
	var z ResolutionStatus
	return z.FromValue(str)
}

// ResolutionMethod is how the ATS slug was obtained.
type ResolutionMethod string

const (
	ResolutionMethodSeed           ResolutionMethod = "SEED"
	ResolutionMethodApplyURL       ResolutionMethod = "APPLY_URL"
	ResolutionMethodAggregatorPage ResolutionMethod = "AGGREGATOR_PAGE"
	ResolutionMethodCareersPage    ResolutionMethod = "CAREERS_PAGE"
)

func (m ResolutionMethod) String() string { return string(m) }

func (m ResolutionMethod) Equals(str string) bool { return string(m) == str }

func (m ResolutionMethod) Pointer() *ResolutionMethod { return &m }

func (m ResolutionMethod) FromValue(str string) (ResolutionMethod, error) {
	switch ResolutionMethod(str) {
	case ResolutionMethodSeed, ResolutionMethodApplyURL, ResolutionMethodAggregatorPage, ResolutionMethodCareersPage:
		return ResolutionMethod(str), nil
	default:
		return "", fmt.Errorf("unknown ResolutionMethod %q: valid values are %v", str, ValuesResolutionMethod())
	}
}

func ValuesResolutionMethod() []ResolutionMethod {
	return []ResolutionMethod{
		ResolutionMethodSeed,
		ResolutionMethodApplyURL,
		ResolutionMethodAggregatorPage,
		ResolutionMethodCareersPage,
	}
}

func FromStringResolutionMethod(str string) (ResolutionMethod, error) {
	var z ResolutionMethod
	return z.FromValue(str)
}

// PollState is whether the company is still in the poll loop.
type PollState string

const (
	PollStateActive   PollState = "ACTIVE"
	PollStateInactive PollState = "INACTIVE"
)

func (s PollState) String() string { return string(s) }

func (s PollState) Equals(str string) bool { return string(s) == str }

func (s PollState) Pointer() *PollState { return &s }

func (s PollState) FromValue(str string) (PollState, error) {
	switch PollState(str) {
	case PollStateActive, PollStateInactive:
		return PollState(str), nil
	default:
		return "", fmt.Errorf("unknown PollState %q: valid values are %v", str, ValuesPollState())
	}
}

func ValuesPollState() []PollState {
	return []PollState{PollStateActive, PollStateInactive}
}

func FromStringPollState(str string) (PollState, error) {
	var z PollState
	return z.FromValue(str)
}

// PollOutcome distinguishes the five answers a board can give. COMPLETE is the
// only one that may close vacancies.
type PollOutcome string

const (
	PollOutcomeComplete    PollOutcome = "COMPLETE"     // reached end of pagination; may close
	PollOutcomeNotModified PollOutcome = "NOT_MODIFIED" // 304; closes nothing
	PollOutcomePartial     PollOutcome = "PARTIAL"      // 200s but truncated; upserts, closes nothing
	PollOutcomeEmpty       PollOutcome = "EMPTY"        // 200 + empty list; success, slug stays verified
	PollOutcomeDeadSlug    PollOutcome = "DEAD_SLUG"    // 404
)

func (o PollOutcome) String() string { return string(o) }

func (o PollOutcome) Equals(str string) bool { return string(o) == str }

func (o PollOutcome) Pointer() *PollOutcome { return &o }

func (o PollOutcome) FromValue(str string) (PollOutcome, error) {
	switch PollOutcome(str) {
	case PollOutcomeComplete, PollOutcomeNotModified, PollOutcomePartial, PollOutcomeEmpty, PollOutcomeDeadSlug:
		return PollOutcome(str), nil
	default:
		return "", fmt.Errorf("unknown PollOutcome %q: valid values are %v", str, ValuesPollOutcome())
	}
}

func ValuesPollOutcome() []PollOutcome {
	return []PollOutcome{
		PollOutcomeComplete,
		PollOutcomeNotModified,
		PollOutcomePartial,
		PollOutcomeEmpty,
		PollOutcomeDeadSlug,
	}
}

func FromStringPollOutcome(str string) (PollOutcome, error) {
	var z PollOutcome
	return z.FromValue(str)
}

// FailureStage tags log lines with which stage of the companies path failed.
type FailureStage string

const (
	FailureStageDiscover FailureStage = "DISCOVER"
	FailureStageResolve  FailureStage = "RESOLVE"
	FailureStagePoll     FailureStage = "POLL"
)

func (s FailureStage) String() string { return string(s) }

func (s FailureStage) Equals(str string) bool { return string(s) == str }

func (s FailureStage) Pointer() *FailureStage { return &s }

func (s FailureStage) FromValue(str string) (FailureStage, error) {
	switch FailureStage(str) {
	case FailureStageDiscover, FailureStageResolve, FailureStagePoll:
		return FailureStage(str), nil
	default:
		return "", fmt.Errorf("unknown FailureStage %q: valid values are %v", str, ValuesFailureStage())
	}
}

func ValuesFailureStage() []FailureStage {
	return []FailureStage{
		FailureStageDiscover,
		FailureStageResolve,
		FailureStagePoll,
	}
}

func FromStringFailureStage(str string) (FailureStage, error) {
	var z FailureStage
	return z.FromValue(str)
}

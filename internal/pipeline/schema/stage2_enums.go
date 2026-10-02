package schema

import "fmt"

// RuleAction is the outcome when a stage-2 rule fires.
type RuleAction string

const (
	RuleActionReject  RuleAction = "reject"
	RuleActionFlag    RuleAction = "flag"
	RuleActionBoost   RuleAction = "boost"
	RuleActionPenalty RuleAction = "penalty"
)

func (a RuleAction) String() string { return string(a) }

func (a RuleAction) Equals(s string) bool { return string(a) == s }

func (a RuleAction) Pointer() *RuleAction { return &a }

func (a RuleAction) FromValue(s string) (RuleAction, error) {
	switch s {
	case string(RuleActionReject):
		return RuleActionReject, nil
	case string(RuleActionFlag):
		return RuleActionFlag, nil
	case string(RuleActionBoost):
		return RuleActionBoost, nil
	case string(RuleActionPenalty):
		return RuleActionPenalty, nil
	default:
		return "", fmt.Errorf("unknown RuleAction %q: valid values are %v", s, ValuesRuleAction())
	}
}

func ValuesRuleAction() []RuleAction {
	return []RuleAction{RuleActionReject, RuleActionFlag, RuleActionBoost, RuleActionPenalty}
}

func FromStringRuleAction(s string) (RuleAction, error) {
	var z RuleAction
	return z.FromValue(s)
}

func (a RuleAction) Valid() bool {
	_, err := a.FromValue(string(a))
	return err == nil
}

// RuleField selects which parsed listing field a rule evaluates.
type RuleField string

const (
	RuleFieldCountriesAllowed RuleField = "countries_allowed"
	RuleFieldPosition         RuleField = "position"
	RuleFieldTitle            RuleField = "title"
	RuleFieldBody             RuleField = "body"
	RuleFieldTitleBody        RuleField = "title_body"
	RuleFieldListing          RuleField = "listing"
)

func (f RuleField) String() string { return string(f) }

func (f RuleField) Equals(s string) bool { return string(f) == s }

func (f RuleField) Pointer() *RuleField { return &f }

func (f RuleField) FromValue(s string) (RuleField, error) {
	switch s {
	case string(RuleFieldCountriesAllowed):
		return RuleFieldCountriesAllowed, nil
	case string(RuleFieldPosition):
		return RuleFieldPosition, nil
	case string(RuleFieldTitle):
		return RuleFieldTitle, nil
	case string(RuleFieldBody):
		return RuleFieldBody, nil
	case string(RuleFieldTitleBody):
		return RuleFieldTitleBody, nil
	case string(RuleFieldListing):
		return RuleFieldListing, nil
	default:
		return "", fmt.Errorf("unknown RuleField %q: valid values are %v", s, ValuesRuleField())
	}
}

func ValuesRuleField() []RuleField {
	return []RuleField{
		RuleFieldCountriesAllowed, RuleFieldPosition,
		RuleFieldTitle, RuleFieldBody, RuleFieldTitleBody, RuleFieldListing,
	}
}

func FromStringRuleField(s string) (RuleField, error) {
	var z RuleField
	return z.FromValue(s)
}

func (f RuleField) Valid() bool {
	_, err := f.FromValue(string(f))
	return err == nil
}

// RuleOp is how rule values are matched against a field.
type RuleOp string

const (
	RuleOpAny      RuleOp = "any"
	RuleOpExcludes RuleOp = "excludes"
	RuleOpPhrase   RuleOp = "phrase"
)

func (o RuleOp) String() string { return string(o) }

func (o RuleOp) Equals(s string) bool { return string(o) == s }

func (o RuleOp) Pointer() *RuleOp { return &o }

func (o RuleOp) FromValue(s string) (RuleOp, error) {
	switch s {
	case string(RuleOpAny):
		return RuleOpAny, nil
	case string(RuleOpExcludes):
		return RuleOpExcludes, nil
	case string(RuleOpPhrase):
		return RuleOpPhrase, nil
	default:
		return "", fmt.Errorf("unknown RuleOp %q: valid values are %v", s, ValuesRuleOp())
	}
}

func ValuesRuleOp() []RuleOp {
	return []RuleOp{RuleOpAny, RuleOpExcludes, RuleOpPhrase}
}

func FromStringRuleOp(s string) (RuleOp, error) {
	var z RuleOp
	return z.FromValue(s)
}

func (o RuleOp) Valid() bool {
	_, err := o.FromValue(string(o))
	return err == nil
}

// RuleWhen controls whether a rule runs when a field is empty (countries_allowed + excludes).
type RuleWhen string

const (
	RuleWhenExplicit RuleWhen = "explicit"
	RuleWhenAlways   RuleWhen = "always"
)

func (w RuleWhen) String() string { return string(w) }

func (w RuleWhen) Equals(s string) bool { return string(w) == s }

func (w RuleWhen) Pointer() *RuleWhen { return &w }

func (w RuleWhen) FromValue(s string) (RuleWhen, error) {
	switch s {
	case string(RuleWhenExplicit), "":
		return RuleWhenExplicit, nil
	case string(RuleWhenAlways):
		return RuleWhenAlways, nil
	default:
		return "", fmt.Errorf("unknown RuleWhen %q: valid values are %v", s, ValuesRuleWhen())
	}
}

func ValuesRuleWhen() []RuleWhen {
	return []RuleWhen{RuleWhenExplicit, RuleWhenAlways}
}

func FromStringRuleWhen(s string) (RuleWhen, error) {
	var z RuleWhen
	return z.FromValue(s)
}

func (w RuleWhen) Valid() bool {
	_, err := w.FromValue(string(w))
	return err == nil
}

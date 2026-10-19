package schema

// PhraseMap lists which current phrase lines a facts field replaces for CONFLICT.
type PhraseMap struct {
	Field   FieldName
	Phrases []string
}

package pslog

// NonFiniteFloatPolicy controls how JSON emitters serialize NaN/+Inf/-Inf.
type NonFiniteFloatPolicy uint8

const (
	// NonFiniteFloatAsString emits non-finite floats as JSON strings:
	// "NaN", "+Inf", "-Inf". This is the default for backward compatibility.
	NonFiniteFloatAsString NonFiniteFloatPolicy = iota
	// NonFiniteFloatAsNull emits non-finite floats as JSON null.
	NonFiniteFloatAsNull
)

func normalizeNonFiniteFloatPolicy(policy NonFiniteFloatPolicy) NonFiniteFloatPolicy {
	switch policy {
	case NonFiniteFloatAsNull:
		return policy
	default:
		return NonFiniteFloatAsString
	}
}

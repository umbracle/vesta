package schema

// Field is a field in a record
type Field struct {
	Type Type

	// Required specifies if the field is required
	Required bool

	// Default value for the field
	Default interface{}

	// ForceNew signals whether a change in this field requires a restart
	ForceNew bool

	// Description of the field
	Description string

	// AllowedValues is the list of allowed values for the field
	AllowedValues []interface{}

	// Filters is the list of filters to apply to the field
	Filters []Filter

	// Arbitrary params per field
	Params map[string]string

	References *References
}

type References struct {
	Type               string `mapstructure:"type"`
	FilterCriteriaFunc string `mapstructure:"filter_criteria_fn"`
}

type Filter struct {
	Criteria string
	Value    string
}

func (s *Field) DefaultOrZero() interface{} {
	if s.Default != nil {
		return s.Default
	}
	return s.Type.Zero()
}

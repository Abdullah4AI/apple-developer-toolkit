package asc

// UnmarshalJSON keeps the original attribute shape for sparse app responses.
func (a *AppAttributes) UnmarshalJSON(data []byte) error {
	type alias AppAttributes
	decoded, api, err := unmarshalAPIAttributes[alias](data)
	if err != nil {
		return err
	}
	*a = AppAttributes(decoded)
	a.api = api
	return nil
}

// MarshalJSON preserves absent, null, and unknown API attributes while retaining
// caller edits and the existing shape of directly constructed AppAttributes.
func (a AppAttributes) MarshalJSON() ([]byte, error) {
	type alias AppAttributes
	return a.api.marshal(alias(a))
}

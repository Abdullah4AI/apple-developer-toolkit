package asc

// BuildBetaDetailAttributes describes build beta detail attributes.
type BuildBetaDetailAttributes struct {
	AutoNotifyEnabled  bool   `json:"autoNotifyEnabled,omitempty"`
	InternalBuildState string `json:"internalBuildState,omitempty"`
	ExternalBuildState string `json:"externalBuildState,omitempty"`

	api apiAttributes
}

// BuildBetaDetailsResponse is the response from build beta details list.
type BuildBetaDetailsResponse = Response[BuildBetaDetailAttributes]

// BuildBetaDetailResponse is the response from build beta detail endpoints.
type BuildBetaDetailResponse = SingleResponse[BuildBetaDetailAttributes]

// BuildBetaDetailUpdateAttributes describes updateable build beta detail attributes.
type BuildBetaDetailUpdateAttributes struct {
	AutoNotifyEnabled *bool `json:"autoNotifyEnabled,omitempty"`
}

// BuildBetaDetailUpdateData is the data portion of a build beta detail update.
type BuildBetaDetailUpdateData struct {
	Type       ResourceType                     `json:"type"`
	ID         string                           `json:"id"`
	Attributes *BuildBetaDetailUpdateAttributes `json:"attributes,omitempty"`
}

// BuildBetaDetailUpdateRequest is a request to update build beta details.
type BuildBetaDetailUpdateRequest struct {
	Data BuildBetaDetailUpdateData `json:"data"`
}

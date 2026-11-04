package platform

type CompositeRequest struct {
	AllOrNone          bool                  `json:"allOrNone"`
	CollateSubrequests *bool                 `json:"collateSubrequests,omitempty"`
	CompositeRequest   []CompositeSubrequest `json:"compositeRequest"`
}
type CompositeSubrequest struct {
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	ReferenceID string         `json:"referenceId"`
	Body        map[string]any `json:"body,omitempty"`
}
type CompositeResponse struct {
	CompositeResponse []CompositeSubresponse `json:"compositeResponse"`
}
type CompositeSubresponse struct {
	Body           any               `json:"body"`
	HTTPHeaders    map[string]string `json:"httpHeaders"`
	HTTPStatusCode int               `json:"httpStatusCode"`
	ReferenceID    string            `json:"referenceId"`
}

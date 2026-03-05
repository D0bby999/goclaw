package extractor

type ExtractionResult struct {
	JSONLD        []map[string]any `json:"json_ld,omitempty"`
	OpenGraph     *OpenGraphData   `json:"open_graph,omitempty"`
	SchemaOrg     []map[string]any `json:"schema_org,omitempty"`
	Tables        []Table          `json:"tables,omitempty"`
	SocialHandles *SocialHandles   `json:"social_handles,omitempty"`
}

type OpenGraphData struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	SiteName    string `json:"site_name"`
}

type Table struct {
	Headers []string            `json:"headers"`
	Rows    []map[string]string `json:"rows"`
}

type SocialHandles struct {
	Twitter   []string `json:"twitter,omitempty"`
	Instagram []string `json:"instagram,omitempty"`
	Facebook  []string `json:"facebook,omitempty"`
	LinkedIn  []string `json:"linkedin,omitempty"`
	Emails    []string `json:"emails,omitempty"`
	Phones    []string `json:"phones,omitempty"`
}

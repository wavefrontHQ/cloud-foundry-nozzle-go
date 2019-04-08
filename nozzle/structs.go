package nozzle

type AppInfo struct {
	Name  string `json:"name,omitempty"`
	Guid  string `json:"guid,omitempty"`
	Space string `json:"space,omitempty"`
	Org   string `json:"org,omitempty"`
}

package api

// HawkeyeAPI defines the interface for the Hawkeye API client.
// *Client satisfies this interface. TUI and tests can use mock implementations.
type HawkeyeAPI interface {
	Login(email, password string) (*LoginResponse, error)
	FetchUserInfo() (*UserSpec, error)
	NewSession(projectUUID string) (*NewSessionResponse, error)
	SessionList(projectUUID string, limit int, filters []PaginationFilter) (*SessionListResponse, error)
	SessionInspect(projectUUID, sessionUUID string) (*SessionInspectResponse, error)
	GetSessionSummary(projectUUID, sessionUUID string) (*GetSessionSummaryResponse, error)
	ProcessPromptStream(projectUUID, sessionUUID, prompt string, cb StreamCallback) error
	PutRating(projectUUID, sessionUUID string, itemIDs []RatingItemID, rating, reason string) error
	PromptLibrary(projectUUID string) (*PromptLibraryResponse, error)
	ListProjects() (*ListProjectResponse, error)
	GetIncidentReport() (*IncidentReportResponse, error)
	ListConnections(projectUUID string) (*ListConnectionsResponse, error)
	ListConnectionResources(connUUID string, limit int) (*ListResourcesResponse, error)
	AddConnection(req *AddConnectionRequest) (*AddConnectionResponse, error)
}

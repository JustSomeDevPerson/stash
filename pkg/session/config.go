package session

type ExternalAccessConfig interface {
	HasCredentials() bool
	GetDangerousAllowPublicWithoutAuth() bool
	GetSecurityTripwireAccessedFromPublicInternet() string
	IsNewSystem() bool
}

type SessionConfig interface {
	GetUsername() string
	GetAPIKey() string

	GetSessionStoreKey() []byte
	GetMaxSessionAge() int
	GetRestrictedSessionTimeout() int
	ValidateCredentials(username string, password string) bool
	ValidateRestrictedPassword(password string) bool
}

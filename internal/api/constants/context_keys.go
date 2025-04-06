package constants

// Context keys for validated requests
const (
	// Auth context keys
	ContextKeyLogin     = "login"
	ContextKeyRegister  = "register"

	// User context keys
	ContextKeyUpdateProfile = "updateProfile"
	ContextKeyUpdateUser    = "updateUser"
	ContextKeyUserID        = "userID"
	ContextKeyUser          = "user"

	// Tunnel context keys
	ContextKeyCreateTunnel = "createTunnel"
	ContextKeyUpdateTunnel = "updateTunnel"
)
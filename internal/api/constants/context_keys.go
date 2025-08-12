package constants

// Context keys for validated requests
const (
	// Auth context keys
	ContextKeyLogin       = "login"
	ContextKeyRegister    = "register"
	ContextKeyVerifyToken = "verifyToken"

	// User context keys
	ContextKeyUpdateProfile = "updateProfile"
	ContextKeyUpdateUser    = "updateUser"
	ContextKeyUserID        = "userID"
	ContextKeyUser          = "user"

	// Request body related keys
	ContextKeyBodyValidation = "body_validation"
	ContextKeyRawBody        = "raw_body"
	ContextKeyValidate       = "validate"
)

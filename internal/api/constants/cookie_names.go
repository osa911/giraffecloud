package constants

// Cookie names used in the application
const (
	// Authentication cookies
	CookieSession   = "session"    // Firebase session cookie (HttpOnly)
	CookieAuthToken = "auth_token" // API authentication token (HttpOnly)

	// Cookie paths
	CookiePathRoot = "/"      // Root path for cookies available throughout the site
	CookiePathAPI  = "/api"   // API path for cookies restricted to API requests

	// Cookie duration in seconds
	CookieDuration24h  = 86400  // 24 hours
	CookieDuration30d  = 2592000 // 30 days
	CookieDurationWeek = 604800  // 7 days
)
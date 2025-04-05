package firebase

import (
	"context"
	"fmt"
	"path/filepath"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

var (
	app        *firebase.App
	authClient *auth.Client
)

// InitializeFirebase initializes the Firebase Admin SDK
func InitializeFirebase() error {
	fmt.Println("==== Initializing Firebase ====")

	// Get the service account key file path
	serviceAccountKeyPath := filepath.Join("internal", "config", "firebase", "service-account.json")
	fmt.Println("Service account key path:", serviceAccountKeyPath)

	// Check if the file exists
	var opt option.ClientOption

	// Create Firebase app
	fmt.Println("Creating Firebase app instance...")
	opt = option.WithCredentialsFile(serviceAccountKeyPath)
	var err error
	app, err = firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return fmt.Errorf("failed to initialize Firebase app: %v", err)
	}
	fmt.Println("Firebase app created successfully")

	// Get Auth client
	fmt.Println("Getting Firebase Auth client...")
	authClient, err = app.Auth(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get Firebase Auth client: %v", err)
	}
	fmt.Println("Firebase Auth client initialized successfully")

	return nil
}

// GetAuthClient returns the Firebase Auth client
func GetAuthClient() *auth.Client {
	if authClient == nil {
		fmt.Println("WARNING: GetAuthClient called but authClient is nil")
	}
	return authClient
}

// VerifyToken verifies a Firebase ID token and returns the user ID
func VerifyToken(ctx context.Context, idToken string) (string, error) {
	fmt.Println("==== VerifyToken called ====")

	if authClient == nil {
		return "", fmt.Errorf("Firebase Auth client not initialized")
	}

	fmt.Println("Verifying token (first 10 chars):", idToken[:10]+"...")
	token, err := authClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		fmt.Printf("Failed to verify ID token: %v (token first 10 chars: %s...)\n", err, idToken[:10])
		return "", fmt.Errorf("failed to verify ID token: %v", err)
	}

	fmt.Println("Token verified successfully. UID:", token.UID)
	return token.UID, nil
}

// GetUserByUID retrieves a user by their Firebase UID
func GetUserByUID(ctx context.Context, uid string) (*auth.UserRecord, error) {
	if authClient == nil {
		return nil, fmt.Errorf("Firebase Auth client not initialized")
	}

	fmt.Println("Getting user by UID:", uid)
	user, err := authClient.GetUser(ctx, uid)
	if err != nil {
		fmt.Println("Failed to get user:", err)
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	fmt.Println("User retrieved successfully. Email:", user.Email)
	return user, nil
}
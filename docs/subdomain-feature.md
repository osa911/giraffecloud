# Auto-Generated Subdomain Feature

## Overview

This feature allows users without their own custom domain to use an auto-generated subdomain under the base domain (e.g., `happy-giraffe-9ix2a.giraffecloud.xyz`).

## Key Features

- **Deterministic Generation**: Same user always gets the same subdomain
- **Secure**: User IDs are hashed using HMAC-SHA256 with a secret, not reversible
- **Human-Friendly**: Format uses `{adjective}-{noun}-{encoded-hash}` (e.g., `happy-giraffe-9ix2a`)
- **One Per User**: Users can only have one auto-generated subdomain
- **Custom Domains Still Supported**: Users can still bring their own domains

## Environment Variables

### Required

```bash
# Secret used for HMAC hashing of user IDs in subdomain generation
# MUST be set in production. Use a strong, random value.
SUBDOMAIN_SECRET=your-very-secret-random-string-here

# Your client/frontend URL - used to extract the base domain for subdomains
# The domain is extracted automatically (e.g., giraffecloud.xyz from https://giraffecloud.xyz)
CLIENT_URL=https://giraffecloud.xyz
```

### Generation Command

```bash
# Generate a secure random secret
openssl rand -base64 32
```

## API Endpoints

### GET /api/v1/tunnels/free

Returns the available auto-generated subdomain for the authenticated user.

**Authentication**: Required

**Response**:

```json
{
  "domain": "happy-giraffe-9ix2a.giraffecloud.xyz",
  "available": true
}
```

**Error Cases**:

- 400: User already has an auto-generated tunnel
- 401: Not authenticated

### POST /api/v1/tunnels

Create a tunnel with either a custom domain or auto-generated domain.

**Authentication**: Required

**Request**:

```json
{
  "domain": "happy-giraffe-9ix2a.giraffecloud.xyz",
  "target_port": 3000
}
```

**Validation**:

- If domain is auto-generated (ends with the domain extracted from CLIENT_URL):
  - Verifies user doesn't already have an auto-generated tunnel
  - Verifies the domain matches the expected auto-generated domain for that user
- If domain is custom: no additional validation

## Implementation Details

### Subdomain Generation Algorithm

1. **Input**: User ID (uint32)
2. **Hash**: HMAC-SHA256(userID + SUBDOMAIN_SECRET)
3. **Word Selection**: First 8 bytes of hash select adjective and noun from word lists
4. **Encoding**: Next 6 bytes encoded to base36 for compact representation
5. **Format**: `{adjective}-{noun}-{encoded}.{domain-from-CLIENT_URL}`

### Word Lists

- **Adjectives**: 200+ words (happy, clever, swift, cosmic, etc.)
- **Nouns**: 200+ words (giraffe, cloud, mountain, phoenix, etc.)
- **Combinations**: 40,000+ unique word pairs
- **Plus Encoded Hash**: Ensures uniqueness even with collisions

### Security

- User IDs are NOT reversible from the subdomain
- HMAC-SHA256 with secret prevents rainbow table attacks
- Even with word list access, cannot determine user ID without secret

## Usage Flow

### Frontend Flow

1. **User wants auto-generated domain**:

   ```javascript
   // Call GET /api/v1/tunnels/free
   const response = await fetch("/api/v1/tunnels/free");
   const { domain } = await response.json();
   // Shows: "Your subdomain: happy-giraffe-9ix2a.giraffecloud.xyz"
   ```

2. **User creates tunnel**:

   ```javascript
   // Call POST /api/v1/tunnels with the domain
   await fetch("/api/v1/tunnels", {
     method: "POST",
     body: JSON.stringify({
       domain: "happy-giraffe-9ix2a.giraffecloud.xyz",
       target_port: 3000,
     }),
   });
   ```

3. **User wants custom domain**:
   ```javascript
   // Just create tunnel with their domain
   await fetch("/api/v1/tunnels", {
     method: "POST",
     body: JSON.stringify({
       domain: "example.com",
       target_port: 3000,
     }),
   });
   ```

## Database Schema

No schema changes required! The feature uses the existing `domain` field in the `tunnels` table.

Detection of auto-generated vs custom domains is done by checking if the domain ends with the domain extracted from `CLIENT_URL`.

## Testing

### Test Subdomain Generation

```go
import "github.com/osa911/giraffecloud/internal/utils"

// Test with a specific user ID
userID := uint32(12345)
subdomain := utils.GenerateSubdomainForUser(userID)
// Always returns same subdomain for userID 12345

// Test idempotency
subdomain2 := utils.GenerateSubdomainForUser(userID)
assert.Equal(subdomain, subdomain2)
```

### Test API Endpoints

```bash
# Get free subdomain
curl -X GET http://localhost:8080/api/v1/tunnels/free \
  -H "Cookie: session_token=your-token"

# Create tunnel with auto-generated domain
curl -X POST http://localhost:8080/api/v1/tunnels \
  -H "Content-Type: application/json" \
  -H "Cookie: session_token=your-token" \
  -d '{
    "domain": "happy-giraffe-9ix2a.giraffecloud.xyz",
    "target_port": 3000
  }'
```

## Troubleshooting

### "User already has an auto-generated subdomain"

User already created a tunnel with an auto-generated domain. They can:

- Delete the existing tunnel and create a new one
- Use a custom domain instead (unlimited custom domains allowed)

### Subdomain doesn't match expected format

The backend validates that auto-generated domains:

1. End with the domain extracted from CLIENT_URL
2. Match the expected domain for that specific user ID

This prevents users from creating tunnels with someone else's auto-generated subdomain.

## Future Enhancements

- Allow users to regenerate their subdomain (with cooldown period)
- Custom word list configuration
- Subdomain reservation system
- Vanity subdomain purchase option

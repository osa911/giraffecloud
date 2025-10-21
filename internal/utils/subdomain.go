package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"strings"
)

// Word lists for subdomain generation
var adjectives = []string{
	"happy", "clever", "swift", "bright", "calm", "eager", "gentle", "kind", "lively", "merry",
	"nice", "proud", "quiet", "brave", "witty", "zesty", "cool", "smart", "wise", "bold",
	"cosmic", "digital", "dynamic", "elegant", "fantastic", "global", "infinite", "jolly", "legendary", "magic",
	"mighty", "noble", "optimal", "perfect", "quantum", "radiant", "stellar", "ultra", "vibrant", "wonder",
	"agile", "astral", "blissful", "brilliant", "cheerful", "daring", "epic", "fearless", "graceful", "heroic",
	"ideal", "joyful", "keen", "lucky", "majestic", "natural", "optimistic", "peaceful", "royal", "serene",
	"super", "true", "unique", "valiant", "warm", "active", "amber", "azure", "bronze", "crystal",
	"diamond", "electric", "fire", "golden", "iron", "jade", "lunar", "mystic", "neon", "onyx",
	"pearl", "quartz", "ruby", "silver", "turbo", "vivid", "wild", "zen", "alpha", "beta",
	"delta", "epic", "free", "grand", "hyper", "ideal", "just", "keen", "level", "mega",
	"nova", "omega", "prime", "quick", "rapid", "solid", "tidal", "vital", "wave", "xenon",
	"zippy", "able", "best", "chic", "dear", "easy", "fair", "glad", "holy", "pure",
	"rare", "safe", "true", "vast", "wise", "young", "zeal", "ace", "big", "fit",
	"fun", "new", "top", "fab", "fly", "hip", "hot", "icy", "key", "mad",
	"max", "pro", "rad", "sky", "sun", "red", "blue", "green", "pink", "aqua",
	"amber", "coral", "cyber", "frost", "glow", "light", "mint", "ocean", "pixel", "storm",
	"thunder", "velvet", "winter", "summer", "spring", "autumn", "forest", "mountain", "river", "cloud",
	"star", "moon", "comet", "nebula", "galaxy", "planet", "solar", "lunar", "cosmic", "space",
	"rocket", "laser", "turbo", "nitro", "boost", "flash", "blaze", "spark", "sonic", "atomic",
	"charged", "powered", "strong", "fierce", "intense", "extreme", "ultimate", "supreme", "prime", "elite",
}

var nouns = []string{
	"giraffe", "cloud", "river", "mountain", "ocean", "forest", "desert", "valley", "lake", "island",
	"peak", "canyon", "beach", "meadow", "garden", "bridge", "tower", "castle", "harbor", "lighthouse",
	"compass", "anchor", "phoenix", "dragon", "eagle", "falcon", "hawk", "lion", "tiger", "bear",
	"wolf", "fox", "deer", "rabbit", "panda", "koala", "dolphin", "whale", "shark", "octopus",
	"star", "moon", "sun", "comet", "meteor", "planet", "galaxy", "nebula", "cosmos", "orbit",
	"rocket", "shuttle", "satellite", "station", "probe", "explorer", "voyager", "pioneer", "venture", "odyssey",
	"thunder", "lightning", "storm", "breeze", "wind", "rain", "snow", "frost", "ice", "fire",
	"flame", "spark", "blaze", "ember", "ash", "smoke", "vapor", "mist", "fog", "dew",
	"crystal", "diamond", "ruby", "pearl", "jade", "onyx", "quartz", "amber", "opal", "topaz",
	"emerald", "sapphire", "garnet", "turquoise", "coral", "marble", "granite", "bronze", "silver", "gold",
	"wave", "tide", "current", "stream", "creek", "rapids", "waterfall", "cascade", "spring", "fountain",
	"echo", "whisper", "song", "melody", "harmony", "rhythm", "beat", "pulse", "vibe", "flow",
	"dream", "vision", "hope", "wish", "quest", "journey", "path", "trail", "road", "route",
	"gate", "door", "portal", "arch", "vault", "chamber", "hall", "temple", "shrine", "sanctuary",
	"citadel", "fortress", "bastion", "keep", "spire", "dome", "pillar", "column", "obelisk", "monument",
	"horizon", "dawn", "dusk", "twilight", "aurora", "zenith", "apex", "summit", "crest", "ridge",
	"pixel", "byte", "code", "node", "link", "sync", "core", "hub", "nexus", "matrix",
	"cipher", "key", "lock", "vault", "shield", "guard", "sentinel", "warden", "keeper", "protector",
	"seeker", "finder", "hunter", "ranger", "scout", "guide", "mentor", "sage", "oracle", "mystic",
	"phoenix", "dragon", "griffin", "pegasus", "unicorn", "sphinx", "hydra", "kraken", "titan", "giant",
}

// getSubdomainSecret returns the secret for subdomain generation from environment
func getSubdomainSecret() string {
	secret := os.Getenv("SUBDOMAIN_SECRET")
	if secret == "" {
		// Fallback to a default for development - MUST be set in production
		secret = "default-development-secret-change-in-production"
	}
	return secret
}

// getBaseDomain returns the base domain for subdomain generation from CLIENT_URL
func getBaseDomain() string {
	clientURL := os.Getenv("CLIENT_URL")
	if clientURL == "" {
		// Fallback for development
		return "localhost"
	}

	// Remove protocol if present
	domain := strings.TrimPrefix(clientURL, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	// Remove www if present
	domain = strings.TrimPrefix(domain, "www.")
	// Remove port if present
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}

	return domain
}

// GenerateSubdomainForUser generates a deterministic subdomain for a given user ID
// Format: {adjective}-{noun}-{encoded-hash}.{base-domain}
// Example: happy-giraffe-9ix2a.giraffecloud.xyz
func GenerateSubdomainForUser(userID uint32) string {
	secret := getSubdomainSecret()
	baseDomain := getBaseDomain()

	// Create HMAC hash of userID with secret
	h := hmac.New(sha256.New, []byte(secret))
	userIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(userIDBytes, userID)
	h.Write(userIDBytes)
	hashBytes := h.Sum(nil)

	// Use first bytes of hash to select words (deterministic)
	adjectiveIndex := int(binary.BigEndian.Uint32(hashBytes[0:4])) % len(adjectives)
	nounIndex := int(binary.BigEndian.Uint32(hashBytes[4:8])) % len(nouns)

	// Use next bytes to create encoded hash (base36 for compact representation)
	hashNum := new(big.Int).SetBytes(hashBytes[8:14]) // Use 6 bytes for good uniqueness
	encodedHash := strings.ToLower(hashNum.Text(36))  // Convert to base36

	// Ensure minimum length for consistency (pad if needed)
	for len(encodedHash) < 6 {
		encodedHash = "0" + encodedHash
	}
	// Limit to 8 characters for readability
	if len(encodedHash) > 8 {
		encodedHash = encodedHash[:8]
	}

	// Build subdomain
	subdomain := fmt.Sprintf("%s-%s-%s", adjectives[adjectiveIndex], nouns[nounIndex], encodedHash)
	fullDomain := fmt.Sprintf("%s.%s", subdomain, baseDomain)

	return fullDomain
}

// IsAutoGeneratedDomain checks if a domain is auto-generated (ends with base domain)
func IsAutoGeneratedDomain(domain string) bool {
	baseDomain := getBaseDomain()
	return strings.HasSuffix(domain, "."+baseDomain)
}

// ExtractSubdomain extracts just the subdomain part from a full domain
// Example: "happy-giraffe-9ix2a.giraffecloud.xyz" -> "happy-giraffe-9ix2a"
func ExtractSubdomain(domain string) string {
	baseDomain := getBaseDomain()
	suffix := "." + baseDomain
	if strings.HasSuffix(domain, suffix) {
		return strings.TrimSuffix(domain, suffix)
	}
	return ""
}

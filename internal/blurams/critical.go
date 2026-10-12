package blurams

import (
	"crypto/hmac"
	"crypto/md5" // #nosec G501 -- the vendor protocol requires MD5 for this password transform
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var (
	directions     = map[string]int{"home": 0, "left": 1, "right": 2, "up": 3, "down": 4}
	featurePattern = regexp.MustCompile(`["']FEATURE["']\s*:\s*(\d+)`)
)

func passwordDigest(password string) string {
	sum := md5.Sum([]byte("Arcsoft_" + password)) // #nosec G401 -- compatibility with the vendor protocol
	return hex.EncodeToString(sum[:])
}

func requestSignature(accessKey, timestamp, path, secret, body, nonce string) string {
	seed := strings.Join([]string{accessKey, timestamp, path, nonce}, "|")
	innerMAC := hmac.New(sha256.New, []byte(seed))
	_, _ = innerMAC.Write([]byte(secret))
	inner := innerMAC.Sum(nil)

	bodySum := sha256.Sum256([]byte(body))
	outerMAC := hmac.New(sha256.New, inner)
	_, _ = outerMAC.Write(bodySum[:])
	return hex.EncodeToString(outerMAC.Sum(nil))
}

func capabilities(comment string) (bool, bool) {
	match := featurePattern.FindStringSubmatch(comment)
	if len(match) != 2 {
		return false, false
	}
	var bits int
	if _, err := fmt.Sscanf(match[1], "%d", &bits); err != nil {
		return false, false
	}
	return bits&0x10 != 0, bits&0x20 != 0
}

func normalizeMove(direction string, steps int) (string, int, error) {
	normalized := strings.ToLower(direction)
	value, ok := directions[normalized]
	if !ok {
		return "", 0, fmt.Errorf("direction must be home, left, right, up, or down")
	}
	if steps < 1 || steps > 5 {
		return "", 0, fmt.Errorf("steps must be between 1 and 5")
	}
	return normalized, value, nil
}

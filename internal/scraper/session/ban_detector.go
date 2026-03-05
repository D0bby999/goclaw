package session

import (
	"fmt"
	"strings"
)

// BanSignal reports whether a response indicates a ban or challenge.
type BanSignal struct {
	IsBanned bool
	Reason   string
}

// bannedStatusCodes lists HTTP status codes that indicate blocking.
var bannedStatusCodes = map[int]string{
	403: "HTTP 403 Forbidden",
	429: "HTTP 429 Too Many Requests",
	503: "HTTP 503 Service Unavailable",
}

// challengePatterns lists body substrings that indicate a bot challenge.
var challengePatterns = []string{
	"cf-browser-verification",
	"cf_chl_opt",
	"captcha",
	"g-recaptcha",
	"hcaptcha",
	"challenge-platform",
}

// DetectBan inspects the HTTP status code and response body for ban signals.
func DetectBan(statusCode int, body string) BanSignal {
	if reason, ok := bannedStatusCodes[statusCode]; ok {
		return BanSignal{IsBanned: true, Reason: reason}
	}

	lower := strings.ToLower(body)
	for _, pattern := range challengePatterns {
		if strings.Contains(lower, pattern) {
			return BanSignal{
				IsBanned: true,
				Reason:   fmt.Sprintf("challenge detected: %s", pattern),
			}
		}
	}

	return BanSignal{}
}

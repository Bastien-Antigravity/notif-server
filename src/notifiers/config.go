package notifiers

/*
ESSENTIAL PROCESS:
Provides configuration map key lookup utilities for notification senders.

DATA FLOW:
1. Receives provider configuration map and candidate keys.
2. Performs exact and case-insensitive lookups for configuration options.
3. Returns trimmed string value.

KEY PARAMETERS:
- conf: Provider configuration key-value map.
- keys: Ordered list of candidate key names to search.
*/

import "strings"

// -----------------------------------------------------------------------------

// getOption retrieves an option from the config map trying multiple case variations.
func getOption(conf map[string]string, keys ...string) string {
	if conf == nil {
		return ""
	}
	for _, key := range keys {
		if val, ok := conf[key]; ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	// Case-insensitive fallback
	for _, key := range keys {
		for k, v := range conf {
			if strings.EqualFold(k, key) && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

package audit

import (
	"encoding/json"
	"strings"
)

// sensitiveKeys adalah nama field yang nilainya tidak boleh tersimpan mentah.
var sensitiveKeys = map[string]bool{
	"password": true, "password_hash": true, "new_password": true, "current_password": true,
	"token": true, "access_token": true, "refresh_token": true, "id_token": true,
	"authorization": true, "pin": true, "otp": true, "secret": true,
	"card_number": true, "cvv": true, "cvc": true, "api_key": true,
}

// Redact mengubah nilai apa pun menjadi bentuk JSON-safe dengan field sensitif
// disamarkan. Nil tetap nil agar kolom JSONB tersimpan sebagai NULL.
func Redact(v any) any {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		// Fail closed: data yang tidak bisa dipastikan aman tidak disimpan.
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	return redactValue(decoded)
}

func redactValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if sensitiveKeys[strings.ToLower(k)] {
				out[k] = "[REDACTED]"
				continue
			}
			out[k] = redactValue(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = redactValue(val)
		}
		return out
	default:
		return v
	}
}

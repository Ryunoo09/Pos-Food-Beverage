package audit

import "testing"

func TestRedact_SensitiveFieldsStripped(t *testing.T) {
	in := map[string]any{
		"username": "owner",
		"password": "secret",
		"nested":   map[string]any{"access_token": "abc", "keep": "x"},
		"list":     []any{map[string]any{"pin": "1234", "name": "a"}},
	}
	out, ok := Redact(in).(map[string]any)
	if !ok {
		t.Fatalf("hasil bukan map: %T", Redact(in))
	}
	if out["username"] != "owner" {
		t.Fatalf("username berubah: %v", out["username"])
	}
	if out["password"] != "[REDACTED]" {
		t.Fatalf("password tidak disamarkan: %v", out["password"])
	}
	nested := out["nested"].(map[string]any)
	if nested["access_token"] != "[REDACTED]" || nested["keep"] != "x" {
		t.Fatalf("redaksi nested salah: %v", nested)
	}
	first := out["list"].([]any)[0].(map[string]any)
	if first["pin"] != "[REDACTED]" || first["name"] != "a" {
		t.Fatalf("redaksi list salah: %v", first)
	}
}

func TestRedact_NilAndStruct(t *testing.T) {
	if Redact(nil) != nil {
		t.Fatal("nil harus tetap nil")
	}
	type payload struct {
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	out := Redact(payload{Password: "x", Name: "y"}).(map[string]any)
	if out["password"] != "[REDACTED]" || out["name"] != "y" {
		t.Fatalf("redaksi struct salah: %v", out)
	}
}

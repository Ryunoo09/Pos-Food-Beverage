package menu

import "testing"

func TestValidateCategory(t *testing.T) {
	if _, err := validateCategory(CategoryInput{Name: "   "}); err == nil {
		t.Error("nama kosong diterima")
	}

	long := make([]rune, maxNameLength+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := validateCategory(CategoryInput{Name: string(long)}); err == nil {
		t.Error("nama lebih dari 80 karakter diterima")
	}

	in, err := validateCategory(CategoryInput{Name: "  Minuman  "})
	if err != nil || in.Name != "Minuman" {
		t.Errorf("got %q err=%v", in.Name, err)
	}
}

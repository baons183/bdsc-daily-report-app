package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateUniqueFile(t *testing.T) {
	dir := t.TempDir()
	want := []string{"image.jpg", "image (2).jpg", "image (3).jpg"}
	for i, w := range want {
		f, name, err := createUniqueFile(dir, "image.jpg")
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString(w)
		f.Close()
		if name != w {
			t.Errorf("upload %d: got %q, want %q", i+1, name, w)
		}
	}
	// the first file must not have been overwritten by the later uploads
	if b, _ := os.ReadFile(filepath.Join(dir, "image.jpg")); string(b) != "image.jpg" {
		t.Errorf("image.jpg overwritten: %q", b)
	}
	// a path in the upload name never escapes the folder
	f, name, err := createUniqueFile(dir, `..\..\evil.jpg`)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if name != "evil.jpg" {
		t.Errorf("got %q, want evil.jpg", name)
	}
}

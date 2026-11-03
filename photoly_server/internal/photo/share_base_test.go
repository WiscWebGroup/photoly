package photo

import "testing"

func TestShareBase(t *testing.T) {
	for _, s := range []string{"http://192.168.1.2:5200/api", "https://photos.example.com/app/api/", "http://[::1]:5200/api"} {
		if _, err := validateShareBase(s); err != nil {
			t.Fatal(s, err)
		}
	}
	for _, s := range []string{"//evil", "javascript:alert(1)", "https://u:p@host/api", "https://host/api?x=1", "https://host/api#x", "https://", "https://host/\\evil", "https://host/api?"} {
		if _, err := validateShareBase(s); err == nil {
			t.Fatal("accepted", s)
		}
	}
}

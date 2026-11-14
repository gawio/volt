package evnhcmc

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
)

func TestHasSessionCookie(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cookies []*http.Cookie
		want    bool
	}{
		{"session without infrastructure cookies", []*http.Cookie{{Name: "evn_session", Value: "session", Path: "/api"}}, true},
		{"infrastructure cookies only", []*http.Cookie{{Name: "TS018cfa5d", Value: "routing", Path: "/"}}, false},
		{"empty session", []*http.Cookie{{Name: "evn_session", Value: "", Path: "/"}}, false},
		{"session for another path", []*http.Cookie{{Name: "evn_session", Value: "session", Path: "/other"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, err := url.Parse("https://example.com/")
			if err != nil {
				t.Fatal(err)
			}
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			jar.SetCookies(base, tc.cookies)
			client := &evnClient{httpClient: &http.Client{Jar: jar}, baseURL: base, electricityConsumptionAPI: "/api/usage"}
			if got := client.hasSessionCookie(); got != tc.want {
				t.Fatalf("hasSessionCookie() = %v, want %v", got, tc.want)
			}
		})
	}
}

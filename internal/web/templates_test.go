package web_test

import (
	"os"
	"strings"
	"testing"
)

func TestAccessRequestSuccessOffersAnotherAddressAction(t *testing.T) {
	body, err := os.ReadFile("templates/base.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)

	for _, want := range []string{
		`{{T "access_request.another"}}`,
		`onclick="document.getElementById('access-request-email').focus()"`,
		`<form id="access-request-form" method="POST" action="/a/access-request"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("base template is missing %q", want)
		}
	}
}

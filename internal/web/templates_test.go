package web_test

import (
	"os"
	"strings"
	"testing"
)

func TestAccessRequestBannerPersistsItsStateInTheBrowser(t *testing.T) {
	body, err := os.ReadFile("templates/base.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)

	for _, want := range []string{
		`<details open id="access-request"`,
		`data-access-request`,
		`data-server-success="{{if .AccessRequest.Success}}true{{else}}false{{end}}"`,
		`[&::-webkit-details-marker]:hidden`,
		`group-open:rotate-180`,
		`data-access-request-success-icon`,
		`data-access-request-default-icon`,
		`data-access-request-success-title`,
		`data-access-request-default-title`,
		`data-access-request-success-body`,
		`data-access-request-default-body`,
		`<label for="access-request-email" class="sr-only">{{T "access_request.email"}}</label>`,
		`{{T "access_request.benefit_1"}}`,
		`{{T "access_request.benefit_2"}}`,
		`{{T "access_request.benefit_3"}}`,
		`{{T "access_request.prompt"}}`,
		`data-access-request-another`,
		`<form id="access-request-form" method="POST" action="/a/access-request"`,
		`<script src="/static/access-request.js" defer></script>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("base template is missing %q", want)
		}
	}

	if strings.Contains(html, `onclick="document.getElementById('access-request-form')`) {
		t.Fatal("access request state must be managed by the dedicated browser script")
	}
}

func TestAccessRequestStateScriptUsesLocalStorageAndToggleEvents(t *testing.T) {
	body, err := os.ReadFile("static/access-request.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(body)

	for _, want := range []string{
		`localStorage`,
		`buh.access-request.v1`,
		`data-access-request`,
		`addEventListener('toggle'`,
		`data-access-request-another`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("access request script is missing %q", want)
		}
	}
}

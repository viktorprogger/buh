package web_test

import (
	"os"
	"strings"
	"testing"
)

func TestAccessRequestBannerStartsOpenAndKeepsSuccessStateCompact(t *testing.T) {
	body, err := os.ReadFile("templates/base.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)

	for _, want := range []string{
		`<details open class="group overflow-hidden`,
		`[&::-webkit-details-marker]:hidden`,
		`group-open:rotate-180`,
		`{{if .AccessRequest.Success}}{{T "access_request.success"}}{{else}}{{T "access_request.title"}}{{end}}`,
		`{{if .AccessRequest.Success}}`,
		`<label for="access-request-email" class="sr-only">{{T "access_request.email"}}</label>`,
		`{{T "access_request.benefit_1"}}`,
		`{{T "access_request.benefit_2"}}`,
		`{{T "access_request.benefit_3"}}`,
		`{{T "access_request.prompt"}}`,
		`{{T "access_request.another"}}`,
		`onclick="document.getElementById('access-request-form').hidden=false; this.hidden=true; document.getElementById('access-request-email').focus()"`,
		`<form id="access-request-form" method="POST" action="/a/access-request" class="mt-5 flex flex-col gap-3 sm:flex-row"{{if .AccessRequest.Success}} hidden{{end}}>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("base template is missing %q", want)
		}
	}
}

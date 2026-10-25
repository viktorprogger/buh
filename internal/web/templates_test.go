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
		`<details open class="bg-blue-50`,
		`{{T "access_request.benefit_1"}}`,
		`{{T "access_request.benefit_2"}}`,
		`{{T "access_request.benefit_3"}}`,
		`{{T "access_request.prompt"}}`,
		`{{T "access_request.another"}}`,
		`onclick="document.getElementById('access-request-form').hidden=false; this.hidden=true; document.getElementById('access-request-email').focus()"`,
		`<form id="access-request-form" method="POST" action="/a/access-request" class="mt-3 flex gap-2"{{if .AccessRequest.Success}} hidden{{end}}>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("base template is missing %q", want)
		}
	}
}

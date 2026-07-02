package shared

import (
	"html/template"
	"log"
	"net/http"
)

func RenderTemplate(w http.ResponseWriter, tmpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func RenderError(w http.ResponseWriter, errPage *template.Template, code int) {
	type errData struct {
		Code    int
		Title   string
		Message string
	}
	data := map[int]errData{
		http.StatusNotFound:  {404, "Страница није пронађена", "Ресурс који тражите не постоји или је премештен."},
		http.StatusForbidden: {403, "Приступ забрањен", "Немате дозволу да приступите овом ресурсу."},
	}
	d, ok := data[code]
	if !ok {
		http.Error(w, http.StatusText(code), code)
		return
	}
	w.WriteHeader(code)
	RenderTemplate(w, errPage, d)
}

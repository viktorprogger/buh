package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

const LangCookie = "lang"

var supported = []language.Tag{
	language.Serbian,
	language.English,
	language.Russian,
}

var matcher = language.NewMatcher(supported)

type contextKey int

const ctxKey contextKey = iota

// Bundle holds all loaded translations.
type Bundle struct {
	b *goi18n.Bundle
}

// NewBundle loads all locale files and returns a ready-to-use bundle.
func NewBundle() *Bundle {
	b := goi18n.NewBundle(language.Serbian)
	b.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, lang := range []string{"sr", "en", "ru"} {
		data, err := fs.ReadFile(localeFS, "locales/"+lang+".json")
		if err != nil {
			panic("i18n: cannot read locale file " + lang + ".json: " + err.Error())
		}
		b.MustParseMessageFileBytes(data, lang+".json")
	}
	return &Bundle{b: b}
}

// Localizer translates strings for a specific language.
type Localizer struct {
	l    *goi18n.Localizer
	lang string
}

// NewLocalizer creates a Localizer for the given language code, falling back to Serbian.
func (b *Bundle) NewLocalizer(lang string) *Localizer {
	return &Localizer{
		l:    goi18n.NewLocalizer(b.b, lang, "sr"),
		lang: lang,
	}
}

// T translates a simple message key. Returns key on failure.
func (l *Localizer) T(key string) string {
	if l.l == nil {
		return key
	}
	msg, err := l.l.Localize(&goi18n.LocalizeConfig{MessageID: key})
	if err != nil {
		return key
	}
	return msg
}

// TWith translates a key with template data. Data is passed as alternating key-value pairs.
func (l *Localizer) TWith(key string, kv ...interface{}) string {
	if l.l == nil {
		return key
	}
	data := make(map[string]interface{}, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok {
			data[k] = kv[i+1]
		}
	}
	msg, err := l.l.Localize(&goi18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: data,
	})
	if err != nil {
		return key
	}
	return msg
}

// Lang returns the resolved language code ("sr", "en", "ru").
func (l *Localizer) Lang() string { return l.lang }

// WithLocalizer stores a localizer in the request context.
func WithLocalizer(r *http.Request, l *Localizer) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxKey, l))
}

// FromContext retrieves the localizer from the context.
// Returns a no-op localizer (key pass-through) if none is set.
func FromContext(ctx context.Context) *Localizer {
	l, _ := ctx.Value(ctxKey).(*Localizer)
	if l == nil {
		return &Localizer{lang: "sr"}
	}
	return l
}

// Detect returns the best-matching supported language for a request.
// Priority: DB preference (passed as dbLang) > cookie > Accept-Language header.
func Detect(r *http.Request, dbLang string) string {
	if isSupported(dbLang) {
		return dbLang
	}
	if c, err := r.Cookie(LangCookie); err == nil && isSupported(c.Value) {
		return c.Value
	}
	tags, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	tag, _, _ := matcher.Match(tags...)
	return baseTag(tag)
}

func isSupported(lang string) bool {
	for _, t := range supported {
		if baseTag(t) == lang {
			return true
		}
	}
	return false
}

func baseTag(t language.Tag) string {
	b, _ := t.Base()
	return b.String()
}

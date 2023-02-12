package locale

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"

	"math/rand"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var OwoFaces = []string{
	"OwO", "Owo", "owO", "ÓwÓ", "ÕwÕ", "@w@", "ØwØ", "øwø", "uwu", "☆w☆", "✧w✧", "♥w♥", "゜w゜", "◕w◕", "ᅌwᅌ", "◔w◔", "ʘwʘ", "⓪w⓪", "(owo)",
}

func Owoify(input string) string {
	pieces := strings.Split(input, "{{")
	full := owoifyString(pieces[0])

	for _, str := range pieces[1:] {
		// NOTE will fail for strings with {{ but no matching }}
		sub := strings.Split(str, "}}")
		full += "{{" + sub[0] + "}}" + owoifyString(sub[1])
	}

	return full
}

func owoifyString(input string) string {
	output := strings.ReplaceAll(input, "th", "d")
	output = strings.ReplaceAll(output, "ove", "uv")
	re := regexp.MustCompile(`(?:r|l)`)
	output = string(re.ReplaceAll([]byte(output), []byte("w")))

	re = regexp.MustCompile(`(?:R|L)`)
	output = string(re.ReplaceAll([]byte(output), []byte("W")))

	re = regexp.MustCompile(`n([aeiou])`)
	output = string(re.ReplaceAll([]byte(output), []byte("ny$1")))

	re = regexp.MustCompile(`N([aeiou])`)
	output = string(re.ReplaceAll([]byte(output), []byte("Ny$1")))

	re = regexp.MustCompile(`N([AEIOU])`)
	output = string(re.ReplaceAll([]byte(output), []byte("NY$1")))

	return output
}

func OwoToml(path, output string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	unmarshalFuncs := map[string]i18n.UnmarshalFunc{
		"toml": toml.Unmarshal,
	}
	mf, err := i18n.ParseMessageFileBytes(bytes, path, unmarshalFuncs)

	if err != nil {
		return fmt.Errorf("failed to load message file %s: %s", path, err)
	}

	messageTemplates := map[string]*i18n.MessageTemplate{}
	for _, m := range mf.Messages {
		template := i18n.NewMessageTemplate(m)
		if template == nil {
			continue
		}

		template.Hash = hash(template)
		messageTemplates[m.ID] = template
	}

	val := marshalOwoValue(messageTemplates)
	content, err := encodeToml(val)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(output, content, 0666); err != nil {
		return err
	}

	return nil
}

func encodeToml(v interface{}) (content []byte, err error) {
	// by toml
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = ""
	err = enc.Encode(v)
	content = buf.Bytes()

	if err != nil {
		return nil, fmt.Errorf("failed to marshal strings: %s", err)
	}
	return
}

func marshalOwoValue(messageTemplates map[string]*i18n.MessageTemplate) interface{} {
	val := make(map[string]interface{}, len(messageTemplates))
	for id, template := range messageTemplates {
		m := map[string]string{}

		m["hash"] = template.Hash

		for pluralForm, template := range template.PluralTemplates {
			text := template.Src
			if rand.Intn(2) == 1 {
				faceIdx := rand.Intn(len(OwoFaces))
				text = fmt.Sprintf("%s %s", Owoify(text), OwoFaces[faceIdx])
			} else {
				text = fmt.Sprintf("%s", Owoify(text))
			}

			m[string(pluralForm)] = text
		}
		val[id] = m
	}
	return val
}

// Source: https://github.com/nicksnyder/go-i18n/blob/603af13488ca751833928c45f7ada0eed720a392/v2/goi18n/merge_command.go#L294
func hash(t *i18n.MessageTemplate) string {
	h := sha1.New()
	_, _ = io.WriteString(h, t.Description)
	_, _ = io.WriteString(h, t.PluralTemplates["other"].Src)
	return fmt.Sprintf("sha1-%x", h.Sum(nil))
}

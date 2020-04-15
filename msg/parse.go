package msg

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/manifoldco/promptui"
)

// TODO: probably should do a real i18n & template library, but they all looked heavy atm
func Parse(str string, data map[string]interface{}) string {
	var buf bytes.Buffer
	tmpl, err := template.New("msg.Parse").Funcs(promptui.FuncMap).Parse(str)
	if err != nil {
		panic(fmt.Errorf("could not compile template:\ntemplate: %s\nerr: %w", str, data, err))
	}
	err = tmpl.Execute(&buf, data)
	if err != nil {
		panic(fmt.Errorf("could not execute template with given args:\ntemplate: %s\nargs: %v\nerr: %w", str, data, err))
	}
	return strings.TrimSpace(buf.String())
}

func Print(str string, data map[string]interface{}) {
	fmt.Println(Parse(str, data))
}

func Fprint(w io.Writer, str string, data map[string]interface{}) {
	fmt.Fprintln(w, Parse(str, data))
}

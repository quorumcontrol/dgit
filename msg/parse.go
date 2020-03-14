package msg

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"strings"
)

// TODO: probably should do a real i18n & template library, but they all looked heavy atm
func Parse(str string, data map[string]interface{}) string {
	var buf bytes.Buffer
	err := template.Must(template.New("msg.Parse").Parse(str)).Execute(&buf, data)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(buf.String())
}

func Print(str string, data map[string]interface{}) {
	fmt.Println(Parse(str, data))
}

func Fprint(w io.Writer, str string, data map[string]interface{}) {
	fmt.Fprintln(w, Parse(str, data))
}

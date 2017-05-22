package main

import (
	//	"fmt"
	"bytes"
	//	"os"
	"testing"
	gotemplate "text/template"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
)

func TestThing(t *testing.T) {
	val := notify.WebhookMessage{
		Data: &template.Data{
			Receiver: "webhook",
			Status:   "firing",
			Alerts: template.Alerts{
				template.Alert{
					Status:       "resolved",
					Labels:       template.KV{"dev": "sda3", "instance": "example4", "alertname": "DiskRunningFull"},
					Annotations:  template.KV{"test": "value"},
					StartsAt:     time.Unix(63630938212, 297874780),
					EndsAt:       time.Unix(63630938512, 297874780),
					GeneratorURL: "http://generator.url/",
				},
				template.Alert{
					Status:       "firing",
					Labels:       template.KV{"instance": "example1", "alertname": "DiskRunningFull", "dev": "sda2"},
					StartsAt:     time.Unix(63630938256, 503821262),
					GeneratorURL: "",
				},
			},
			GroupLabels:  template.KV{"alertname": "DiskRunningFull"},
			CommonLabels: template.KV{"alertname": "DiskRunningFull"},
			ExternalURL:  "http://localhost:9093",
		},
		Version:  "4",
		GroupKey: "{}:{alertname=\"DiskRunningFull\"}",
	}
	message := `
Alertmanager URL: {{.Data.ExternalURL}}
{{range .Data.Alerts}}
  * {{.Status}} {{.GeneratorURL}}
  {{if .Labels}}
    Labels:
  {{- end}}
  {{range $key, $value := .Labels}}
    - {{$key}} = {{$value -}}
  {{end}}
  {{if .Annotations}}
    Annotations:
  {{- end}}
  {{range $key, $value := .Annotations}}
    - {{$key}} = {{$value -}}
  {{end}}
{{end}}
`
	var b bytes.Buffer
	tmpl := gotemplate.Must(gotemplate.New("alert").Parse(message))
	err := tmpl.Execute(&b, val)
	if err != nil {
		t.Fatalf("Error executing template: %s", err)
	}
	t.Logf("Thing: %s\n", b.String())
}

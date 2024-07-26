{{/*
Expand the name of the chart.
*/}}
{{- define "dpf-ovn-kubernetes-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "dpf-ovn-kubernetes-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "dpf-ovn-kubernetes-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "dpf-ovn-kubernetes-operator.labels" -}}
helm.sh/chart: {{ include "dpf-ovn-kubernetes-operator.chart" . }}
{{ include "dpf-ovn-kubernetes-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "dpf-ovn-kubernetes-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "dpf-ovn-kubernetes-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Function that returns if the operator webhook is enabled. The result can be compared using string comparison.
*/}}
{{- define "dpf-ovn-kubernetes-operator.isWebhookEnabled" -}}
{{- if has "--enable-webhook=false" .Values.controllerManager.args -}}
false
{{- else -}}
true
{{- end }}
{{- end }}

{{/*
The name of the webhook certificate
*/}}
{{- define "dpf-ovn-kubernetes-operator.webhook.certificateName" -}}
{{ include "dpf-ovn-kubernetes-operator.fullname" . }}-webhook
{{- end }}

{{/*
The name of the webhook secret that contains the certificate
*/}}
{{- define "dpf-ovn-kubernetes-operator.webhook.secretName" -}}
{{ include "dpf-ovn-kubernetes-operator.fullname" . }}-webhook-cert
{{- end }}

{{/*
The name of the webhook service
*/}}
{{- define "dpf-ovn-kubernetes-operator.webhook.serviceName" -}}
{{ include "dpf-ovn-kubernetes-operator.fullname" . }}-webhook
{{- end }}

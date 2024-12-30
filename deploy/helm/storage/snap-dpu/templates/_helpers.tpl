{{/*
Expand the name of the chart.
*/}}
{{- define "snap-dpu.name" -}}
{{- default "snap-dpu" .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "snap-dpu.fullname" -}}
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
{{- define "snap-dpu.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "snap-dpu.labels" -}}
helm.sh/chart: {{ include "snap-dpu.chart" . }}
{{ include "snap-dpu.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "snap-dpu.selectorLabels" -}}
app.kubernetes.io/name: {{ include "snap-dpu.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "snap-dpu.serviceAccountName" -}}
{{- printf "%s-sa" (include "snap-dpu.fullname" .) }}
{{- end }}

{{/*
Create the name of the role to use
*/}}
{{- define "snap-dpu.roleName" -}}
{{- printf "%s-role" (include "snap-dpu.fullname" .) }}
{{- end }}

{{/*
Create the name of the role binding to use
*/}}
{{- define "snap-dpu.roleBindingName" -}}
{{- printf "%s-role-binding" (include "snap-dpu.fullname" .) }}
{{- end }}


{{- define "upcloud-csi.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "upcloud-csi.fullname" -}}
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

{{- define "upcloud-csi.labels" -}}
helm.sh/chart: {{ include "upcloud-csi.name" . }}-{{ .Chart.Version | replace "+" "_" }}
{{ include "upcloud-csi.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "upcloud-csi.selectorLabels" -}}
app.kubernetes.io/name: {{ include "upcloud-csi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "upcloud-csi.driverImage" -}}
{{- $repository := .Values.controller.image.repository -}}
{{- $tag := .Values.controller.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" $repository $tag -}}
{{- end }}

{{- define "upcloud-csi.credentialsSecret" -}}
{{- .Values.credentials.secretName -}}
{{- end }}

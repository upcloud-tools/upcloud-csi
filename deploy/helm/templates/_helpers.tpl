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
{{- with .Values.commonLabels }}
{{ tpl (toYaml .) $ }}
{{- end }}
{{- end }}

{{- define "upcloud-csi.selectorLabels" -}}
app.kubernetes.io/name: {{ include "upcloud-csi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "upcloud-csi.driverImage" -}}
{{- $repository := .Values.image.repository -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" $repository $tag -}}
{{- end }}

{{- define "upcloud-csi.credentialsSecret" -}}
{{- .Values.credentials.secretName -}}
{{- end }}

{{- define "upcloud-csi.credentialsChecksum" -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace (include "upcloud-csi.credentialsSecret" .) -}}
{{- if $secret -}}
{{- sha256sum (toJson $secret.data) -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end }}

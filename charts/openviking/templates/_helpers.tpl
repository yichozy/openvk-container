{{/*
Expand the name of the chart.
*/}}
{{- define "openviking.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
fullnameOverride > release name > chart name
*/}}
{{- define "openviking.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Common labels (includes role for visibility, but role is NOT in selectorLabels to keep selector immutable)
*/}}
{{- define "openviking.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "openviking.selectorLabels" . }}
app.openviking/role: {{ .Values.role }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels (immutable after creation — keep minimal)
*/}}
{{- define "openviking.selectorLabels" -}}
{{- range $key, $value := .Values.labels }}
{{ $key }}: {{ $value | quote }}
{{- end }}
{{- end }}

{{/*
Render ov.conf JSON for the Secret.
*/}}
{{- define "openviking.ovConf" -}}
{{- $_ := required "config.server.root_api_key is required (set in values or -f override)" .Values.config.server.root_api_key -}}
{{- $_ := required "config.embedding.dense.api_key is required" .Values.config.embedding.dense.api_key -}}
{{- $_ := required "config.vlm.api_key is required" .Values.config.vlm.api_key -}}
{{- $conf := dict -}}
{{- $_ := set $conf "server" .Values.config.server -}}
{{- $_ := set $conf "storage" (dict "workspace" .Values.config.storageWorkspace "vectordb" .Values.config.vectordb) -}}
{{- $_ := set $conf "log" .Values.config.log -}}
{{- $_ := set $conf "embedding" .Values.config.embedding -}}
{{- $_ := set $conf "vlm" .Values.config.vlm -}}
{{- mustToJson $conf -}}
{{- end }}

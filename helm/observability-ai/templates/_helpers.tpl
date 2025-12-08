{{/*
Expand the name of the chart.
*/}}
{{- define "observability-ai.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "observability-ai.fullname" -}}
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
{{- define "observability-ai.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "observability-ai.labels" -}}
helm.sh/chart: {{ include "observability-ai.chart" . }}
{{ include "observability-ai.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "observability-ai.selectorLabels" -}}
app.kubernetes.io/name: {{ include "observability-ai.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "observability-ai.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "observability-ai.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Query Processor component labels
*/}}
{{- define "observability-ai.queryProcessor.labels" -}}
{{ include "observability-ai.labels" . }}
app.kubernetes.io/component: query-processor
{{- end }}

{{/*
Query Processor selector labels
*/}}
{{- define "observability-ai.queryProcessor.selectorLabels" -}}
{{ include "observability-ai.selectorLabels" . }}
app.kubernetes.io/component: query-processor
{{- end }}

{{/*
Web UI component labels
*/}}
{{- define "observability-ai.web.labels" -}}
{{ include "observability-ai.labels" . }}
app.kubernetes.io/component: web
{{- end }}

{{/*
Web UI selector labels
*/}}
{{- define "observability-ai.web.selectorLabels" -}}
{{ include "observability-ai.selectorLabels" . }}
app.kubernetes.io/component: web
{{- end }}

{{/*
Database host
*/}}
{{- define "observability-ai.database.host" -}}
{{- .Values.postgresql.host | required "postgresql.host is required" }}
{{- end }}

{{/*
Database port
*/}}
{{- define "observability-ai.database.port" -}}
{{- .Values.postgresql.port }}
{{- end }}

{{/*
Database name
*/}}
{{- define "observability-ai.database.name" -}}
{{- .Values.postgresql.database }}
{{- end }}

{{/*
Database username
*/}}
{{- define "observability-ai.database.username" -}}
{{- .Values.postgresql.username }}
{{- end }}

{{/*
Redis host
*/}}
{{- define "observability-ai.redis.host" -}}
{{- .Values.redis.host | required "redis.host is required" }}
{{- end }}

{{/*
Redis port
*/}}
{{- define "observability-ai.redis.port" -}}
{{- .Values.redis.port }}
{{- end }}

{{/*
Secret name for application secrets
*/}}
{{- define "observability-ai.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- include "observability-ai.fullname" . }}
{{- end }}
{{- end }}

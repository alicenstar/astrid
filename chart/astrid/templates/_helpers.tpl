{{- define "astrid.fullname" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "astrid.name" -}}
astrid
{{- end -}}

{{- define "astrid.labels" -}}
app.kubernetes.io/name: {{ include "astrid.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "astrid.selectorLabels" -}}
app.kubernetes.io/name: {{ include "astrid.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "astrid.databaseURL" -}}
{{- if .Values.postgresql.enabled -}}
postgres://{{ .Values.postgresql.auth.username }}:{{ .Values.postgresql.auth.password }}@{{ include "astrid.fullname" . }}-postgresql:5432/{{ .Values.postgresql.auth.database }}?sslmode=disable
{{- else -}}
postgres://{{ .Values.externalDatabase.user }}:{{ .Values.externalDatabase.password }}@{{ .Values.externalDatabase.host }}:{{ .Values.externalDatabase.port }}/{{ .Values.externalDatabase.database }}?sslmode=disable
{{- end -}}
{{- end -}}

{{- define "astrid.redisURL" -}}
{{- if .Values.redis.enabled -}}
redis://{{ include "astrid.fullname" . }}-redis-master:6379/0
{{- else -}}
{{- if .Values.externalRedis.password -}}
redis://:{{ .Values.externalRedis.password }}@{{ .Values.externalRedis.host }}:{{ .Values.externalRedis.port }}/0
{{- else -}}
redis://{{ .Values.externalRedis.host }}:{{ .Values.externalRedis.port }}/0
{{- end -}}
{{- end -}}
{{- end -}}

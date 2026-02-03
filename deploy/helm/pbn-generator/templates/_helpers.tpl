{{- define "pbn-generator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "pbn-generator.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "pbn-generator.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

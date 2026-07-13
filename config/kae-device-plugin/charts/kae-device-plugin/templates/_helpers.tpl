{{- define "kae-device-plugin.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kae-device-plugin.image" -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}

{{- define "kae-device-plugin.labels" -}}
helm.sh/chart: {{ include "kae-device-plugin.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "kae-device-plugin.devicePluginLabels" -}}
app: {{ .Values.devicePlugin.name }}
app.kubernetes.io/name: {{ .Values.devicePlugin.name }}
{{ include "kae-device-plugin.labels" . }}
{{- end -}}

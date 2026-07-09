{{- if .Values.admissionWebhook.enabled }}
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.admissionWebhook.name }}
  labels:
{{ include "kae-device-plugin.labels" . | indent 4 }}
spec:
  selector:
    app: {{ .Values.devicePlugin.name }}
  ports:
    - name: https
      port: 443
      protocol: TCP
      targetPort: https
{{- end }}

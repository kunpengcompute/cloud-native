{{- if .Values.admissionWebhook.enabled }}
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: {{ .Values.admissionWebhook.name }}
  labels:
{{ include "kae-device-plugin.labels" . | indent 4 }}
{{- if eq .Values.admissionWebhook.cert.mode "certManager" }}
  annotations:
    cert-manager.io/inject-ca-from: {{ .Release.Namespace }}/{{ .Values.admissionWebhook.cert.certManager.servingCertificateName }}
{{- end }}
webhooks:
  - name: kae-injection.kunpeng.com
    admissionReviewVersions:
      - v1beta1
    sideEffects: None
    failurePolicy: {{ .Values.admissionWebhook.failurePolicy }}
    clientConfig:
      service:
        name: {{ .Values.admissionWebhook.name }}
        namespace: {{ .Release.Namespace }}
        path: /mutate--v1-pod
        port: 443
      caBundle: {{ .Values.admissionWebhook.cert.caBundle | quote }}
    objectSelector:
      matchExpressions:
        - key: app
          operator: NotIn
          values:
            - {{ .Values.devicePlugin.name }}
    rules:
      - operations:
          - CREATE
          - UPDATE
        apiGroups:
          - ""
        apiVersions:
          - v1
        resources:
          - pods
{{- end }}

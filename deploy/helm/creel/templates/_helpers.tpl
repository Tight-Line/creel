{{/*
Validate required values and fail early if missing.
*/}}
{{- define "creel.validateRequired" -}}
{{- if not .Values.auth.bootstrapKeyHash -}}
{{- fail "auth.bootstrapKeyHash is required. Set it to the SHA-256 hash of your system account API key." -}}
{{- end -}}
{{- if .Values.dashboard.enabled -}}
  {{- if not .Values.dashboard.auth.password -}}
  {{- fail "dashboard.auth.password is required when the dashboard is enabled." -}}
  {{- end -}}
  {{- if not .Values.dashboard.auth.apiKey -}}
  {{- fail "dashboard.auth.apiKey is required when the dashboard is enabled. Set it to a valid Creel system account API key." -}}
  {{- end -}}
  {{- if and (not .Values.dashboard.auth.appKey) (not .Values.dashboard.auth.appKeySecret) -}}
  {{- fail "dashboard.auth.appKey (or dashboard.auth.appKeySecret) is required when the dashboard is enabled. Generate one with: echo \"base64:$(openssl rand -base64 32)\"" -}}
  {{- end -}}
{{- end -}}
{{- if .Values.ingress.enabled -}}
  {{- if not .Values.ingress.creel.host -}}
  {{- fail "ingress.creel.host is required when ingress is enabled." -}}
  {{- end -}}
  {{- if and .Values.dashboard.enabled (not .Values.ingress.dashboard.host) -}}
  {{- fail "ingress.dashboard.host is required when both ingress and dashboard are enabled." -}}
  {{- end -}}
{{- end -}}
{{- end -}}

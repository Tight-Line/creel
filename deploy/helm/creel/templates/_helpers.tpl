{{/*
Validate required values and fail early if missing.
*/}}
{{- define "creel.validateRequired" -}}
{{- if .Values.ingress.enabled -}}
  {{- if not .Values.ingress.creel.host -}}
  {{- fail "ingress.creel.host is required when ingress is enabled." -}}
  {{- end -}}
  {{- if and .Values.dashboard.enabled (not .Values.ingress.dashboard.host) -}}
  {{- fail "ingress.dashboard.host is required when both ingress and dashboard are enabled." -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/*
Name of the auto-managed secrets resource.
*/}}
{{- define "creel.secretName" -}}
{{- printf "%s-creel-secrets" .Release.Name -}}
{{- end -}}

{{/*
Validate required values and fail early if missing.
*/}}
{{- define "creel.validateRequired" -}}
{{- if .Values.ingress.enabled -}}
  {{- if not .Values.ingress.rest.host -}}
  {{- fail "ingress.rest.host is required when ingress is enabled." -}}
  {{- end -}}
  {{- if not .Values.ingress.grpc.host -}}
  {{- fail "ingress.grpc.host is required when ingress is enabled." -}}
  {{- end -}}
  {{- if eq .Values.ingress.rest.host .Values.ingress.grpc.host -}}
  {{- fail "ingress.grpc.host must differ from ingress.rest.host (nginx does not allow duplicate host+path across Ingress resources)." -}}
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

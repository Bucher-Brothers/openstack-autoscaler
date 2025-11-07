{{/*
Expand the name of the chart.
*/}}
{{- define "openstack-autoscaler.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "openstack-autoscaler.fullname" -}}
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
{{- define "openstack-autoscaler.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "openstack-autoscaler.labels" -}}
helm.sh/chart: {{ include "openstack-autoscaler.chart" . }}
{{ include "openstack-autoscaler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "openstack-autoscaler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "openstack-autoscaler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "openstack-autoscaler.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "openstack-autoscaler.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret to use for OpenStack credentials
*/}}
{{- define "openstack-autoscaler.secretName" -}}
{{- if .Values.openstack.existingSecret }}
{{- .Values.openstack.existingSecret }}
{{- else }}
{{- include "openstack-autoscaler.fullname" . }}-openstack
{{- end }}
{{- end }}

{{/*
Create the image reference
*/}}
{{- define "openstack-autoscaler.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Common annotations
*/}}
{{- define "openstack-autoscaler.annotations" -}}
{{- with .Values.commonAnnotations }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Create gRPC probe command
*/}}
{{- define "openstack-autoscaler.grpcProbe" -}}
- /bin/sh
- -c
- |
  grpc_health_probe -addr=localhost:{{ .Values.service.targetPort | default 50051 }} || exit 1
{{- end }}
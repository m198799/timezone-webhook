{{/*
Copyright © 2021 Yonatan Kahana

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/}}

{{/*
Expand the name of the charts.
*/}}
{{- define "service-webhook.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains charts name it will be used as a full name.
*/}}
{{- define "service-webhook.fullname" -}}
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
Create charts name and version as used by the charts label.
*/}}
{{- define "service-webhook.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "service-webhook.labels" -}}
helm.sh/chart: {{ include "service-webhook.chart" . }}
{{ include "service-webhook.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "service-webhook.selectorLabels" -}}
app.kubernetes.io/name: {{ include "service-webhook.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "service-webhook.serviceAccountName" -}}
{{- default (include "service-webhook.fullname" .) .Values.serviceAccount.name }}
{{- end }}

{{/*
Defines the service name for the webhook
*/}}
{{- define "service-webhook.serviceName" -}}
{{ .Release.Name }}
{{- end }}


{{/*
Replica for timezone-webhook deployment
*/}}
{{- define "service-webhook.replicas" -}}
{{- if eq .Values.deploy_mode "performance" }}2
{{- else }}1
{{- end }}
{{- end }}

{{/*
Create or get a self-signed certificate.
*/}}
{{- define "service-webhook.ca" -}}
{{- $fqdn := printf "%s.%s.svc" (include "service-webhook.serviceName" .) .Release.Namespace }}
{{- $secretName := printf "%s-tls" (include "service-webhook.fullname" .) }}
{{- $secret := (lookup "v1" "Secret" .Release.Namespace $secretName) }}
{{- if and .Values.webhook.crtPEM .Values.webhook.keyPEM }}
  {{- $cert := .Values.webhook.crtPEM | b64dec }}
  {{- $key := .Values.webhook.keyPEM | b64dec }}
  {{- dict "Cert" $cert "Key" $key | toYaml }}  {{/* 假设 CA Bundle 同时用作证书和密钥，根据实际情况调整 */}}
{{/* 判断 Secrets 是否存在并且包含 tls.crt 和 tls.key */}}
{{- else if and $secret (index $secret.data "tls.crt") (index $secret.data "tls.key") }}
  {{- $tlsCrt := index $secret.data "tls.crt" | b64dec }}
  {{- $tlsKey := index $secret.data "tls.key" | b64dec }}
  {{- dict "Cert" $tlsCrt "Key" $tlsKey | toYaml }}
{{/* 如果没有找到有效的 Secrets 且 .Values.webhook.caBundle 也没有提供，生成新的证书 */}}
{{- else }}
  {{- $ca := genSelfSignedCert $fqdn (list) (list $fqdn) 5114 }}
  {{- $ca | toYaml }}
{{- end }}
{{- end }}




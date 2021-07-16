service: cawi-portal
runtime: go115

env_variables:
  BLAISE_BASE_URL: _BLAISE_BASE_URL
  BUS_CLIENT_ID: _BUS_CLIENT_ID
  BUS_URL: _BUS_URL

vpc_access_connector:
  name: projects/_PROJECT_ID/locations/europe-west2/connectors/vpcconnect

basic_scaling:
  idle_timeout: 60s
  max_instances: 1

instance_class: B4

handlers:
- url: /.*
  script: auto
  secure: always
  redirect_http_response_code: 301

service: cawi-portal
runtime: go122

env_variables:
  BLAISE_REST_API: _BLAISE_REST_API
  BUS_CLIENT_ID: _BUS_CLIENT_ID
  BUS_URL: _BUS_URL
  CATI_URL: _CATI_URL
  JWT_SECRET: _JWT_SECRET
  SESSION_SECRET: _SESSION_SECRET
  ENCRYPTION_SECRET: _ENCRYPTION_SECRET
  REDIS_SESSION_DB: _REDIS_SESSION_DB
  GIN_MODE: release

vpc_access_connector:
  name: projects/_PROJECT_ID/locations/europe-west2/connectors/vpcconnect

automatic_scaling:
  min_instances: _MIN_INSTANCES
  max_instances: _MAX_INSTANCES
  target_cpu_utilization: _TARGET_CPU_UTILIZATION
  max_concurrent_requests: 40

instance_class: F2

handlers:
- url: /.*
  script: auto
  secure: always
  redirect_http_response_code: 301

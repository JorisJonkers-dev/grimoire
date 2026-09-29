# Docker Pattern Templates

These are complete, parameterized Dockerfile and entrypoint templates. They are
intended to be copied into a target repository and rendered by replacing every
`{{placeholder}}` with a repo-specific value, Kustomize/Flux `postBuild`
substitution, or another explicit template renderer. They are not one shared
base Dockerfile and they do not encode source-repository domains, namespaces,
hostnames, queues, IP addresses, or image prefixes.

## Templates

- `jvm/Dockerfile.jvm.tmpl`: multi-stage JVM application build and runtime.
- `jvm/Dockerfile.crac-jvm.tmpl`: multi-stage CRaC JVM build with `train` and
  `runtime` targets.
- `entrypoints/otel-entrypoint.sh.tmpl`: OTel-aware Java entrypoint that
  composes resource attributes and optional Java agent flags.
- `entrypoints/crac-entrypoint.sh.tmpl`: CRaC train, restore, and normal run
  entrypoint.
- `vue/Dockerfile.vue-build-runtime.tmpl`: generic Vue build plus nginx
  runtime.
- `vue/Dockerfile.pnpm-build.tmpl`: pnpm Vue build plus nginx runtime.
- `vue/Dockerfile.yarn-build.tmpl`: Yarn Vue build plus nginx runtime.
- `nginx/Dockerfile.privileged-nginx.tmpl`: root-capable nginx static runtime.
- `nginx/Dockerfile.unprivileged-nginx.tmpl`: non-root nginx static runtime.

## Placeholder Contract

All placeholders are required unless the target repository deliberately renders
them to an empty value. Keep concrete values in the consuming repository or in
deployment-time substitution, not in these templates.

| Placeholder | Meaning |
| --- | --- |
| `{{app_gid}}` | Numeric application group ID used for JVM runtime ownership. |
| `{{app_group}}` | Application group name used by `{{runtime_user_create_command}}`. |
| `{{app_home}}` | Application working directory in the runtime image. |
| `{{app_jar_path}}` | Final jar path inside the runtime image. |
| `{{app_uid}}` | Numeric application user ID used for JVM runtime ownership. |
| `{{app_user}}` | Application user name used by `{{runtime_user_create_command}}`. |
| `{{build_revision}}` | Build or VCS revision baked into `SERVICE_VERSION`. |
| `{{checkpoint_dir}}` | CRaC checkpoint directory mounted or copied into runtime. |
| `{{crac_entrypoint_source}}` | Build-context path to `crac-entrypoint.sh`. |
| `{{crac_mode}}` | Default CRaC mode: `train`, `restore`, or `run`. |
| `{{crac_restore_java_args}}` | Extra JVM args used only for CRaC restore. |
| `{{crac_runtime_image}}` | CRaC-capable JVM runtime image. |
| `{{crac_train_java_args}}` | Extra JVM args used only for CRaC checkpoint training. |
| `{{deployment_environment}}` | Optional deployment environment OTel resource value. |
| `{{entrypoint_path}}` | Runtime path where the copied entrypoint is installed. |
| `{{frontend_build_command}}` | Generic frontend build command. |
| `{{frontend_build_script}}` | Package-manager script name for Vue builds. |
| `{{frontend_dist_dir}}` | Built frontend output directory in the build stage. |
| `{{frontend_source_paths}}` | Source paths copied before frontend build. |
| `{{frontend_workdir}}` | Working directory for frontend dependency and build stages. |
| `{{healthcheck_command}}` | JVM runtime healthcheck command. |
| `{{healthcheck_interval}}` | Docker healthcheck interval. |
| `{{healthcheck_retries}}` | Docker healthcheck retry count. |
| `{{healthcheck_start_period}}` | Docker healthcheck startup grace period. |
| `{{healthcheck_timeout}}` | Docker healthcheck timeout. |
| `{{java_opts}}` | Default JVM options. |
| `{{jvm_artifact_from_build_stage}}` | Jar glob or path copied from the JVM build stage. |
| `{{jvm_build_cache_target}}` | Build cache mount path for JVM dependency/build tooling. |
| `{{jvm_build_command}}` | JVM artifact build command. |
| `{{jvm_build_image}}` | JDK/build-tool image used by JVM build stages. |
| `{{jvm_dependency_manifest_paths}}` | Build manifest paths copied for JVM dependency caching. |
| `{{jvm_dependency_resolution_command}}` | Optional dependency-resolution command for JVM build cache warming. |
| `{{jvm_runtime_image}}` | Non-CRaC JVM runtime image. |
| `{{jvm_source_paths}}` | JVM source paths copied before the build command. |
| `{{nginx_config_path}}` | Build-context path to nginx `default.conf`. |
| `{{nginx_gid}}` | Numeric nginx group ID for unprivileged runtime. |
| `{{nginx_healthcheck_command}}` | nginx runtime healthcheck command. |
| `{{nginx_html_dir}}` | nginx document root. |
| `{{nginx_listen_port}}` | Generic nginx listen/expose port. |
| `{{nginx_runtime_image}}` | nginx runtime image for generic or privileged patterns. |
| `{{nginx_uid}}` | Numeric nginx user ID for unprivileged runtime. |
| `{{node_build_image}}` | Node image used for frontend dependency and build stages. |
| `{{otel_entrypoint_source}}` | Build-context path to `otel-entrypoint.sh`. |
| `{{otel_exporter_otlp_endpoint}}` | OTLP endpoint supplied by the consuming environment. |
| `{{otel_java_agent_enabled}}` | `true` or `false` toggle for adding `-javaagent`. |
| `{{otel_java_agent_path}}` | Runtime path to the OpenTelemetry Java agent jar. |
| `{{otel_java_agent_source}}` | Build-context path to the OpenTelemetry Java agent jar. |
| `{{otel_resource_attributes}}` | Comma-separated base OTel resource attributes. |
| `{{otel_service_name}}` | OTel service name. |
| `{{package_manager_cache_target}}` | Generic frontend package manager cache mount path. |
| `{{package_manager_files}}` | Generic package manager manifest and lockfile paths. |
| `{{package_manager_install_command}}` | Generic immutable/frozen dependency install command. |
| `{{pnpm_bootstrap_command}}` | Command that enables or installs the required pnpm version. |
| `{{pnpm_build_filter}}` | Optional pnpm workspace filter for the target app. |
| `{{pnpm_home}}` | pnpm home directory added to `PATH`. |
| `{{pnpm_package_manifest_paths}}` | pnpm package manifests copied for dependency caching. |
| `{{pnpm_store_path}}` | pnpm store cache mount path. |
| `{{pnpm_workspace_files}}` | pnpm workspace, lockfile, and npm config files. |
| `{{privileged_listen_port}}` | Port exposed by the privileged nginx runtime. |
| `{{runtime_user_create_command}}` | Image-specific command that creates the JVM runtime user/group. |
| `{{server_port}}` | JVM service port exposed by the image. |
| `{{service_version}}` | Fallback service version when `SERVICE_VERSION` is not set. |
| `{{static_assets_dir}}` | Static asset directory copied into stand-alone nginx images. |
| `{{unprivileged_listen_port}}` | Port exposed by the unprivileged nginx runtime. |
| `{{unprivileged_nginx_runtime_image}}` | nginx image intended to run as a non-root user. |
| `{{vite_api_base_url}}` | Vite build-time API base URL or empty fallback. |
| `{{vite_public_base_url}}` | Vite build-time public base URL or empty fallback. |
| `{{yarn_cache_folder}}` | Yarn cache folder mounted during immutable install. |
| `{{yarn_package_manifest_paths}}` | Yarn package manifests copied for dependency caching. |
| `{{yarn_workspace_files}}` | Yarn workspace, lockfile, and `.yarn` metadata paths. |

## Validation

Run `scripts/validate-templates.sh` from the repository root. The validator
checks JSON/YAML syntax, policy invariants, forbidden source-specific values,
Docker template structure, shell entrypoint syntax, and placeholder
documentation coverage without network access.

/* global pm */
function uuid4() {
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, function (c) {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}
function setIf(h, k, v) {
  if (!v) {
    return;
  }
  try {
    pm.request.headers.upsert({ key: k, value: String(v) });
  } catch (e) {
    pm.request.headers.add({ key: k, value: String(v) });
  }
}
const reqId = uuid4();
const corr = uuid4();
const idem = uuid4();
const evId = uuid4();
const nowIso = new Date().toISOString();
pm.collectionVariables.set("x_request_id", reqId);
pm.collectionVariables.set("x_correlation_id", corr);
pm.collectionVariables.set("idempotency_key", idem);
pm.collectionVariables.set("event_id", evId);
pm.collectionVariables.set("event_time", nowIso);
pm.collectionVariables.set("now_iso", nowIso);
setIf(pm.request, "X-Request-ID", reqId);
setIf(pm.request, "X-Correlation-ID", corr);
setIf(pm.request, "Idempotency-Key", idem);
setIf(pm.request, "X-Event-ID", evId);
setIf(pm.request, "X-Event-Time", nowIso);
setIf(pm.request, "Content-Type", "application/json");
setIf(pm.request, "Accept", "application/json");
setIf(pm.request, "X-Client-Name", "postman-avf");
const appEnv =
  (pm.environment.get("app_env") || pm.collectionVariables.get("app_env") || "").toLowerCase();
setIf(pm.request, "X-App-Env", appEnv || "unknown");
const base = (pm.environment.get("base_url") || pm.collectionVariables.get("base_url") || "").toLowerCase();
const pay = (pm.environment.get("payment_env") || "").toLowerCase();
const mqtt = (pm.environment.get("mqtt_topic_prefix") || "").trim();
const isStaging = appEnv === "staging" || /staging-api[.]ldtv[.]dev/.test(base);
const isProd = appEnv === "production" || /(^|\/)api[.]ldtv[.]dev/.test(base);
if (isStaging) {
  if (pay === "live") {
    throw new Error("postman-avf: staging cannot use payment_env=live");
  }
  if (mqtt === "avf/devices") {
    throw new Error("postman-avf: staging must not use production MQTT topic prefix avf/devices");
  }
}
if (isProd) {
  if (pay !== "live") {
    throw new Error("postman-avf: production requires payment_env=live");
  }
  if (mqtt !== "avf/devices") {
    throw new Error("postman-avf: production requires mqtt_topic_prefix=avf/devices");
  }
}
const method = (pm.request.method || "GET").toUpperCase();
const isWrite = ["POST", "PUT", "PATCH", "DELETE"].indexOf(method) >= 0;
if (isWrite && isProd) {
  const a = pm.environment.get("allow_mutation");
  const b = pm.environment.get("allow_production_mutation");
  const c = pm.environment.get("confirm_production_run");
  if (a !== "true" || b !== "true" || c !== "I_UNDERSTAND_PRODUCTION_MUTATION") {
    throw new Error(
      "postman-avf: production mutating request blocked. Set allow_mutation, allow_production_mutation, confirm_production_run on the production environment to unlock (dangerous).",
    );
  }
}
const mode = (
  pm.environment.get("auth_type") ||
  pm.collectionVariables.get("auth_type") ||
  "public"
).toLowerCase();
let active = "";
if (mode === "admin") {
  active =
    pm.environment.get("admin_token") ||
    pm.collectionVariables.get("admin_token") ||
    "";
} else if (mode === "machine") {
  active =
    pm.environment.get("machine_token") ||
    pm.collectionVariables.get("machine_token") ||
    "";
}
pm.collectionVariables.set("active_token", active);
if (active) {
  pm.request.headers.upsert({ key: "Authorization", value: "Bearer " + active });
} else {
  pm.request.headers.remove("Authorization");
}

/* global pm */
function uuid4() {
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, function (c) {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}
/** RFC 9562 UUID v7 for client-supplied internal resource IDs (Postman canary bodies). */
function uuid7() {
  let t = Date.now();
  const b = new Uint8Array(16);
  for (let i = 5; i >= 0; i--) {
    b[i] = t & 0xff;
    t = Math.floor(t / 256);
  }
  for (let i = 6; i < 16; i++) {
    b[i] = (Math.random() * 256) | 0;
  }
  b[6] = (b[6] & 0x0f) | 0x70;
  b[8] = (b[8] & 0x3f) | 0x80;
  const hex = Array.from(b, function (n) {
    return n.toString(16).padStart(2, "0");
  }).join("");
  return (
    hex.slice(0, 8) +
    "-" +
    hex.slice(8, 12) +
    "-" +
    hex.slice(12, 16) +
    "-" +
    hex.slice(16, 20) +
    "-" +
    hex.slice(20)
  );
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
const resourceUUID = uuid7();
const nowIso = new Date().toISOString();
pm.collectionVariables.set("resource_uuid", resourceUUID);
pm.collectionVariables.set("x_request_id", reqId);
pm.collectionVariables.set("x_correlation_id", corr);
pm.collectionVariables.set("idempotency_key", idem);
pm.collectionVariables.set("event_id", evId);
pm.collectionVariables.set("event_time", nowIso);
pm.collectionVariables.set("now_iso", nowIso);
try {
  const at = pm.environment.get("accessToken") || pm.environment.get("admin_token");
  if (at) pm.collectionVariables.set("admin_token", String(at));
  const mt = pm.environment.get("machineToken") || pm.environment.get("machine_token");
  if (mt) pm.collectionVariables.set("machine_token", String(mt));
} catch (e) {}
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
const baseRaw =
  pm.environment.get("baseUrl") ||
  pm.environment.get("base_url") ||
  pm.collectionVariables.get("base_url") ||
  "";
const base = baseRaw.toLowerCase();
const pay = (pm.environment.get("payment_env") || "").toLowerCase();
const mqtt = (
  pm.environment.get("mqtt_topic_prefix") ||
  pm.environment.get("mqttTopicPrefix") ||
  ""
).trim();
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
const isLocalDev =
  appEnv === "development" ||
  /localhost/.test(base) ||
  /127\.0\.0\.1/.test(base);
if (isWrite && isLocalDev && method === "POST") {
  let urlStr = "";
  try {
    urlStr = pm.request.url.toString();
  } catch (e) {
    urlStr = String(pm.request.url || "");
  }
  const u = urlStr.replace(/\\/g, "/").toLowerCase();
  if (u.includes("/v1/admin/sites") && !/\/v1\/admin\/sites\//.test(u)) {
    const ad = pm.environment.get("allow_destructive");
    const cm = pm.environment.get("canaryMode");
    if (ad !== "true" && cm !== "true") {
      throw new Error(
        "postman-avf: local POST /v1/admin/sites requires allow_destructive=true or canaryMode=true",
      );
    }
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
    pm.environment.get("accessToken") ||
    pm.collectionVariables.get("accessToken") ||
    pm.environment.get("admin_token") ||
    pm.collectionVariables.get("admin_token") ||
    "";
} else if (mode === "machine") {
  active =
    pm.environment.get("machineToken") ||
    pm.collectionVariables.get("machineToken") ||
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

import http from "k6/http";
import { check, sleep } from "k6";
import { Trend, Rate } from "k6/metrics";

export const restLatency = new Trend("avf_load_rest_latency_ms");
export const restErrors = new Rate("avf_load_rest_errors");

const baseURL = (__ENV.API_BASE_URL || "http://localhost:8080").replace(/\/+$/, "");
const token = __ENV.ADMIN_JWT || "";
const scenario = __ENV.SCENARIO || "smoke";

const targets = {
  smoke: { vus: 1, duration: "30s" },
  "100": { vus: 10, duration: "5m" },
  "500": { vus: 50, duration: "10m" },
  "1000": { vus: 100, duration: "15m" },
  webhook_burst: { vus: 50, duration: "2m" },
};

export const options = {
  scenarios: {
    admin_rest_crud_smoke: {
      executor: "constant-vus",
      vus: (targets[scenario] || targets.smoke).vus,
      duration: (targets[scenario] || targets.smoke).duration,
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.05"],
    http_req_duration: ["p(95)<750", "p(99)<1500"],
  },
};

function headers() {
  const h = { "content-type": "application/json" };
  if (token) h.authorization = `Bearer ${token}`;
  return h;
}

function record(res) {
  restLatency.add(res.timings.duration);
  restErrors.add(res.status >= 400);
}

export default function () {
  const paths = [
    "/health/live",
    "/health/ready",
    "/v1/admin/machines?limit=10&offset=0",
    "/v1/admin/sites?limit=10&offset=0",
    "/v1/admin/technicians?limit=10&offset=0",
    "/v1/admin/ops/outbox?limit=10&offset=0",
    "/v1/admin/ops/retention",
  ];
  for (const path of paths) {
    const res = http.get(`${baseURL}${path}`, { headers: headers(), tags: { route: path } });
    record(res);
    check(res, {
      "status is not 5xx": (r) => r.status < 500,
      "auth present for admin routes": (r) => !path.startsWith("/v1/admin") || token || r.status === 401,
    });
  }
  sleep(1);
}

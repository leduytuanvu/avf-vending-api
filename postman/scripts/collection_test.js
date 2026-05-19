/* global pm */
pm.test("Response time < 30s", function () {
  if (pm.response.responseTime >= 30000) {
    throw new Error("response too slow: " + pm.response.responseTime + "ms");
  }
});
function tryJson() {
  try {
    return pm.response.json();
  } catch (e) {
    return null;
  }
}
function envSet(k, v) {
  if (v === undefined || v === null || v === "") {
    return;
  }
  try {
    pm.environment.set(k, String(v));
  } catch (e) {}
}
const j = tryJson();
if (!j) {
  return;
}
const save = (val, vname) => {
  if (val === undefined || val === null || val === "") {
    return;
  }
  pm.collectionVariables.set(vname, String(val));
};
const tok = j.tokens || {};
if (tok.accessToken) {
  save(tok.accessToken, "admin_token");
  envSet("accessToken", tok.accessToken);
}
if (tok.refreshToken) {
  envSet("refreshToken", tok.refreshToken);
}
if (tok.accessToken || j.accessToken) {
  envSet("auth_type", "admin");
}
if (j.accessToken) {
  save(j.accessToken, "admin_token");
  envSet("accessToken", j.accessToken);
}
if (j.access_token) {
  save(j.access_token, "admin_token");
  envSet("accessToken", j.access_token);
}
if (j.token) {
  save(j.token, "admin_token");
  envSet("accessToken", j.token);
}
if (j.machineToken) {
  save(j.machineToken, "machine_token");
  envSet("machineToken", j.machineToken);
}
if (j.machine_token) {
  save(j.machine_token, "machine_token");
  envSet("machineToken", j.machine_token);
}
const cred = j.credentials || {};
if (cred.machineToken) {
  save(cred.machineToken, "machine_token");
  envSet("machineToken", cred.machineToken);
}
if (cred.accessToken) {
  save(cred.accessToken, "machine_token");
  envSet("machineToken", cred.accessToken);
}
if (j.mediaId) {
  save(j.mediaId, "media_id");
  envSet("mediaId", j.mediaId);
}
if (j.activationCode) {
  save(j.activationCode, "activation_code");
  envSet("activationCode", j.activationCode);
}
if (j.order_id) {
  save(j.order_id, "order_id");
  envSet("orderId", j.order_id);
}
if (j.catalogVersion !== undefined && j.catalogVersion !== null) {
  envSet("catalogVersion", String(j.catalogVersion));
}
if (j.mediaManifestVersion !== undefined && j.mediaManifestVersion !== null) {
  envSet("mediaManifestVersion", String(j.mediaManifestVersion));
}
if (j.machineId) {
  save(j.machineId, "machine_id");
  envSet("machineId", j.machineId);
}
if (j.siteId) {
  save(j.siteId, "site_id");
  envSet("siteId", j.siteId);
}
if (j.orderId) {
  save(j.orderId, "order_id");
  envSet("orderId", j.orderId);
}
if (j.paymentId) {
  save(j.paymentId, "payment_id");
}
if (j.vendId) {
  save(j.vendId, "vend_id");
}
if (j.refundId) {
  save(j.refundId, "refund_id");
}
if (j.machine && j.machine.id) {
  save(j.machine.id, "machine_id");
  envSet("machineId", j.machine.id);
}
if (j.data) {
  const d = j.data;
  if (d.machineId) {
    save(d.machineId, "machine_id");
    envSet("machineId", d.machineId);
  }
  if (d.orderId) {
    save(d.orderId, "order_id");
    envSet("orderId", d.orderId);
  }
  if (d.accessToken) {
    save(d.accessToken, "admin_token");
    envSet("accessToken", d.accessToken);
  }
}
if (j.order && j.order.id) {
  save(j.order.id, "order_id");
  envSet("orderId", j.order.id);
}
if (j.payment && j.payment.id) {
  save(j.payment.id, "payment_id");
}
if (j.vend && j.vend.id) {
  save(j.vend.id, "vend_id");
}
if (j.refund && j.refund.id) {
  save(j.refund.id, "refund_id");
}
const method = (pm.request.method || "GET").toUpperCase();
let pathname = "";
try {
  const parts = pm.request.url.path;
  if (parts && parts.length) {
    pathname = "/" + parts.join("/");
  }
} catch (e) {
  pathname = "";
}
if (
  method === "POST" &&
  pm.response.code >= 200 &&
  pm.response.code < 300 &&
  j &&
  j.id
) {
  const tail = pathname.replace(/^\/+/, "");
  const id = String(j.id);
  if (tail === "v1/admin/categories") {
    save(id, "category_id");
    envSet("categoryId", id);
  } else if (tail === "v1/admin/brands") {
    save(id, "brand_id");
    envSet("brandId", id);
  } else if (tail === "v1/admin/tags") {
    save(id, "tag_id");
    envSet("tagId", id);
  } else if (tail === "v1/admin/products") {
    save(id, "product_id");
    envSet("productId", id);
  } else if (tail === "v1/admin/sites") {
    save(id, "site_id");
    envSet("siteId", id);
  } else if (tail === "v1/admin/machines") {
    save(id, "machine_id");
    envSet("machineId", id);
  }
}

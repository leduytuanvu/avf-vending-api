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
if (j.accessToken) {
  save(j.accessToken, "admin_token");
}
if (j.access_token) {
  save(j.access_token, "admin_token");
}
if (j.token) {
  save(j.token, "admin_token");
}
if (j.machineToken) {
  save(j.machineToken, "machine_token");
}
if (j.machine_token) {
  save(j.machine_token, "machine_token");
}
const cred = j.credentials || {};
if (cred.machineToken) {
  save(cred.machineToken, "machine_token");
}
if (cred.accessToken) {
  save(cred.accessToken, "machine_token");
}
if (j.machineId) {
  save(j.machineId, "machine_id");
}
if (j.organizationId) {
  save(j.organizationId, "organization_id");
}
if (j.siteId) {
  save(j.siteId, "site_id");
}
if (j.orderId) {
  save(j.orderId, "order_id");
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
}
if (j.data) {
  const d = j.data;
  if (d.machineId) {
    save(d.machineId, "machine_id");
  }
  if (d.orderId) {
    save(d.orderId, "order_id");
  }
  if (d.accessToken) {
    save(d.accessToken, "admin_token");
  }
}
if (j.order && j.order.id) {
  save(j.order.id, "order_id");
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

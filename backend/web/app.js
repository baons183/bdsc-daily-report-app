// Khớp đúng bố cục bảng KPI trong file Word gốc: TT/Nhóm gộp dọc theo nhóm,
// "groupSize" chỉ đặt ở chỉ tiêu đầu nhóm (số dòng cần rowspan).
// "auto: true" = Thực hiện tự đếm từ mục Sự cố (server tính, không cho gõ
// tay — xem report.KPIAutoCountMap phía Go).
const KPI_CATEGORIES = [
  { tt: 1, nhom: "Sự cố", chiTieu: "Sự cố", dvt: "Vụ", groupSize: 3, auto: true },
  { tt: 1, nhom: "Sự cố", chiTieu: "Near Miss", dvt: "Vụ", auto: true },
  { tt: 1, nhom: "Sự cố", chiTieu: "Technical event", dvt: "Vụ", auto: true },
  { tt: 2, nhom: "Chất lượng", chiTieu: "Rework", dvt: "WO", groupSize: 1 },
  { tt: 3, nhom: "Hỏng thiết bị", chiTieu: "Dừng ngoài kế hoạch", dvt: "Vụ", groupSize: 1 },
];

let TOKEN = localStorage.getItem("bdsc_token") || "";
let ROLES = localStorage.getItem("bdsc_roles") || "";
let FULLNAME = localStorage.getItem("bdsc_fullname") || "";
let EMPCODE = localStorage.getItem("bdsc_empcode") || "";

// Trả về true nếu người dùng hiện tại được sửa/xóa dòng do `createdBy` tạo
// (chính chủ hoặc Admin) — khớp đúng luật phía server (checkOwnership).
// Sửa: chính chủ, Admin, hoặc Moderator (Moderator sửa được của người khác
// nhưng không được xóa — khớp đúng luật phía server, xem checkEditOwnership).
function canEdit(createdBy) {
  return hasRole("Admin") || hasRole("Moderator") || createdBy === EMPCODE;
}
// Xóa: chỉ chính chủ, Admin, hoặc Approver — khớp checkOwnership phía server.
function canDelete(createdBy) {
  return hasRole("Admin") || hasRole("Approver") || createdBy === EMPCODE;
}

// Ô Thiết bị dạng select có gõ-tìm (input + <datalist>, trình duyệt tự vẽ
// mũi tên/danh sách xổ xuống như 1 select thật): gõ từ 2 ký tự trở lên,
// debounce 250ms, nạp tối đa 20 gợi ý từ Asset_Master (155k+ thiết bị) vào
// datalist tương ứng.
function attachAssetAutocomplete(formEl) {
  const input = formEl.querySelector('input[list^="assets-"]');
  if (!input) return;
  const datalist = document.getElementById(input.getAttribute("list"));
  let timer = null;

  input.addEventListener("input", () => {
    clearTimeout(timer);
    const q = input.value.trim();
    if (q.length < 2) { datalist.innerHTML = ""; return; }
    timer = setTimeout(async () => {
      let hits = [];
      try { hits = (await api("/api/assets/search?q=" + encodeURIComponent(q))) || []; } catch (e) { return; }
      datalist.innerHTML = hits.map(h =>
        `<option value="${esc(h.tagName)}">${esc((h.equipDescription || "").substring(0, 90))}</option>`
      ).join("");
    }, 250);
  });
}

// Khớp đúng auth.HasRole phía server: Admin luôn qua; Approver ngang Admin
// ở MỌI role khác, trừ chính "Admin" (không vào được tab/API Quản trị).
function hasRole(r) {
  const roles = ROLES.split(",").map(s => s.trim());
  if (roles.includes(r) || roles.includes("Admin")) return true;
  return r !== "Admin" && roles.includes("Approver");
}
function hasAnyRole(roles) {
  return roles.some(hasRole);
}

async function api(path, opts = {}) {
  opts.headers = opts.headers || {};
  if (TOKEN) opts.headers["Authorization"] = "Bearer " + TOKEN;
  const res = await fetch(path, opts);
  if (res.status === 401) {
    // Token het han hoac khong hop le - quay lai man hinh dang nhap thay vi
    // de nguoi dung thay 1 loi chung chung roi ket ket tren trang cu.
    logout();
    throw new Error("Phiên đăng nhập đã hết hạn, vui lòng đăng nhập lại.");
  }
  if (!res.ok) {
    const t = await res.text();
    throw new Error(t || res.statusText);
  }
  const ct = res.headers.get("content-type") || "";
  return ct.includes("application/json") ? res.json() : res.text();
}

async function doLogin() {
  const employeeCode = document.getElementById("loginCode").value.trim();
  const password = document.getElementById("loginPw").value;
  const errBox = document.getElementById("loginErr");
  errBox.textContent = "";
  try {
    const data = await api("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ employeeCode, password }),
    });
    TOKEN = data.token; ROLES = data.roles; FULLNAME = data.fullName; EMPCODE = data.employeeCode;
    localStorage.setItem("bdsc_token", TOKEN);
    localStorage.setItem("bdsc_roles", ROLES);
    localStorage.setItem("bdsc_fullname", FULLNAME);
    localStorage.setItem("bdsc_empcode", EMPCODE);
    showApp();
  } catch (e) {
    errBox.textContent = "Đăng nhập thất bại: " + e.message;
  }
}

function logout() {
  localStorage.clear();
  location.reload();
}

function openRegister() {
  document.getElementById("regCode").value = "";
  document.getElementById("regPw").value = "";
  document.getElementById("regErr").textContent = "";
  document.getElementById("registerModal").hidden = false;
}
function closeRegister() {
  document.getElementById("registerModal").hidden = true;
}
// "Đăng ký" no longer creates a new account/password - it verifies the
// caller's EXISTING Maintenance Report App/Checksheet credentials and grants
// Viewer access to this app if they don't have a role here yet (see
// RegisterViewer in auth_handlers.go).
async function submitRegister() {
  const employeeCode = document.getElementById("regCode").value.trim();
  const password = document.getElementById("regPw").value;
  const errBox = document.getElementById("regErr");
  errBox.textContent = "";
  if (!employeeCode || !password) { errBox.textContent = "Vui lòng nhập đủ Mã nhân viên/Mật khẩu."; return; }
  try {
    await api("/api/register", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ employeeCode, password }),
    });
  } catch (e) {
    errBox.textContent = "Lỗi: " + e.message;
    return;
  }
  closeRegister();
  alert("Đã cấp quyền dùng app. Vui lòng đăng nhập.");
  document.getElementById("loginCode").value = employeeCode;
}

function showApp() {
  document.getElementById("loginScreen").style.display = "none";
  document.getElementById("app").style.display = "block";
  document.getElementById("whoami").textContent = FULLNAME || "";
  buildTabs();
}

const TAB_DEFS = [
  { id: "kpi", label: "KPI - Sự cố", role: ["KPI", "SuCo", "Moderator"] },
  { id: "congviec", label: "Công việc", role: ["CongViec", "Moderator"] },
  { id: "giamsat", label: "Giám sát/Chẩn đoán", role: ["GiamSat", "Moderator"] },
  { id: "cbm", label: "Giám sát trực tuyến", role: "GiamSat" },
  { id: "rsm", label: "Reliability", role: ["GiamSat", "Moderator"] },
  { id: "thongke", label: "Báo cáo ngày", role: null },
  { id: "weekly", label: "Báo cáo tuần", role: null },
  { id: "admin", label: "Quản trị", role: "Admin" },
];

// Tài khoản tự đăng ký chỉ có duy nhất role "Viewer" - chỉ được xem 2 tab
// nay, du cac tab khac co role: null (nghia la "ai dang nhap cung xem
// duoc", vd Bao cao ngay/Bao cao tuan) vi Viewer thi khong duoc tinh la
// "ai cung xem duoc" nhu cac role thao tac khac.
const VIEWER_ONLY_TABS = ["thongke", "weekly"];
function isViewerOnly() {
  const roles = ROLES.split(",").map(s => s.trim()).filter(Boolean);
  return roles.length > 0 && roles.every(r => r === "Viewer");
}

function buildTabs() {
  const nav = document.getElementById("tabs");
  nav.innerHTML = "";
  let first = null;
  TAB_DEFS.forEach(t => {
    if (t.role && !(Array.isArray(t.role) ? hasAnyRole(t.role) : hasRole(t.role))) return;
    if (isViewerOnly() && !VIEWER_ONLY_TABS.includes(t.id)) return;
    if (!first) first = t.id;
    const btn = document.createElement("button");
    btn.textContent = t.label;
    btn.onclick = () => switchTab(t.id);
    btn.dataset.tab = t.id;
    nav.appendChild(btn);
  });
  if (first) switchTab(first);
}

function switchTab(id) {
  document.querySelectorAll("#tabs button").forEach(b => b.classList.toggle("active", b.dataset.tab === id));
  document.querySelectorAll(".panel").forEach(p => p.classList.toggle("active", p.id === "panel-" + id));
  const loaders = { giamsat: loadGiamSat, congviec: loadCongViec, cbm: loadCBM, rsm: initRSMReview, thongke: initThongKe, weekly: initWeeklyReport, kpi: loadKPISuCoPanel, admin: loadAdminPanel };
  if (loaders[id]) loaders[id]().catch(e => alert("Lỗi tải dữ liệu: " + e.message));
}

function fd2obj(form) {
  const obj = {};
  new FormData(form).forEach((v, k) => { if (k !== "photos") obj[k] = v; });
  return obj;
}

// Cache dòng dữ liệu mới tải để nút "Sửa" có sẵn dữ liệu điền lại vào form
// mà không cần thêm API lấy 1 dòng riêng.
const ROWS_CACHE = { suco: {}, giamsat: {}, congviec: {}, rsm: {} };

// Ô Sửa/Xóa chỉ hiện nếu chính chủ tạo dòng đó hoặc là Admin — khớp luật
// checkOwnership phía server (server vẫn chặn lại nếu cố tình gọi API thẳng).
function actionButtons(kind, r) {
  let html = "";
  if (canEdit(r.createdBy)) {
    html += `<button class="btn" style="margin-top:0" onclick="edit${kind}(${r.id})">Sửa</button> `;
  }
  if (canDelete(r.createdBy)) {
    html += `<button class="btn danger" onclick="delRow('${kind.toLowerCase()}', ${r.id})">Xóa</button>`;
  }
  return html;
}

function enterEditMode(form, id, values, submitLabel) {
  form.dataset.editId = id;
  Object.keys(values).forEach(k => {
    const el = form.elements[k];
    if (el) el.value = values[k] ?? "";
  });
  const submitBtn = form.querySelector('button[type=submit]');
  submitBtn.textContent = submitLabel || "Cập nhật";
  let cancelBtn = form.querySelector(".cancel-edit-btn");
  if (!cancelBtn) {
    cancelBtn = document.createElement("button");
    cancelBtn.type = "button";
    cancelBtn.className = "btn cancel-edit-btn";
    cancelBtn.style.cssText = "background:#888;margin-left:8px;margin-top:8px";
    cancelBtn.textContent = "Hủy sửa";
    cancelBtn.onclick = () => exitEditMode(form);
    submitBtn.insertAdjacentElement("afterend", cancelBtn);
  }
  cancelBtn.hidden = false;
  form.scrollIntoView({ behavior: "smooth", block: "start" });
}
function exitEditMode(form) {
  delete form.dataset.editId;
  form.reset();
  form.querySelector('button[type=submit]').textContent = "Thêm dòng";
  const cancelBtn = form.querySelector(".cancel-edit-btn");
  if (cancelBtn) cancelBtn.hidden = true;
}

// ---- Sự cố ----
async function loadSuCo() {
  // Moderator xem/sửa được nhưng không tạo dòng mới — ẩn form thêm dòng.
  document.getElementById("form-suco").hidden = !hasRole("SuCo");
  const rows = (await api("/api/suco")) || [];
  ROWS_CACHE.suco = Object.fromEntries(rows.map(r => [r.id, r]));
  const t = document.getElementById("table-suco");
  t.innerHTML = `<tr><th>TT</th><th>Thiết bị</th><th>Phòng</th><th>Loại sự cố</th><th>Tình trạng</th><th>TG dừng PX (h)</th><th>TG dừng TB (h)</th>
      <th>Phương án xử lý</th><th>Đơn vị thực hiện</th><th>Thời hạn</th><th>Đề xuất</th><th>Lưu ý</th><th></th></tr>` +
    rows.map(r => `<tr>
      <td>${r.tt}</td><td>${esc(r.thietBi)}</td><td>${esc(r.dm)}</td><td>${esc(r.loaiSuCo)}</td><td>${esc(r.tinhTrang)}</td>
      <td>${r.thoiGianDungPX ?? ""}</td><td>${r.thoiGianDungTB ?? ""}</td>
      <td>${esc(r.phuongAn)}</td><td>${esc(r.donVi)}</td><td>${r.thoiHan ? r.thoiHan.substring(0,10) : ""}</td>
      <td>${esc(r.deXuat)}</td><td>${esc(r.luuY)}</td>
      <td>${actionButtons("SuCo", r)}</td>
    </tr>`).join("");
}
function editSuCo(id) {
  const r = ROWS_CACHE.suco[id];
  if (!r) return;
  enterEditMode(document.getElementById("form-suco"), id, {
    thietBi: r.thietBi, dm: r.dm, loaiSuCo: r.loaiSuCo, tinhTrang: r.tinhTrang,
    thoiGianDungPX: r.thoiGianDungPX ?? "", thoiGianDungTB: r.thoiGianDungTB ?? "",
    phuongAn: r.phuongAn, donVi: r.donVi, thoiHan: r.thoiHan ? r.thoiHan.substring(0, 10) : "",
    deXuat: r.deXuat, luuY: r.luuY,
  });
}
document.getElementById("form-suco").addEventListener("submit", async e => {
  e.preventDefault();
  const form = e.target;
  const body = fd2obj(form);
  ["thoiGianDungPX","thoiGianDungTB"].forEach(k => body[k] = body[k] ? Number(body[k]) : null);
  const editId = form.dataset.editId;
  if (editId) {
    await api(`/api/suco/${editId}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    exitEditMode(form);
  } else {
    await api("/api/suco", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    form.reset();
  }
  loadSuCo();
  refreshKPIAutoCells(); // KPI Sự cố/Near Miss/Technical event tự đếm từ đây, cần cập nhật theo
});

// ---- Giám sát ----
async function loadGiamSat() {
  // Moderator xem/sửa được nhưng không tạo dòng mới — ẩn form thêm dòng.
  document.getElementById("form-giamsat").hidden = !hasRole("GiamSat");
  const rows = (await api("/api/giamsat")) || [];
  ROWS_CACHE.giamsat = Object.fromEntries(rows.map(r => [r.id, r]));
  const t = document.getElementById("table-giamsat");
  t.innerHTML = `<tr><th>TT</th><th>Thiết bị</th><th>Phòng</th><th>Tình trạng</th><th>Rủi ro</th><th>Phương án xử lý</th>
      <th>Đơn vị thực hiện</th><th>Thời hạn</th><th>Đề xuất</th><th>Lưu ý</th><th></th></tr>` +
    rows.map(r => `<tr>
      <td>${r.tt}</td><td>${esc(r.thietBi)}</td><td>${esc(r.dm)}</td><td>${esc(r.tinhTrang)}</td>
      <td>${esc(r.ruiRo)}</td><td>${esc(r.phuongAn)}</td><td>${esc(r.donVi)}</td><td>${r.thoiHan ? r.thoiHan.substring(0,10) : ""}</td>
      <td>${esc(r.deXuat)}</td><td>${esc(r.luuY)}</td>
      <td>${actionButtons("GiamSat", r)}</td>
    </tr>`).join("");
}
function editGiamSat(id) {
  const r = ROWS_CACHE.giamsat[id];
  if (!r) return;
  enterEditMode(document.getElementById("form-giamsat"), id, {
    thietBi: r.thietBi, dm: r.dm, tinhTrang: r.tinhTrang, ruiRo: r.ruiRo,
    phuongAn: r.phuongAn, donVi: r.donVi, thoiHan: r.thoiHan ? r.thoiHan.substring(0, 10) : "",
    deXuat: r.deXuat, luuY: r.luuY,
  });
}
document.getElementById("form-giamsat").addEventListener("submit", async e => {
  e.preventDefault();
  const form = e.target;
  const body = fd2obj(form);
  const editId = form.dataset.editId;
  if (editId) {
    await api(`/api/giamsat/${editId}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    exitEditMode(form);
  } else {
    await api("/api/giamsat", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    form.reset();
  }
  loadGiamSat();
});

// ---- Công việc (multipart, có ảnh) ----
async function loadCongViec() {
  // Moderator xem/sửa được nhưng không tạo dòng mới — ẩn form thêm dòng.
  document.getElementById("form-congviec").hidden = !hasRole("CongViec");
  await Promise.all([loadCongViecToday(), loadCongViecYesterday()]);
}
async function loadCongViecToday() {
  const rows = (await api("/api/congviec")) || [];
  ROWS_CACHE.congviec = Object.fromEntries(rows.map(r => [r.id, r]));
  const t = document.getElementById("table-congviec");
  t.innerHTML = `<tr><th>TT</th><th>Thiết bị</th><th>Phòng</th><th>Nội dung</th><th>Kế hoạch</th><th>Tiến độ (%)</th>
      <th>Hôm nay</th><th>Ngày mai</th><th>Đề xuất</th><th>Lưu ý</th><th>Ảnh</th><th></th></tr>` +
    rows.map(r => `<tr>
      <td>${r.tt}</td><td>${esc(r.thietBi)}</td><td>${esc(r.dm)}</td><td>${esc(r.noiDung)}</td><td>${esc(r.keHoach)}</td><td>${r.tienDo ?? ""}</td>
      <td>${esc(r.homNay)}</td><td>${esc(r.ngayMai)}</td><td>${esc(r.deXuat)}</td><td>${esc(r.luuY)}</td>
      <td>${(r.attachments||[]).map(a => `<img class="thumb" src="/uploads/${a.filePath}">`).join("")}</td>
      <td>${actionButtons("CongViec", r)}</td>
    </tr>`).join("");
}
// Danh sách Công việc hôm qua, chỉ để xem/tái sử dụng - không sửa/xoá được
// ở đây (đã bị khoá theo checkNotLocked nếu ngày đó đã Kiểm tra rồi, và dù
// gì cũng không phải trọng tâm của bảng này). Bấm "Dùng cho báo cáo hôm
// nay" chỉ điền vào form thêm dòng, không tự lưu gì cả.
let CONGVIEC_YESTERDAY_CACHE = {};
async function loadCongViecYesterday() {
  const rows = (await api("/api/congviec/yesterday")) || [];
  CONGVIEC_YESTERDAY_CACHE = Object.fromEntries(rows.map(r => [r.id, r]));
  const t = document.getElementById("table-congviec-yesterday");
  t.innerHTML = `<tr><th>Thiết bị</th><th>Phòng</th><th>Nội dung</th><th>Kế hoạch</th><th>Tiến độ (%)</th>
      <th>Hôm nay</th><th>Ngày mai</th><th></th></tr>` +
    (rows.length === 0
      ? '<tr><td colspan="8" style="text-align:center;color:#888">Không có dữ liệu hôm qua</td></tr>'
      : rows.map(r => `<tr>
          <td>${esc(r.thietBi)}</td><td>${esc(r.dm)}</td><td>${esc(r.noiDung)}</td><td>${esc(r.keHoach)}</td><td>${r.tienDo ?? ""}</td>
          <td>${esc(r.homNay)}</td><td>${esc(r.ngayMai)}</td>
          <td><button class="btn" style="margin-top:0" onclick="reuseCongViecYesterday(${r.id})">Dùng cho báo cáo hôm nay</button></td>
        </tr>`).join(""));
}
function reuseCongViecYesterday(id) {
  const r = CONGVIEC_YESTERDAY_CACHE[id];
  if (!r) return;
  const form = document.getElementById("form-congviec");
  exitEditMode(form); // đảm bảo không đang ở chế độ Sửa dòng khác - đây luôn là thêm dòng mới
  ["thietBi", "dm", "noiDung", "keHoach", "tienDo", "homNay", "ngayMai", "deXuat", "luuY"].forEach(k => {
    const el = form.elements[k];
    if (el) el.value = r[k] ?? "";
  });
  form.scrollIntoView({ behavior: "smooth", block: "start" });
}
function editCongViec(id) {
  const r = ROWS_CACHE.congviec[id];
  if (!r) return;
  enterEditMode(document.getElementById("form-congviec"), id, {
    thietBi: r.thietBi, dm: r.dm, noiDung: r.noiDung, keHoach: r.keHoach, tienDo: r.tienDo ?? "",
    homNay: r.homNay, ngayMai: r.ngayMai, deXuat: r.deXuat, luuY: r.luuY,
  });
  alert("Ảnh cũ vẫn giữ nguyên. Nếu chọn thêm ảnh mới ở đây, ảnh mới sẽ được thêm vào (không thay ảnh cũ).");
}
document.getElementById("form-congviec").addEventListener("submit", async e => {
  e.preventDefault();
  const form = e.target;
  const fd = new FormData(form);
  const editId = form.dataset.editId;
  if (editId) {
    await api(`/api/congviec/${editId}`, { method: "PUT", body: fd });
    exitEditMode(form);
  } else {
    await api("/api/congviec", { method: "POST", body: fd });
    form.reset();
  }
  loadCongViec();
});

["form-suco", "form-giamsat", "form-congviec", "form-rsm"].forEach(id => {
  attachAssetAutocomplete(document.getElementById(id));
});


// ---- Giám sát trực tuyến (CBM) — chỉ xem, dữ liệu tự động từ hệ thống ngoài ----
// Chỉ GiamSat/Moderator/Approver/Admin mới được tick "Đưa vào báo cáo" —
// khớp đúng luật phía server (SetCBMExclusion), tab CBM thì ai đăng nhập
// cũng xem được (role: null) nhưng quyết định dữ liệu nào vào báo cáo thật
// thì không phải hành động chỉ-xem.
function canToggleCBMInclude() {
  return hasRole("GiamSat") || hasRole("Moderator") || hasRole("Approver");
}
async function loadCBM() {
  const dateInput = document.getElementById("cbm-date");
  if (!dateInput.value) dateInput.value = new Date().toISOString().slice(0, 10);
  const rows = (await api("/api/cbm?date=" + encodeURIComponent(dateInput.value))) || [];
  window.CBM_ROWS_CACHE = rows;
  const canToggle = canToggleCBMInclude();
  const t = document.getElementById("table-cbm");
  t.innerHTML = "<tr><th>TT</th><th>Thời gian</th><th>Tagname</th><th>Thiết bị</th><th>Mô tả</th><th>Giá trị cảnh báo</th><th>Mức cảnh báo</th><th>Đưa vào báo cáo</th></tr>" +
    (rows.length === 0
      ? '<tr><td colspan="8" style="text-align:center;color:#888">Không có dữ liệu trong khung giờ báo cáo hiện tại</td></tr>'
      : rows.map((r, i) => `<tr>
          <td>${i + 1}</td><td>${fmtDateTime(r.dataTime)}</td><td>${esc(r.tagname)}</td><td>${esc(r.equipment)}</td>
          <td>${esc(r.description)}</td><td>${esc(r.alarmValue)}</td><td>${esc(r.alarmLevel)}</td>
          <td style="text-align:center"><input type="checkbox" ${r.included ? "checked" : ""} ${canToggle ? "" : "disabled"} onchange="toggleCBMInclude(${i}, this.checked)"></td>
        </tr>`).join(""));
}
// Tick/bỏ tick "Đưa vào báo cáo" cho 1 dòng cụ thể — lưu ngay qua API, ảnh
// hưởng tới PDF/email/Báo cáo ngày/Báo cáo tuần lần sinh báo cáo kế tiếp, chứ
// không ẩn dòng khỏi tab Giám sát trực tuyến này (vẫn thấy để tick lại được).
async function toggleCBMInclude(idx, included) {
  const r = (window.CBM_ROWS_CACHE || [])[idx];
  if (!r) return;
  try {
    await api("/api/cbm/exclude", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tagname: r.tagname, description: r.description, dataTime: r.dataTime, included }),
    });
  } catch (e) {
    alert("Lỗi: " + e.message);
    loadCBM();
  }
}
function fmtDateTime(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  const pad = n => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())} ${pad(d.getDate())}/${pad(d.getMonth() + 1)}`;
}

// ---- Reliability (mục 3.3 báo cáo) ----
// Kết hợp nhập tay và tự động: mỗi lần tải tab này, server tự đồng bộ
// thêm các vấn đề mới từ file RSM PDF của "hôm nay" (nếu có) vào cùng
// bảng — dòng tự động (createdBy rỗng) sửa/xoá được như dòng nhập tay,
// không còn cơ chế tick "đưa vào báo cáo" riêng nữa (xoá dòng là cách
// duy nhất để bỏ nó khỏi báo cáo).
async function initRSMReview() {
  const dateInput = document.getElementById("rsm-date");
  if (!dateInput.value) dateInput.value = new Date().toISOString().slice(0, 10);
  return loadRSMReview();
}
async function loadRSMReview() {
  document.getElementById("form-rsm").hidden = !hasRole("GiamSat");
  const date = document.getElementById("rsm-date").value;
  const rows = (await api("/api/reliability?date=" + encodeURIComponent(date))) || [];
  ROWS_CACHE.rsm = Object.fromEntries(rows.map(r => [r.id, r]));
  const t = document.getElementById("table-rsm");
  t.innerHTML = "<tr><th>TT</th><th>Tagname</th><th>Vấn đề phát hiện</th><th>Phương án xử lý</th><th>Đơn vị thực hiện</th><th>Thời hạn</th><th>Đưa vào báo cáo</th><th></th></tr>" +
    rows.map(r => `<tr>
      <td>${r.tt}</td><td>${esc(r.thietBi)}</td><td>${esc(r.vanDe)}</td><td>${esc(r.phuongAn)}</td>
      <td>${esc(r.donVi)}</td><td>${r.thoiHan ? r.thoiHan.substring(0, 10) : ""}</td>
      <td style="text-align:center">${reliabilityIncludeCheckbox(r)}</td>
      <td>${reliabilityActionButtons(r)}</td>
    </tr>`).join("");
}
// Dòng tự động lấy từ RSM PDF có createdBy rỗng (không thuộc về ai cụ
// thể) — cho phép bất kỳ ai có quyền GiamSat sửa/xoá, khác với các bảng
// khác chỉ cho chính chủ hoặc Moderator/Admin (khớp canTouchReliability
// phía server).
function canEditReliability(createdBy) {
  return hasRole("Admin") || hasRole("Approver") || createdBy === "" || createdBy === EMPCODE;
}
// Dòng tự động (createdBy rỗng): xoá không có tác dụng (lần đồng bộ kế
// tiếp sẽ chèn lại y hệt vì key Thiết bị+Vấn đề vẫn còn trong PDF nguồn) -
// nên thay nút Xoá bằng checkbox "Đưa vào báo cáo" (giống Giám sát trực
// tuyến), vẫn sửa được như thường. Dòng nhập tay thì giữ nguyên Sửa/Xoá.
function reliabilityIncludeCheckbox(r) {
  if (r.createdBy !== "") return "";
  const disabled = canEditReliability(r.createdBy) ? "" : "disabled";
  return `<input type="checkbox" ${r.included ? "checked" : ""} ${disabled} onchange="toggleReliabilityInclude(${r.id}, this.checked)">`;
}
async function toggleReliabilityInclude(id, included) {
  try {
    await api(`/api/reliability/${id}/include`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ included }),
    });
  } catch (e) {
    alert("Lỗi: " + e.message);
    loadRSMReview();
  }
}
function reliabilityActionButtons(r) {
  if (!canEditReliability(r.createdBy)) return "";
  const editBtn = `<button class="btn" style="margin-top:0" onclick="editReliability(${r.id})">Sửa</button>`;
  if (r.createdBy === "") return editBtn; // tu dong: khong co nut Xoa, dung checkbox o tren
  return `${editBtn} <button class="btn danger" onclick="delRow('reliability', ${r.id})">Xóa</button>`;
}
function editReliability(id) {
  const r = ROWS_CACHE.rsm[id];
  if (!r) return;
  enterEditMode(document.getElementById("form-rsm"), id, {
    thietBi: r.thietBi, vanDe: r.vanDe, phuongAn: r.phuongAn, donVi: r.donVi,
    thoiHan: r.thoiHan ? r.thoiHan.substring(0, 10) : "",
  });
}
document.getElementById("form-rsm").addEventListener("submit", async e => {
  e.preventDefault();
  const form = e.target;
  const body = fd2obj(form);
  const editId = form.dataset.editId;
  if (editId) {
    await api(`/api/reliability/${editId}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    exitEditMode(form);
  } else {
    await api("/api/reliability", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    form.reset();
    // Dòng mới luôn thuộc cửa sổ "hôm nay" (server dùng window.EntryWindow),
    // bất kể đang lọc xem ngày nào - nhảy bộ lọc về hôm nay để dòng vừa
    // thêm hiện ra ngay, thay vì "biến mất" nếu đang xem 1 ngày cũ.
    document.getElementById("rsm-date").value = new Date().toISOString().slice(0, 10);
  }
  loadRSMReview();
});

// ---- Báo cáo ngày (xem lại báo cáo theo ngày, giao diện giống PDF) ----
async function initThongKe() {
  const dateInput = document.getElementById("thongke-date");
  if (!dateInput.value) dateInput.value = new Date().toISOString().slice(0, 10);
  document.getElementById("thongke-admin-tools").hidden = !hasRole("Admin");
  return loadThongKe();
}

// Ngày báo cáo thực sự đang hiển thị (vd "2026-09-15") - LUÔN lệch 1 ngày
// so với ô "Chọn ngày" (vốn là ngày cửa sổ ĐÓNG, không phải ngày báo cáo -
// xem GetReportView). Nút Kiểm tra/Phê duyệt/Reject phải dùng đúng giá trị
// này (server trả về sẵn qua d.reportDate) thay vì đọc thẳng ô ngày, nếu
// không sẽ đánh dấu nhầm sang ngày hôm sau (cửa sổ chưa đóng).
let THONGKE_REPORT_DATE = "";
async function loadThongKe() {
  const date = document.getElementById("thongke-date").value;
  const content = document.getElementById("thongke-content");
  content.innerHTML = "Đang tải...";
  let data;
  try {
    data = await api("/api/report/view?date=" + encodeURIComponent(date));
  } catch (e) {
    content.innerHTML = "Lỗi: " + e.message;
    return;
  }
  THONGKE_REPORT_DATE = data.reportDate;
  content.innerHTML = renderReportView(data);
}

// ---- Báo cáo tuần (Sự cố + 2 phần Giám sát, khoảng ngày tự chọn) ----
async function initWeeklyReport() {
  const startInput = document.getElementById("weekly-start");
  const endInput = document.getElementById("weekly-end");
  if (!endInput.value) {
    const today = new Date();
    const weekAgo = new Date(today.getTime() - 6 * 24 * 60 * 60 * 1000);
    startInput.value = weekAgo.toISOString().slice(0, 10);
    endInput.value = today.toISOString().slice(0, 10);
  }
  return loadWeeklyReport();
}

async function loadWeeklyReport() {
  const start = document.getElementById("weekly-start").value;
  const end = document.getElementById("weekly-end").value;
  const content = document.getElementById("weekly-content");
  if (!start || !end) { content.innerHTML = "Chọn đủ Từ ngày/Đến ngày."; return; }
  content.innerHTML = "Đang tải...";
  let d;
  try {
    d = await api(`/api/report/weekly?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`);
  } catch (e) {
    content.innerHTML = "Lỗi: " + e.message;
    return;
  }
  content.innerHTML = renderWeeklyReport(d);
}

function renderWeeklyReport(d) {
  const sum = d.sucoSummary || {};
  const cbmList = d.cbm || [];
  const giamsatList = d.giamsat || [];
  const kpiRows = KPI_CATEGORIES.map(c => {
    const e = (d.kpi || []).find(k => k.chiTieu === c.chiTieu) || {};
    return [c.tt, c.nhom, c.chiTieu, c.dvt, e.keHoach ?? 0, e.thucHien ?? 0, e.luyKeThang ?? 0, e.danhGia || "-"].map(esc);
  });
  const sucoRows = (d.suco || []).map(s => [s.tt, s.thietBi, s.dm, s.loaiSuCo, s.tinhTrang, s.thoiGianDungPX ?? "", s.thoiGianDungTB ?? "", s.phuongAn, s.donVi, s.thoiHan ? s.thoiHan.substring(0, 10) : ""].map(esc));
  const giamsatRows = giamsatList.map(s => [s.tt, s.thietBi, s.dm, s.tinhTrang, s.ruiRo, s.phuongAn, s.donVi, s.thoiHan ? s.thoiHan.substring(0, 10) : ""].map(esc));
  const cbmRows = cbmList.map((c, i) => [i + 1, fmtDateTime(c.dataTime), c.tagname, c.equipment, c.description, c.alarmValue, c.alarmLevel].map(esc));
  const reliabilityRows = (d.reliability || []).map(r => [
    esc(r.tt), esc(r.thietBi), esc(r.vanDe), esc(r.phuongAn), esc(r.donVi),
    r.thoiHan ? `<span style="white-space:nowrap">${esc(r.thoiHan.substring(0, 10))}</span>` : "",
  ]);
  const congviecRows = (d.congviec || []).map(c => [c.tt, c.thietBi, c.dm, c.noiDung, c.keHoach, c.tienDo ?? "", c.homNay, c.ngayMai].map(esc));
  const dexuatRows = (d.deXuat || []).map(it => [it.tt, it.thietBi, it.note].map(esc));
  const luuyRows = (d.luuY || []).map(it => [it.tt, it.thietBi, it.note].map(esc));
  const images = d.images || [];

  // Số lượng giám sát thiết bị: tổng số lượt + số thiết bị khác nhau, cho
  // cả 2 phần Giám sát (trực tuyến + chẩn đoán), cộng thêm phân loại theo
  // mức cảnh báo của phần trực tuyến (dữ liệu đã có sẵn, tận dụng luôn).
  const cbmEquip = new Set(cbmList.map(c => c.equipment).filter(Boolean));
  const giamsatEquip = new Set(giamsatList.map(s => s.thietBi).filter(Boolean));
  const allEquip = new Set([...cbmEquip, ...giamsatEquip]);
  const cbmByLevel = {};
  cbmList.forEach(c => { const k = c.alarmLevel || "(khác)"; cbmByLevel[k] = (cbmByLevel[k] || 0) + 1; });
  const cbmLevelKeys = Object.keys(cbmByLevel);

  return `
    <div class="report-view">
      <div class="rv-heading">Các chỉ số KPI (Kế hoạch/Thực hiện cộng dồn cả khoảng đã chọn; Lũy kế tháng tính đến ngày cuối)</div>
      <table><tr><th>TT</th><th>Nhóm</th><th>Chỉ tiêu</th><th>ĐVT</th><th>Kế hoạch</th><th>Thực hiện</th><th>Lũy kế tháng</th><th>Đánh giá</th></tr>
        ${kpiRows.map(r => `<tr>${r.map(c => `<td>${c}</td>`).join("")}</tr>`).join("")}
      </table>

      <div class="rv-heading">Tổng số Sự cố theo loại</div>
      <table><tr><th>Accident</th><th>Near Miss</th><th>Technical Event</th><th>Tổng</th></tr>
        <tr><td>${sum["Accident"] ?? 0}</td><td>${sum["Near Miss"] ?? 0}</td><td>${sum["Technical Event"] ?? 0}</td>
          <td>${(sum["Accident"] ?? 0) + (sum["Near Miss"] ?? 0) + (sum["Technical Event"] ?? 0)}</td></tr>
      </table>

      <div class="rv-heading">Tổng số lượt giám sát thiết bị</div>
      <table><tr><th></th><th>Số lượt</th><th>Số thiết bị</th></tr>
        <tr><td>Giám sát trực tuyến</td><td>${cbmList.length}</td><td>${cbmEquip.size}</td></tr>
        <tr><td>Chẩn đoán chuyên dụng</td><td>${giamsatList.length}</td><td>${giamsatEquip.size}</td></tr>
        <tr><td><b>Tổng</b></td><td><b>${cbmList.length + giamsatList.length}</b></td><td><b>${allEquip.size}</b></td></tr>
      </table>
      ${cbmLevelKeys.length ? `
      <div class="rv-heading">Giám sát trực tuyến theo mức cảnh báo</div>
      <table><tr>${cbmLevelKeys.map(k => `<th>${esc(k)}</th>`).join("")}</tr>
        <tr>${cbmLevelKeys.map(k => `<td>${cbmByLevel[k]}</td>`).join("")}</tr>
      </table>` : ""}

      <div class="rv-heading">GIÁM SÁT TRỰC TUYẾN (ONLINE MONITORING)</div>
      ${rvTable(["TT", "Thời gian", "Tagname", "Thiết bị", "Mô tả", "Giá trị cảnh báo", "Mức cảnh báo"], cbmRows)}

      <div class="rv-heading">GIÁM SÁT OFFLINE BẰNG THIẾT BỊ CHUYÊN DỤNG/ PHÁT HIỆN TẠI HIỆN TRƯỜNG</div>
      ${rvTable(["TT", "Thiết bị", "Phòng", "Tình trạng", "Rủi ro", "Phương án", "Đơn vị", "Thời hạn"], giamsatRows)}

      <div class="rv-heading">VẤN ĐỀ KỸ THUẬT ẢNH HƯỞNG TOÀN VẸN VÀ ĐỘ TIN CẬY THIẾT BỊ</div>
      ${rvTable(["TT", "Thiết bị", "Vấn đề phát hiện", "Phương án xử lý", "Đơn vị thực hiện", "Thời hạn"], reliabilityRows)}

      <div class="rv-heading">Chi tiết Sự cố, vấn đề kỹ thuật ảnh hưởng vận hành</div>
      ${rvTable(["TT", "Thiết bị", "Phòng", "Loại sự cố", "Tình trạng", "PX (h)", "TB (h)", "Phương án", "Đơn vị", "Thời hạn"], sucoRows, "Không có sự cố/vấn đề kỹ thuật ảnh hưởng vận hành.")}

      <div class="rv-heading">Kết quả thực hiện công việc BDSC quan trọng (mỗi thiết bị + phòng chỉ lấy dòng của ngày mới nhất)</div>
      ${rvTable(["TT", "Thiết bị", "Phòng", "Nội dung", "Kế hoạch", "Tiến độ (%)", "Hôm nay", "Ngày mai"], congviecRows)}

      <div class="rv-heading">Vấn đề cần đề xuất</div>
      ${rvTable(["TT", "Thiết bị", "Đề xuất"], dexuatRows, "Không có đề xuất.")}

      <div class="rv-heading">Lưu ý quan trọng</div>
      ${rvTable(["TT", "Thiết bị", "Lưu ý"], luuyRows)}

      <div class="rv-heading">Một số hình ảnh thực hiện công việc tại hiện trường</div>
      ${renderImagesTable(images)}
    </div>
  `;
}

function rvTable(headers, rows, emptyMsg) {
  if (!rows || rows.length === 0) return `<div class="rv-nodata">${emptyMsg || "(Không có dữ liệu)"}</div>`;
  return `<table><tr>${headers.map(h => `<th>${h}</th>`).join("")}</tr>` +
    rows.map(r => `<tr>${r.map(c => `<td>${c}</td>`).join("")}</tr>`).join("") +
    `</table>`;
}

// Bảng ảnh 3 cột (TT | Nội dung | Hình ảnh), gộp ô TT/Nội dung theo nhóm ảnh
// liên tiếp cùng thiết bị — khớp đúng bố cục bảng ảnh trong PDF.
function renderImagesTable(images) {
  if (!images || images.length === 0) return `<div class="rv-nodata">(Không có dữ liệu)</div>`;
  const groups = [];
  images.forEach(img => {
    const last = groups[groups.length - 1];
    if (last && last.thietBi === img.thietBi) last.images.push(img);
    else groups.push({ thietBi: img.thietBi, images: [img] });
  });
  let rows = "";
  groups.forEach((g, gi) => {
    g.images.forEach((img, pi) => {
      rows += "<tr>";
      if (pi === 0) {
        rows += `<td rowspan="${g.images.length}" style="text-align:center">${gi + 1}</td>`;
        rows += `<td rowspan="${g.images.length}">${esc(g.thietBi)}</td>`;
      }
      rows += `<td style="text-align:center"><img src="/uploads/${img.filePath}" style="max-width:350px;max-height:260px"></td>`;
      rows += "</tr>";
    });
  });
  return `<table><tr><th>TT</th><th>Nội dung</th><th>Hình ảnh</th></tr>${rows}</table>`;
}

// Trạng thái Kiểm tra/Phê duyệt hiển thị + nút hành động, dựa trên
// d.approval (report/view trả về, xem reportapproval.go/reportview.go).
// Moderator chỉ thấy "Kiểm tra" khi còn draft; Approver chỉ thấy
// "Phê duyệt"/"Reject" khi đã reviewed — khớp đúng luật phía server.
function renderApprovalBlock(a) {
  a = a || { status: "draft", reviewedByName: "", approvedByName: "" };
  const statusLabel = { draft: "Chưa kiểm tra", reviewed: "Đã kiểm tra, chờ phê duyệt", approved: "Đã phê duyệt" }[a.status] || a.status;
  const buttons = [];
  if (a.status === "draft" && hasRole("Moderator")) {
    buttons.push(`<button class="btn" onclick="doReviewReport()">Kiểm tra</button>`);
  }
  if (a.status === "reviewed" && hasRole("Approver")) {
    buttons.push(`<button class="btn" onclick="doApproveReport()">Phê duyệt</button>`);
    buttons.push(`<button class="btn" style="background:#a33" onclick="doRejectReport()">Reject</button>`);
  }
  return `
    <div class="rv-heading">Trạng thái Kiểm tra / Phê duyệt</div>
    <div style="display:flex; align-items:center; gap:12px; margin-bottom:10px; flex-wrap:wrap;">
      <span><b>${esc(statusLabel)}</b></span>
      ${buttons.join("")}
    </div>
    <table>
      <tr><th style="width:50%">NGƯỜI LẬP</th><th style="width:50%">PHÊ DUYỆT</th></tr>
      <tr><td style="text-align:center">${esc(a.reviewedByName || "")}</td><td style="text-align:center">${esc(a.approvedByName || "")}</td></tr>
    </table>
  `;
}
async function doReviewReport() {
  const date = THONGKE_REPORT_DATE;
  if (!confirm("Xác nhận Kiểm tra báo cáo ngày " + date + "? Sau khi Kiểm tra, dữ liệu ngày này sẽ bị khoá không sửa được nữa (trừ Admin).")) return;
  try {
    await api("/api/report-approval/review", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ date }) });
  } catch (e) { alert("Lỗi: " + e.message); return; }
  loadThongKe();
}
async function doApproveReport() {
  const date = THONGKE_REPORT_DATE;
  if (!confirm("Xác nhận Phê duyệt báo cáo ngày " + date + "?")) return;
  try {
    await api("/api/report-approval/approve", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ date }) });
  } catch (e) { alert("Lỗi: " + e.message); return; }
  loadThongKe();
}
async function doRejectReport() {
  const date = THONGKE_REPORT_DATE;
  if (!confirm("Reject báo cáo ngày " + date + "? Dữ liệu sẽ mở lại để sửa như ban đầu.")) return;
  try {
    await api("/api/report-approval/reject", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ date }) });
  } catch (e) { alert("Lỗi: " + e.message); return; }
  loadThongKe();
}

function renderReportView(d) {
  const kpiRows = KPI_CATEGORIES.map(c => {
    const e = (d.kpi || []).find(k => k.chiTieu === c.chiTieu) || {};
    return [c.tt, c.nhom, c.chiTieu, c.dvt, e.keHoach ?? 0, e.thucHien ?? 0, e.luyKeThang ?? 0, e.danhGia || "-"].map(esc);
  });
  const sucoRows = (d.suco || []).map(s => [s.tt, s.thietBi, s.dm, s.tinhTrang, s.thoiGianDungPX ?? "", s.thoiGianDungTB ?? "", s.phuongAn, s.donVi, s.thoiHan ? s.thoiHan.substring(0, 10) : ""].map(esc));
  const cbmRows = (d.cbm || []).map((c, i) => [i + 1, fmtDateTime(c.dataTime), c.tagname, c.equipment, c.description, c.alarmValue, c.alarmLevel].map(esc));
  const giamsatRows = (d.giamSat || []).map(s => [s.tt, s.thietBi, s.dm, s.tinhTrang, s.ruiRo, s.phuongAn, s.donVi, s.thoiHan ? s.thoiHan.substring(0, 10) : ""].map(esc));
  const reliabilityRows = (d.reliability || []).map(r => [
    esc(r.tt), esc(r.thietBi), esc(r.vanDe), esc(r.phuongAn), esc(r.donVi),
    r.thoiHan ? `<span style="white-space:nowrap">${esc(r.thoiHan.substring(0, 10))}</span>` : "",
  ]);
  const congviecRows = (d.congViec || []).map(c => [c.tt, c.thietBi, c.dm, c.noiDung, c.keHoach, c.tienDo ?? "", c.homNay, c.ngayMai].map(esc));
  const dexuatRows = (d.deXuat || []).map(it => [it.tt, it.thietBi, it.note].map(esc));
  const luuyRows = (d.luuY || []).map(it => [it.tt, it.thietBi, it.note].map(esc));
  const images = d.images || [];

  return `
    <div class="report-view">
      <p class="rv-org">TỔNG CÔNG TY LỌC HÓA DẦU VIỆT NAM</p>
      <p class="rv-company">BAN BẢO DƯỠNG SỬA CHỮA</p>
      <p class="rv-title">BÁO CÁO NGÀY BDSC</p>
      <p class="rv-meta">Ngày: ${esc(d.runAt)} — Dữ liệu từ ${esc(d.start)} đến ${esc(d.end)}</p>

      <div class="rv-heading">1. CÁC CHỈ SỐ KPI</div>
      <table><tr><th>TT</th><th>Nhóm</th><th>Chỉ tiêu</th><th>ĐVT</th><th>Kế hoạch</th><th>Thực hiện</th><th>Lũy kế tháng</th><th>Đánh giá</th></tr>
        ${kpiRows.map(r => `<tr>${r.map(c => `<td>${c}</td>`).join("")}</tr>`).join("")}
      </table>

      <div class="rv-heading">2. KẾT QUẢ GIÁM SÁT, CHẨN ĐOÁN THIẾT BỊ</div>
      <div class="rv-sub">2.1. GIÁM SÁT TRỰC TUYẾN (ONLINE MONITORING)</div>
      ${rvTable(["TT", "Thời gian", "Tagname", "Thiết bị", "Mô tả", "Giá trị cảnh báo", "Mức cảnh báo"], cbmRows)}
      <div class="rv-sub">2.2. GIÁM SÁT OFFLINE BẰNG THIẾT BỊ CHUYÊN DỤNG/ PHÁT HIỆN TẠI HIỆN TRƯỜNG</div>
      ${rvTable(["TT", "Thiết bị", "DM", "Tình trạng", "Rủi ro", "Phương án", "Đơn vị", "Thời hạn"], giamsatRows)}
      <div class="rv-sub">2.3. VẤN ĐỀ KỸ THUẬT ẢNH HƯỞNG TOÀN VẸN VÀ ĐỘ TIN CẬY THIẾT BỊ</div>
      ${rvTable(["TT", "Thiết bị", "Vấn đề phát hiện", "Phương án xử lý", "Đơn vị thực hiện", "Thời hạn"], reliabilityRows)}

      <div class="rv-heading">3. SỰ CỐ, VẤN ĐỀ KỸ THUẬT ẢNH HƯỞNG VHSX</div>
      ${rvTable(["TT", "Thiết bị", "DM", "Tình trạng", "PX (h)", "TB (h)", "Phương án", "Đơn vị", "Thời hạn"], sucoRows, "Không có sự cố/vấn đề kỹ thuật ảnh hưởng vận hành.")}

      <div class="rv-heading">4. KẾT QUẢ THỰC HIỆN CÔNG VIỆC BDSC QUAN TRỌNG</div>
      ${rvTable(["TT", "Thiết bị", "DM", "Nội dung", "Kế hoạch", "Tiến độ (%)", "Hôm nay", "Ngày mai"], congviecRows)}

      <div class="rv-heading">5. VẤN ĐỀ CẦN ĐỀ XUẤT</div>
      ${rvTable(["TT", "Thiết bị", "Đề xuất"], dexuatRows, "Không có đề xuất.")}

      <div class="rv-heading">6. LƯU Ý QUAN TRỌNG</div>
      ${rvTable(["TT", "Thiết bị", "Lưu ý"], luuyRows)}

      <div class="rv-heading">7. MỘT SỐ HÌNH ẢNH THỰC HIỆN CÔNG VIỆC TẠI HIỆN TRƯỜNG</div>
      ${renderImagesTable(images)}

      ${renderApprovalBlock(d.approval)}
    </div>
  `;
}

// ---- KPI - Sự cố (trang gộp) ----
async function loadKPISuCoPanel() {
  const canSeeSuCo = hasRole("SuCo") || hasRole("Moderator");
  document.getElementById("kpi-block").hidden = !hasRole("KPI");
  document.getElementById("suco-block").hidden = !canSeeSuCo;
  const tasks = [];
  if (hasRole("KPI")) tasks.push(loadKPI());
  if (canSeeSuCo) tasks.push(loadSuCo());
  await Promise.all(tasks);
}

async function loadKPI() {
  const existing = (await api("/api/kpi")) || [];
  const byChiTieu = Object.fromEntries(existing.map(k => [k.chiTieu, k]));
  const tbody = document.getElementById("kpi-rows");
  tbody.innerHTML = KPI_CATEGORIES.map((c, i) => {
    const e = byChiTieu[c.chiTieu] || {};
    // TT/Nhóm chỉ in ở dòng đầu nhóm, dùng rowspan để gộp dọc — đúng bố cục
    // bảng KPI trong file Word (mỗi nhóm chỉ có 1 ô TT/Nhóm chung).
    const groupCells = c.groupSize
      ? `<td rowspan="${c.groupSize}">${c.tt}</td><td rowspan="${c.groupSize}" class="left">${esc(c.nhom)}</td>`
      : "";
    // Thực hiện của các chỉ tiêu "auto" là server tự đếm từ mục Sự cố — chỉ
    // hiển thị, không cho gõ tay (xem app.js loadKPISuCoPanel/report.go).
    const thCell = c.auto
      ? `<td class="k-th-display">${e.thucHien ?? 0}</td>`
      : `<td><input class="k-th" type="number" step="any" placeholder="0" value="${e.thucHien ?? 0}"></td>`;
    return `<tr data-idx="${i}" data-orig-th="${e.thucHien ?? 0}" data-orig-lk="${e.luyKeThang ?? 0}">
      ${groupCells}
      <td class="left">${esc(c.chiTieu)}<input type="hidden" class="k-chitieu" value="${esc(c.chiTieu)}"></td>
      <td>${esc(c.dvt)}<input type="hidden" class="k-dvt" value="${esc(c.dvt)}"></td>
      <td><input class="k-kh" type="number" step="any" placeholder="0" value="${e.keHoach ?? 0}"></td>
      ${thCell}
      <td class="k-lk-display">${e.luyKeThang ?? 0}</td>
      <td class="k-dg-display">${esc(e.danhGia || "")}</td>
    </tr>`;
  }).join("");

  // Gõ Kế hoạch/Thực hiện là tính Đánh giá (và với 2 chỉ tiêu nhập tay,
  // Lũy kế tháng) ngay trên trình duyệt, khỏi đợi bấm Lưu KPI mới thấy —
  // khớp đúng công thức server dùng (report.KPIDanhGia/ApplyKPIAutoCounts).
  document.querySelectorAll("#kpi-rows tr").forEach(tr => {
    tr.querySelectorAll(".k-kh, .k-th").forEach(inp => {
      inp.addEventListener("input", () => liveUpdateDanhGia(tr));
    });
  });
}
function liveUpdateDanhGia(tr) {
  const khInput = tr.querySelector(".k-kh");
  const thInput = tr.querySelector(".k-th");
  const thDisplay = tr.querySelector(".k-th-display");
  const kh = khInput ? khInput.value : 0;
  const th = thInput ? thInput.value : (thDisplay ? thDisplay.textContent : 0);
  const dg = tr.querySelector(".k-dg-display");
  if (dg) dg.textContent = (Number(th) || 0) <= (Number(kh) || 0) ? "Đạt" : "Không đạt";

  // Lũy kế tháng của 2 chỉ tiêu nhập tay (Rework, Dừng ngoài kế hoạch) là
  // tổng Thực hiện đã lưu mỗi ngày trong tháng — số hiển thị lúc tải trang
  // đã gồm sẵn giá trị CŨ của hôm nay, nên xem trước = số đó trừ giá trị cũ
  // cộng số mới đang gõ.
  if (thInput) {
    const origTh = Number(tr.dataset.origTh || 0);
    const origLk = Number(tr.dataset.origLk || 0);
    const lk = tr.querySelector(".k-lk-display");
    if (lk) lk.textContent = origLk - origTh + (Number(thInput.value) || 0);
  }
}
async function saveKPI() {
  const rows = [...document.querySelectorAll("#kpi-rows tr")].map(tr => {
    const thInput = tr.querySelector(".k-th");
    return {
      chiTieu: tr.querySelector(".k-chitieu").value,
      nhom: KPI_CATEGORIES[Number(tr.dataset.idx)].nhom,
      dvt: tr.querySelector(".k-dvt").value,
      keHoach: numOrNull(tr.querySelector(".k-kh").value),
      thucHien: thInput ? numOrNull(thInput.value) : null,
    };
  });
  await api("/api/kpi", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(rows) });
  alert("Đã lưu KPI hôm nay.");
  loadKPI();
}
function numOrNull(v) { return v === "" ? null : Number(v); }

// Cập nhật số đếm tự động + Đánh giá sau khi Sự cố thay đổi, mà KHÔNG vẽ
// lại toàn bộ bảng KPI (tránh xoá mất Kế hoạch người dùng đang gõ dở mà
// chưa bấm Lưu KPI).
async function refreshKPIAutoCells() {
  const existing = (await api("/api/kpi")) || [];
  const byChiTieu = Object.fromEntries(existing.map(k => [k.chiTieu, k]));
  document.querySelectorAll("#kpi-rows tr").forEach(tr => {
    const chiTieu = tr.querySelector(".k-chitieu").value;
    const e = byChiTieu[chiTieu] || {};
    const th = tr.querySelector(".k-th-display");
    if (th) th.textContent = e.thucHien ?? 0;
    const lk = tr.querySelector(".k-lk-display");
    if (lk) lk.textContent = e.luyKeThang ?? 0;
    const dg = tr.querySelector(".k-dg-display");
    if (dg) dg.textContent = e.danhGia || "";
  });
}

// ---- Admin ----
document.getElementById("form-user").addEventListener("submit", async e => {
  e.preventDefault();
  const body = fd2obj(e.target);
  await api("/api/users", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  e.target.reset();
  alert("Đã tạo tài khoản.");
  loadUsersList();
});

async function loadAdminPanel() {
  await Promise.all([loadUsersList(), loadReportSchedule(), loadCBMExcelPath(), loadRSMPdfFolder(), loadSendAllDays(), loadScheduleOverrides(), loadKPIBaseline(), loadApprovalSchedule()]);
}

// ---- Danh sách user / sửa thông tin (kể cả role) ----
let USERS_CACHE = [];
async function loadUsersList() {
  USERS_CACHE = (await api("/api/users")) || [];
  renderUsersList();
}
// Lọc client-side theo mã NV/họ tên/quyền — danh sách user hiếm khi lớn tới
// mức cần gọi lại API mỗi lần gõ, nên lọc thẳng trên USERS_CACHE đã tải.
function renderUsersList() {
  const q = (document.getElementById("userSearch").value || "").trim().toLowerCase();
  const rows = !q ? USERS_CACHE : USERS_CACHE.filter(u =>
    (u.employeeCode || "").toLowerCase().includes(q) ||
    (u.fullName || "").toLowerCase().includes(q) ||
    (u.roles || "").toLowerCase().includes(q));
  const t = document.getElementById("table-users");
  t.innerHTML = "<tr><th>Mã NV</th><th>Họ tên</th><th>Quyền</th><th></th></tr>" +
    (rows.length === 0
      ? '<tr><td colspan="4" style="text-align:center;color:#888">Không tìm thấy user nào</td></tr>'
      : rows.map(u => `<tr>
          <td>${esc(u.employeeCode)}</td><td>${esc(u.fullName)}</td><td>${esc(u.roles)}</td>
          <td><button class="btn" style="margin-top:0" onclick="openEditUser('${esc(u.employeeCode)}')">Sửa</button></td>
        </tr>`).join(""));
}
function openEditUser(employeeCode) {
  const u = USERS_CACHE.find(x => x.employeeCode === employeeCode);
  if (!u) return;
  document.getElementById("editUserModal").dataset.employeeCode = employeeCode;
  document.getElementById("editUserCode").textContent = u.employeeCode;
  document.getElementById("editUserRoles").value = u.roles;
  document.getElementById("editUserErr").textContent = "";
  document.getElementById("editUserModal").hidden = false;
}
function closeEditUser() {
  document.getElementById("editUserModal").hidden = true;
}
async function submitEditUser() {
  const employeeCode = document.getElementById("editUserModal").dataset.employeeCode;
  const roles = document.getElementById("editUserRoles").value.trim();
  const errBox = document.getElementById("editUserErr");
  errBox.textContent = "";
  try {
    await api("/api/users/" + encodeURIComponent(employeeCode), {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ roles }),
    });
  } catch (e) {
    errBox.textContent = "Lỗi: " + e.message;
    return;
  }
  closeEditUser();
  loadUsersList();
}

// ---- Gửi báo cáo tất cả các ngày (kể cả T7/CN) hay chỉ ngày thường ----
async function loadSendAllDays() {
  const s = await api("/api/send-all-days");
  document.getElementById("sendAllDays").checked = s.sendAllDays;
  document.getElementById("scheduleExceptionsBlock").hidden = s.sendAllDays;
}
async function saveSendAllDays() {
  const sendAllDays = document.getElementById("sendAllDays").checked;
  document.getElementById("scheduleExceptionsBlock").hidden = sendAllDays;
  await api("/api/send-all-days", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ sendAllDays }) });
}

// ---- Đường dẫn file Excel CBM ----
async function loadCBMExcelPath() {
  const s = await api("/api/cbm-excel-path");
  document.getElementById("cbmExcelPath").value = s.cbmExcelPath || "";
}
async function saveCBMExcelPath() {
  const cbmExcelPath = document.getElementById("cbmExcelPath").value;
  await api("/api/cbm-excel-path", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ cbmExcelPath }) });
  alert("Đã lưu.");
}

// ---- Thư mục file RSM Night Shift Report (PDF) ----
async function loadRSMPdfFolder() {
  const s = await api("/api/rsm-pdf-folder");
  document.getElementById("rsmPdfFolder").value = s.rsmPdfFolder || "";
}
async function saveRSMPdfFolder() {
  const rsmPdfFolder = document.getElementById("rsmPdfFolder").value;
  await api("/api/rsm-pdf-folder", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ rsmPdfFolder }) });
  alert("Đã lưu.");
}

// ---- Giờ tự động gửi báo cáo hàng ngày ----
async function loadReportSchedule() {
  const s = await api("/api/report-schedule");
  const hh = String(s.reportHour).padStart(2, "0");
  const mm = String(s.reportMinute).padStart(2, "0");
  document.getElementById("reportScheduleTime").value = `${hh}:${mm}`;
}
async function saveReportSchedule() {
  const val = document.getElementById("reportScheduleTime").value;
  if (!val) { alert("Chọn giờ trước đã."); return; }
  const [hh, mm] = val.split(":").map(Number);
  await api("/api/report-schedule", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ reportHour: hh, reportMinute: mm }) });
  alert("Đã lưu. Có tác dụng trong vòng 1 phút.");
}

// ---- Hạn Kiểm tra / Phê duyệt báo cáo ----
async function loadApprovalSchedule() {
  const s = await api("/api/approval-schedule");
  document.getElementById("kiemTraDeadline").value = `${String(s.kiemTraHour).padStart(2, "0")}:${String(s.kiemTraMinute).padStart(2, "0")}`;
  document.getElementById("pheDuyetDeadline").value = `${String(s.pheDuyetHour).padStart(2, "0")}:${String(s.pheDuyetMinute).padStart(2, "0")}`;
}
async function saveApprovalSchedule() {
  const kt = document.getElementById("kiemTraDeadline").value;
  const pd = document.getElementById("pheDuyetDeadline").value;
  if (!kt || !pd) { alert("Chọn đủ 2 giờ trước đã."); return; }
  const [kh, km] = kt.split(":").map(Number);
  const [ph, pm] = pd.split(":").map(Number);
  await api("/api/approval-schedule", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ kiemTraHour: kh, kiemTraMinute: km, pheDuyetHour: ph, pheDuyetMinute: pm })
  });
  alert("Đã lưu.");
}

// ---- Ngày ngoại lệ chạy báo cáo vào Thứ 7/CN ----
async function loadScheduleOverrides() {
  const rows = (await api("/api/schedule-overrides")) || [];
  const t = document.getElementById("table-overrides");
  t.innerHTML = "<tr><th>Ngày</th><th>Người thêm</th><th></th></tr>" +
    rows.map(r => `<tr>
      <td>${esc(r.overrideDate)}</td><td>${esc(r.createdBy)}</td>
      <td><button class="btn danger" onclick="deleteScheduleOverride(${r.id})">Xóa</button></td>
    </tr>`).join("");
}
async function addScheduleOverride() {
  const date = document.getElementById("overrideDate").value;
  if (!date) { alert("Chọn ngày trước đã."); return; }
  await api("/api/schedule-overrides", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ date }) });
  document.getElementById("overrideDate").value = "";
  loadScheduleOverrides();
}
async function deleteScheduleOverride(id) {
  if (!confirm("Xóa ngoại lệ này?")) return;
  await api(`/api/schedule-overrides/${id}`, { method: "DELETE" });
  loadScheduleOverrides();
}

// ---- Số lũy kế tháng ban đầu (KPI) ----
async function loadKPIBaseline() {
  const rows = (await api("/api/kpi-baseline")) || [];
  const t = document.getElementById("table-kpi-baseline");
  t.innerHTML = `<tr><th>Chỉ tiêu</th><th>Tháng</th><th>Số lũy kế có sẵn</th><th></th></tr>` +
    rows.map(r => `<tr data-chitieu="${esc(r.chiTieu)}">
      <td>${esc(r.chiTieu)}</td><td>${esc(r.yearMonth)}</td>
      <td><input class="kb-value" type="number" step="any" value="${r.baselineValue}"></td>
      <td><button class="btn" style="margin-top:0" onclick="saveKPIBaseline(this)">Lưu</button></td>
    </tr>`).join("");
}
async function saveKPIBaseline(btn) {
  const tr = btn.closest("tr");
  const chiTieu = tr.dataset.chitieu;
  const baselineValue = Number(tr.querySelector(".kb-value").value) || 0;
  await api("/api/kpi-baseline", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ chiTieu, baselineValue }) });
  alert("Đã lưu.");
}
function reportTestDateQuery() {
  const date = document.getElementById("thongke-date").value;
  return date ? "?date=" + encodeURIComponent(date) : "";
}
async function runReportNow() {
  document.getElementById("runResult").textContent = "Đang chạy...";
  try {
    const path = await api("/api/report/run-now" + reportTestDateQuery(), { method: "POST" });
    document.getElementById("runResult").textContent = "Xong: " + path;
    loadThongKe();
  } catch (e) {
    document.getElementById("runResult").textContent = "Lỗi: " + e.message;
  }
}
async function sendTestEmail() {
  document.getElementById("runResult").textContent = "Đang sinh báo cáo và gửi email...";
  try {
    const result = await api("/api/report/send-test-email" + reportTestDateQuery(), { method: "POST" });
    document.getElementById("runResult").textContent = result;
    loadThongKe();
  } catch (e) {
    document.getElementById("runResult").textContent = "Lỗi: " + e.message;
  }
}

async function delRow(kind, id) {
  if (!confirm("Xóa dòng này?")) return;
  try {
    await api(`/api/${kind}/${id}`, { method: "DELETE" });
  } catch (e) {
    alert("Không xóa được: " + e.message);
    return;
  }
  if (kind === "suco") { loadSuCo(); refreshKPIAutoCells(); }
  if (kind === "giamsat") loadGiamSat();
  if (kind === "congviec") loadCongViec();
  if (kind === "reliability") loadRSMReview();
}

function esc(s) {
  if (s == null) return "";
  return String(s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

if (TOKEN) showApp(); else document.getElementById("loginScreen").style.display = "flex";

// Lưới an toàn chung: mọi lỗi API không được bắt riêng (VD: submit form bị
// chặn do không phải chính chủ tạo dòng) vẫn hiện được thông báo cho người
// dùng thay vì im lặng thất bại.
window.addEventListener("unhandledrejection", e => {
  alert("Lỗi: " + (e.reason && e.reason.message ? e.reason.message : e.reason));
});

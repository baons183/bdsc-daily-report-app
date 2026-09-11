// Khớp đúng bố cục bảng KPI trong file Word gốc: TT/Nhóm gộp dọc theo nhóm,
// "groupSize" chỉ đặt ở chỉ tiêu đầu nhóm (số dòng cần rowspan).
const KPI_CATEGORIES = [
  { tt: 1, nhom: "Sự cố", chiTieu: "Sự cố", dvt: "Vụ", groupSize: 3 },
  { tt: 1, nhom: "Sự cố", chiTieu: "Near Miss", dvt: "Vụ" },
  { tt: 1, nhom: "Sự cố", chiTieu: "Technical event", dvt: "Vụ" },
  { tt: 2, nhom: "Chất lượng", chiTieu: "Rework", dvt: "WO", groupSize: 1 },
  { tt: 3, nhom: "Hỏng thiết bị", chiTieu: "Dừng ngoài kế hoạch", dvt: "Vụ", groupSize: 1 },
];

let TOKEN = localStorage.getItem("bdsc_token") || "";
let ROLES = localStorage.getItem("bdsc_roles") || "";
let FULLNAME = localStorage.getItem("bdsc_fullname") || "";
let EMPCODE = localStorage.getItem("bdsc_empcode") || "";

// Trả về true nếu người dùng hiện tại được sửa/xóa dòng do `createdBy` tạo
// (chính chủ hoặc Admin) — khớp đúng luật phía server (checkOwnership).
function canModify(createdBy) {
  return hasRole("Admin") || createdBy === EMPCODE;
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

function hasRole(r) {
  return ROLES.split(",").map(s => s.trim()).includes(r) || ROLES.includes("Admin");
}

async function api(path, opts = {}) {
  opts.headers = opts.headers || {};
  if (TOKEN) opts.headers["Authorization"] = "Bearer " + TOKEN;
  const res = await fetch(path, opts);
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

function openChangePassword() {
  document.getElementById("cpOld").value = "";
  document.getElementById("cpNew").value = "";
  document.getElementById("cpNew2").value = "";
  document.getElementById("cpErr").textContent = "";
  document.getElementById("changePwModal").hidden = false;
}
function closeChangePassword() {
  document.getElementById("changePwModal").hidden = true;
}
async function submitChangePassword() {
  const oldPassword = document.getElementById("cpOld").value;
  const newPassword = document.getElementById("cpNew").value;
  const newPassword2 = document.getElementById("cpNew2").value;
  const errBox = document.getElementById("cpErr");
  errBox.textContent = "";
  if (!oldPassword || !newPassword) {
    errBox.textContent = "Vui lòng nhập đủ thông tin.";
    return;
  }
  if (newPassword !== newPassword2) {
    errBox.textContent = "Mật khẩu mới nhập lại không khớp.";
    return;
  }
  try {
    await api("/api/change-password", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ oldPassword, newPassword }),
    });
    closeChangePassword();
    alert("Đã đổi mật khẩu. Vui lòng đăng nhập lại.");
    logout();
  } catch (e) {
    errBox.textContent = "Lỗi: " + e.message;
  }
}

function showApp() {
  document.getElementById("loginScreen").style.display = "none";
  document.getElementById("app").style.display = "block";
  document.getElementById("whoami").textContent = FULLNAME || "";
  buildTabs();
}

const TAB_DEFS = [
  { id: "kpi", label: "KPI", role: "KPI" },
  { id: "suco", label: "Sự cố", role: "SuCo" },
  { id: "giamsat", label: "Giám sát/Chẩn đoán", role: "GiamSat" },
  { id: "congviec", label: "Công việc", role: "CongViec" },
  { id: "cbm", label: "Giám sát trực tuyến", role: null },
  { id: "thongke", label: "Thống kê", role: null },
  { id: "admin", label: "Quản trị", role: "Admin" },
];

function buildTabs() {
  const nav = document.getElementById("tabs");
  nav.innerHTML = "";
  let first = null;
  TAB_DEFS.forEach(t => {
    if (t.role && !hasRole(t.role)) return;
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
  const loaders = { suco: loadSuCo, giamsat: loadGiamSat, congviec: loadCongViec, cbm: loadCBM, thongke: initThongKe, kpi: loadKPI, admin: loadScheduleOverrides };
  if (loaders[id]) loaders[id]().catch(e => alert("Lỗi tải dữ liệu: " + e.message));
}

function fd2obj(form) {
  const obj = {};
  new FormData(form).forEach((v, k) => { if (k !== "photos") obj[k] = v; });
  return obj;
}

// Cache dòng dữ liệu mới tải để nút "Sửa" có sẵn dữ liệu điền lại vào form
// mà không cần thêm API lấy 1 dòng riêng.
const ROWS_CACHE = { suco: {}, giamsat: {}, congviec: {} };

// Ô Sửa/Xóa chỉ hiện nếu chính chủ tạo dòng đó hoặc là Admin — khớp luật
// checkOwnership phía server (server vẫn chặn lại nếu cố tình gọi API thẳng).
function actionButtons(kind, r) {
  if (!canModify(r.createdBy)) return "";
  return `<button class="btn" style="margin-top:0" onclick="edit${kind}(${r.id})">Sửa</button> ` +
    `<button class="btn danger" onclick="delRow('${kind.toLowerCase()}', ${r.id})">Xóa</button>`;
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
  const rows = (await api("/api/suco")) || [];
  ROWS_CACHE.suco = Object.fromEntries(rows.map(r => [r.id, r]));
  const t = document.getElementById("table-suco");
  t.innerHTML = `<tr><th>TT</th><th>Thiết bị</th><th>DM</th><th>Tình trạng</th><th>TG dừng PX (h)</th><th>TG dừng TB (h)</th>
      <th>Phương án xử lý</th><th>Đơn vị thực hiện</th><th>Thời hạn</th><th>Đề xuất</th><th>Lưu ý</th><th></th></tr>` +
    rows.map(r => `<tr>
      <td>${r.tt}</td><td>${esc(r.thietBi)}</td><td>${esc(r.dm)}</td><td>${esc(r.tinhTrang)}</td>
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
    thietBi: r.thietBi, dm: r.dm, tinhTrang: r.tinhTrang,
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
});

// ---- Giám sát ----
async function loadGiamSat() {
  const rows = (await api("/api/giamsat")) || [];
  ROWS_CACHE.giamsat = Object.fromEntries(rows.map(r => [r.id, r]));
  const t = document.getElementById("table-giamsat");
  t.innerHTML = `<tr><th>TT</th><th>Thiết bị</th><th>DM</th><th>Tình trạng</th><th>Rủi ro</th><th>Phương án xử lý</th>
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
  const rows = (await api("/api/congviec")) || [];
  ROWS_CACHE.congviec = Object.fromEntries(rows.map(r => [r.id, r]));
  const t = document.getElementById("table-congviec");
  t.innerHTML = `<tr><th>TT</th><th>Thiết bị</th><th>DM</th><th>Nội dung</th><th>Kế hoạch</th><th>Tiến độ (%)</th>
      <th>Hôm nay</th><th>Ngày mai</th><th>Đề xuất</th><th>Lưu ý</th><th>Ảnh</th><th></th></tr>` +
    rows.map(r => `<tr>
      <td>${r.tt}</td><td>${esc(r.thietBi)}</td><td>${esc(r.dm)}</td><td>${esc(r.noiDung)}</td><td>${esc(r.keHoach)}</td><td>${r.tienDo ?? ""}</td>
      <td>${esc(r.homNay)}</td><td>${esc(r.ngayMai)}</td><td>${esc(r.deXuat)}</td><td>${esc(r.luuY)}</td>
      <td>${(r.attachments||[]).map(a => `<img class="thumb" src="/uploads/${a.filePath}">`).join("")}</td>
      <td>${actionButtons("CongViec", r)}</td>
    </tr>`).join("");
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

["form-suco", "form-giamsat", "form-congviec"].forEach(id => {
  attachAssetAutocomplete(document.getElementById(id));
});

function kpiDanhGiaSelect(value) {
  const options = ["", "Đạt", "Không đạt"];
  return `<select class="k-dg">` +
    options.map(o => `<option value="${esc(o)}"${o === (value || "") ? " selected" : ""}>${o || "-"}</option>`).join("") +
    `</select>`;
}

// ---- Giám sát trực tuyến (CBM) — chỉ xem, dữ liệu tự động từ hệ thống ngoài ----
async function loadCBM() {
  const rows = (await api("/api/cbm")) || [];
  const t = document.getElementById("table-cbm");
  t.innerHTML = "<tr><th>TT</th><th>Thời gian</th><th>Tagname</th><th>Thiết bị</th><th>Mô tả</th><th>Giá trị cảnh báo</th><th>Mức cảnh báo</th></tr>" +
    (rows.length === 0
      ? '<tr><td colspan="7" style="text-align:center;color:#888">Không có dữ liệu trong khung giờ báo cáo hiện tại</td></tr>'
      : rows.map((r, i) => `<tr>
          <td>${i + 1}</td><td>${fmtDateTime(r.dataTime)}</td><td>${esc(r.tagname)}</td><td>${esc(r.equipment)}</td>
          <td>${esc(r.description)}</td><td>${esc(r.alarmValue)}</td><td>${esc(r.alarmLevel)}</td>
        </tr>`).join(""));
}
function fmtDateTime(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  const pad = n => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())} ${pad(d.getDate())}/${pad(d.getMonth() + 1)}`;
}

// ---- Thống kê (xem lại báo cáo theo ngày, giao diện giống PDF) ----
async function initThongKe() {
  const dateInput = document.getElementById("thongke-date");
  if (!dateInput.value) dateInput.value = new Date().toISOString().slice(0, 10);
  document.getElementById("thongke-admin-tools").hidden = !hasRole("Admin");
  return loadThongKe();
}

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
  content.innerHTML = renderReportView(data);
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

function renderReportView(d) {
  const kpiRows = KPI_CATEGORIES.map(c => {
    const e = (d.kpi || []).find(k => k.chiTieu === c.chiTieu) || {};
    return [c.tt, c.nhom, c.chiTieu, c.dvt, e.keHoach ?? 0, e.thucHien ?? 0, e.luyKeThang ?? 0, e.danhGia || "-"].map(esc);
  });
  const sucoRows = (d.suco || []).map(s => [s.tt, s.thietBi, s.dm, s.tinhTrang, s.thoiGianDungPX ?? "", s.thoiGianDungTB ?? "", s.phuongAn, s.donVi, s.thoiHan ? s.thoiHan.substring(0, 10) : ""].map(esc));
  const cbmRows = (d.cbm || []).map((c, i) => [i + 1, fmtDateTime(c.dataTime), c.tagname, c.equipment, c.description, c.alarmValue, c.alarmLevel].map(esc));
  const giamsatRows = (d.giamSat || []).map(s => [s.tt, s.thietBi, s.dm, s.tinhTrang, s.ruiRo, s.phuongAn, s.donVi, s.thoiHan ? s.thoiHan.substring(0, 10) : ""].map(esc));
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

      <div class="rv-heading">2. SỰ CỐ, VẤN ĐỀ KỸ THUẬT ẢNH HƯỞNG VHSX</div>
      ${rvTable(["TT", "Thiết bị", "DM", "Tình trạng", "PX (h)", "TB (h)", "Phương án", "Đơn vị", "Thời hạn"], sucoRows, "Không có sự cố/vấn đề kỹ thuật ảnh hưởng vận hành.")}

      <div class="rv-heading">3. KẾT QUẢ GIÁM SÁT, CHẨN ĐOÁN THIẾT BỊ</div>
      <div class="rv-sub">3.1. Giám sát trực tuyến (online monitoring)</div>
      ${rvTable(["TT", "Thời gian", "Tagname", "Thiết bị", "Mô tả", "Giá trị cảnh báo", "Mức cảnh báo"], cbmRows)}
      <div class="rv-sub">3.2. Chẩn đoán bằng tool chuyên dụng</div>
      ${rvTable(["TT", "Thiết bị", "DM", "Tình trạng", "Rủi ro", "Phương án", "Đơn vị", "Thời hạn"], giamsatRows)}

      <div class="rv-heading">4. KẾT QUẢ THỰC HIỆN CÔNG VIỆC BDSC QUAN TRỌNG</div>
      ${rvTable(["TT", "Thiết bị", "DM", "Nội dung", "Kế hoạch", "Tiến độ (%)", "Hôm nay", "Ngày mai"], congviecRows)}

      <div class="rv-heading">5. VẤN ĐỀ CẦN ĐỀ XUẤT</div>
      ${rvTable(["TT", "Thiết bị", "Đề xuất"], dexuatRows, "Không có đề xuất.")}

      <div class="rv-heading">6. LƯU Ý QUAN TRỌNG</div>
      ${rvTable(["TT", "Thiết bị", "Lưu ý"], luuyRows)}

      <div class="rv-heading">7. MỘT SỐ HÌNH ẢNH THỰC HIỆN CÔNG VIỆC TẠI HIỆN TRƯỜNG</div>
      ${renderImagesTable(images)}
    </div>
  `;
}

// ---- KPI ----
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
    return `<tr data-idx="${i}">
      ${groupCells}
      <td class="left">${esc(c.chiTieu)}<input type="hidden" class="k-chitieu" value="${esc(c.chiTieu)}"></td>
      <td>${esc(c.dvt)}<input type="hidden" class="k-dvt" value="${esc(c.dvt)}"></td>
      <td><input class="k-kh" type="number" step="any" placeholder="0" value="${e.keHoach ?? ""}"></td>
      <td><input class="k-th" type="number" step="any" placeholder="0" value="${e.thucHien ?? ""}"></td>
      <td><input class="k-lk" type="number" step="any" placeholder="0" value="${e.luyKeThang ?? ""}"></td>
      <td>${kpiDanhGiaSelect(e.danhGia)}</td>
    </tr>`;
  }).join("");
}
async function saveKPI() {
  const rows = [...document.querySelectorAll("#kpi-rows tr")].map(tr => ({
    chiTieu: tr.querySelector(".k-chitieu").value,
    nhom: KPI_CATEGORIES[Number(tr.dataset.idx)].nhom,
    dvt: tr.querySelector(".k-dvt").value,
    keHoach: numOrNull(tr.querySelector(".k-kh").value),
    thucHien: numOrNull(tr.querySelector(".k-th").value),
    luyKeThang: numOrNull(tr.querySelector(".k-lk").value),
    danhGia: tr.querySelector(".k-dg").value,
  }));
  await api("/api/kpi", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(rows) });
  alert("Đã lưu KPI hôm nay.");
}
function numOrNull(v) { return v === "" ? null : Number(v); }

// ---- Admin ----
document.getElementById("form-user").addEventListener("submit", async e => {
  e.preventDefault();
  const body = fd2obj(e.target);
  await api("/api/users", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  e.target.reset();
  alert("Đã tạo tài khoản.");
});

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
  if (kind === "suco") loadSuCo();
  if (kind === "giamsat") loadGiamSat();
  if (kind === "congviec") loadCongViec();
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

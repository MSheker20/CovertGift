'use strict';

const $ = (selector) => document.querySelector(selector);
const storageKey = 'covertgift-test-token-v1';
const state = { token: sessionStorage.getItem(storageKey) || '', user: null, groups: [], detail: null, ideas: [], logs: [] };
let busy = false;
const money = (value) => new Intl.NumberFormat('tr-TR', { style: 'currency', currency: 'TRY' }).format(Number(value || 0));
const escapeHTML = (value) => String(value ?? '').replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
const groupPath = () => `/api/v1/groups/${encodeURIComponent(state.detail.group.id)}`;
const formData = (form) => Object.fromEntries(new FormData(form));
const notice = (message, error = false) => {
  $('#notice').textContent = message;
  $('#notice').classList.toggle('error', error);
};
const safeURL = (value) => {
  try {
    const url = new URL(value);
    return ['http:', 'https:'].includes(url.protocol) ? escapeHTML(url.href) : '';
  } catch { return ''; }
};

function logResponse(method, path, status, body) {
  // Only response bodies are logged. Never log submitted passwords or bearer headers.
  const redacted = JSON.stringify(body, (key, value) => /token|password|secret/i.test(key) ? '[gizlendi]' : value, 2);
  state.logs.unshift(`${new Date().toLocaleTimeString('tr-TR')} ${method} ${path} → ${status}\n${redacted}`);
  state.logs = state.logs.slice(0, 15);
  $('#api-log').textContent = state.logs.join('\n\n');
}

async function api(path, { method = 'GET', body, publicRequest = false } = {}) {
  let response;
  try {
    response = await fetch(path, {
      method, headers: {
        ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
        ...(!publicRequest && state.token ? { Authorization: `Bearer ${state.token}` } : {}),
      },
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
      signal: AbortSignal.timeout(185_000),
    });
  } catch (error) {
    logResponse(method, path, 'ağ hatası', { error: error.message });
    throw new Error('Sunucuya erişilemiyor veya istek zaman aşımına uğradı. Bağlantıyı kontrol edip listeleri yenileyin.');
  }
  const text = await response.text();
  let payload;
  try { payload = JSON.parse(text); } catch { payload = { error: text.slice(0, 1000) || 'Boş sunucu yanıtı' }; }
  logResponse(method, path, response.status, payload);
  if (!response.ok || payload.success === false) {
    if (response.status === 401 && !publicRequest) clearSession();
    const error = new Error(`HTTP ${response.status}: ${payload.error || response.statusText}`);
    error.status = response.status;
    throw error;
  }
  return payload.data;
}

// Prevent duplicate writes and account/group switches while an operation is running.
async function run(task) {
  if (busy) return;
  busy = true;
  const buttons = [...document.querySelectorAll('button')];
  buttons.forEach((button) => { button.disabled = true; });
  $('#working').hidden = false;
  try { await task(); } catch (error) { notice(error.message, true); }
  finally {
    busy = false;
    document.querySelectorAll('button').forEach((button) => { button.disabled = false; });
    $('#working').hidden = true;
  }
}

function clearGroup() {
  state.detail = null;
  state.ideas = [];
  $('#group-panel').hidden = true;
  $('#group-title').textContent = '';
  $('#members').replaceChildren();
  $('#ideas').replaceChildren();
  $('#idea-form').reset();
  $('#preview-form').reset();
  $('#member-form').reset();
  $('#interests-form').reset();
  $('#budget-form').reset();
  $('#budget-form').hidden = true;
  $('#interests-list').replaceChildren();
  $('#preview-note').textContent = 'Önizleme veritabanına kayıt yapmaz. Alanları kontrol edip aşağıdan kaydet.';
}

function clearSession() {
  state.token = '';
  state.user = null;
  state.groups = [];
  sessionStorage.removeItem(storageKey);
  clearGroup();
  $('#profile').textContent = '';
  $('#user-id').value = '';
  $('#groups').replaceChildren();
  $('#workspace').hidden = true;
  $('#auth-panel').hidden = false;
}

function renderProfile() {
  $('#profile').textContent = `${state.user.name} · ${state.user.email}`;
  $('#user-id').value = state.user.id;
  $('#auth-panel').hidden = true;
  $('#workspace').hidden = false;
}

async function loadProfile() {
  state.user = await api('/api/v1/me');
  renderProfile();
}

async function login(email, password) {
  const result = await api('/api/v1/auth/login', { method: 'POST', body: { email, password }, publicRequest: true });
  state.token = result.token;
  sessionStorage.setItem(storageKey, state.token);
  state.user = result.user;
  renderProfile();
  $('#login-form').reset();
  $('#register-form').reset();
  await loadGroups();
  notice(`Giriş yapıldı: ${state.user.name}`);
}

function renderGroups() {
  $('#groups').innerHTML = state.groups.length ? state.groups.map((group) => `
    <button class="group-item" data-group="${escapeHTML(group.id)}" aria-current="${state.detail?.group.id === group.id}">
      ${escapeHTML(group.title)}<small>${escapeHTML(group.target_person_name)} · ${money(group.total_budget)}</small>
    </button>`).join('') : '<p>Henüz üye olduğun bir grup yok. Yeni grup oluşturabilirsin.</p>';
}

async function loadGroups() {
  state.groups = await api('/api/v1/groups') || [];
  if (state.detail && !state.groups.some((group) => group.id === state.detail.group.id)) clearGroup();
  renderGroups();
}

async function openGroup(id) {
  const previousID = state.detail?.group.id;
  // Clear the previous group before checking access to a different group.
  if (id !== previousID) clearGroup();
  let detail;
  let ideas;
  try {
    detail = await api(`/api/v1/groups/${encodeURIComponent(id)}`);
    ideas = await api(`/api/v1/groups/${encodeURIComponent(id)}/ideas`) || [];
  } catch (error) {
    clearGroup();
    renderGroups();
    throw error;
  }
  state.detail = detail;
  state.ideas = ideas;
  const group = detail.group;
  $('#group-panel').hidden = false;
  $('#group-title').textContent = group.title;
  $('#group-description').textContent = `${group.target_person_name} için${group.description ? ` · ${group.description}` : ''}`;
  $('#group-id').value = group.id;
  $('#budget-form').hidden = detail.my_role !== 'admin';
  $('#budget-form').elements.namedItem('total_budget').value = group.total_budget;
  renderInterests();
  $('#group-stats').textContent = `Toplam: ${money(group.total_budget)} · ${detail.member_count} kişi · Kişi başı: ${money(detail.per_person_budget)} · Rolün: ${detail.my_role}`;
  $('#members').innerHTML = (detail.members || []).map((member) => `<li>${escapeHTML(member.user?.name || member.user_id)} · ${escapeHTML(member.user?.email || '')} <span class="badge">${escapeHTML(member.role)}</span></li>`).join('') || '<li>Üye listesi yanıtta bulunmuyor.</li>';
  renderIdeas();
  renderGroups();
  if (busy) document.querySelectorAll('button').forEach((button) => { button.disabled = true; });
}

function renderInterests() {
  const admin = state.detail.my_role === 'admin';
  const interests = state.detail.interests || [];
  $('#interests-list').innerHTML = interests.map((interest) => `<li><span>${escapeHTML(interest)}</span>${admin ? `<button class="danger" data-remove-interest="${escapeHTML(interest)}" aria-label="${escapeHTML(interest)} ilgi alanını sil">Sil</button>` : ''}</li>`).join('');
  $('#interests-help').textContent = `${interests.length ? '' : 'Henüz ilgi alanı yok. '}${admin ? 'Tüm üyeler ekleyebilir. Admin olarak listedeki öğeleri silebilirsin.' : 'Yeni ilgi alanları ekleyebilirsin. Silme işlemini admin yapabilir.'} Yeni AI önerileri bu listeyi kullanır.`;
}

function renderIdeas() {
  const filter = $('#idea-filter').value;
  const ideas = state.ideas.filter((idea) => filter === 'ALL' || idea.status === filter);
  const admin = state.detail?.my_role === 'admin';
  $('#ideas').innerHTML = ideas.length ? ideas.map((idea) => {
    const image = safeURL(idea.image_url);
    const product = safeURL(idea.product_url);
    const query = encodeURIComponent(idea.search_query || idea.title);
    return `<article class="idea">
      ${image ? `<img src="${image}" alt="${escapeHTML(idea.title)}" loading="lazy" referrerpolicy="no-referrer">` : ''}
      <div class="actions"><span class="badge">${escapeHTML(idea.source_type)}</span><span class="badge">${escapeHTML(idea.status)}</span></div>
      <h3>${escapeHTML(idea.title)}</h3><div class="price">${money(idea.estimated_price)}</div>
      ${idea.is_over_budget ? '<p class="warning">Toplam bütçeyi aşıyor</p>' : ''}
      <p>${escapeHTML(idea.description)}</p>
      <small>Öneren: ${escapeHTML(idea.creator?.name || idea.created_by)}</small>
      <div class="actions">
        <button data-idea="${escapeHTML(idea.id)}" data-action="vote" data-vote="1" aria-pressed="${idea.my_vote === 1}" aria-label="${escapeHTML(idea.title)} için olumlu oy">👍 +1</button>
        <strong>Skor: ${escapeHTML(idea.score ?? 0)}</strong>
        <button data-idea="${escapeHTML(idea.id)}" data-action="vote" data-vote="-1" aria-pressed="${idea.my_vote === -1}" aria-label="${escapeHTML(idea.title)} için olumsuz oy">👎 −1</button>
      </div>
      <div class="links">
        ${product ? `<a href="${product}" target="_blank" rel="noopener noreferrer">Ürüne git ↗</a>` : ''}
        <a href="https://www.trendyol.com/sr?q=${query}" target="_blank" rel="noopener noreferrer">Trendyol ↗</a>
        <a href="https://www.amazon.com.tr/s?k=${query}" target="_blank" rel="noopener noreferrer">Amazon ↗</a>
      </div>
      <div class="actions">
        ${admin && idea.status !== 'REJECTED' ? `<button data-idea="${escapeHTML(idea.id)}" data-action="reject">Veto et</button>` : ''}
        ${admin || idea.created_by === state.user.id ? `<button class="danger" data-idea="${escapeHTML(idea.id)}" data-action="delete">Sil…</button>` : ''}
      </div>
    </article>`;
  }).join('') : '<p>Bu filtrede hediye yok. Manuel ekleyebilir veya Gemini’den öneri isteyebilirsin.</p>';
}

async function checkHealth() {
  try {
    const result = await api('/health', { publicRequest: true });
    $('#health').textContent = result?.db === 'up' ? 'API ve DB bağlı' : 'API yanıt verdi';
  } catch (error) {
    $('#health').textContent = 'API bağlantısı yok';
    throw error;
  }
}

function onSubmit(id, handler) {
  $(id).addEventListener('submit', (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const data = formData(form);
    run(() => handler(data, form));
  });
}

onSubmit('#login-form', ({ email, password }) => login(email, password));
onSubmit('#register-form', async (data) => {
  await api('/api/v1/auth/register', { method: 'POST', body: data, publicRequest: true });
  notice('Hesap oluşturuldu, giriş yapılıyor…');
  await login(data.email, data.password);
});
onSubmit('#create-group-form', async (data, form) => {
  data.total_budget = String(data.total_budget);
  if (!data.target_user_id) delete data.target_user_id;
  const group = await api('/api/v1/groups', { method: 'POST', body: data });
  form.reset();
  await loadGroups();
  await openGroup(group.id);
  $('#group-panel').scrollIntoView({ block: 'start' });
  notice('Grup oluşturuldu.');
});
onSubmit('#open-group-form', async ({ group_id }) => {
  await openGroup(group_id.trim());
  $('#group-panel').scrollIntoView({ block: 'start' });
  notice('Grup açıldı.');
});
onSubmit('#budget-form', async (data) => {
  const id = state.detail.group.id;
  await api(`${groupPath()}/budget`, { method: 'PATCH', body: { total_budget: data.total_budget } });
  await openGroup(id);
  await loadGroups();
  notice('Bütçe güncellendi. Kişi başı tutar ve hediye uyarıları yenilendi.');
});
onSubmit('#interests-form', async (data, form) => {
  const id = state.detail.group.id;
  await api(`${groupPath()}/interests`, { method: 'POST', body: { interest: data.interest.trim() } });
  form.reset();
  await openGroup(id);
  await loadGroups();
  notice('İlgi alanı eklendi.');
});
onSubmit('#member-form', async (data, form) => {
  const id = state.detail.group.id;
  await api(`${groupPath()}/members`, { method: 'POST', body: data });
  form.reset();
  await openGroup(id);
  notice('Üye eklendi, kişi başı bütçe yenilendi.');
});
onSubmit('#preview-form', async ({ url }) => {
  const preview = await api(`${groupPath()}/ideas/preview-link`, { method: 'POST', body: { url } });
  const fields = $('#idea-form').elements;
  // Replace the entire previous preview, including missing fields, to avoid mixing products.
  for (const key of ['title', 'description', 'search_query', 'image_url']) fields.namedItem(key).value = preview[key] || '';
  fields.namedItem('estimated_price').value = preview.price || '0';
  fields.namedItem('product_url').value = preview.final_url || url;
  $('#preview-note').textContent = preview.partial ? `Kısmi önizleme: ${preview.warning || 'Eksik alanları elle doldurabilirsin.'}` : 'Önizleme hazır. Alanları kontrol edip hediyeyi kaydet.';
  notice(preview.partial ? 'Önizleme kısmi; manuel devam edebilirsin.' : 'Form linkten dolduruldu.');
});
onSubmit('#idea-form', async (data, form) => {
  const id = state.detail.group.id;
  data.search_query = data.search_query || data.title;
  data.estimated_price = String(data.estimated_price);
  await api(`${groupPath()}/ideas`, { method: 'POST', body: data });
  form.reset();
  $('#preview-form').reset();
  $('#preview-note').textContent = 'Hediye kaydedildi. Yeni bir link önizleyebilirsin.';
  await openGroup(id);
  notice('Hediye kaydedildi.');
});

const clicks = {
  'check-health': async () => { await checkHealth(); notice('Bağlantı kontrol edildi.'); },
  logout: () => { clearSession(); notice('Çıkış yapıldı.'); },
  'refresh-profile': async () => { await loadProfile(); notice('Profil yenilendi.'); },
  'refresh-groups': async () => { await loadGroups(); notice('Grup listesi yenilendi.'); },
  'refresh-group': async () => { await openGroup(state.detail.group.id); notice('Grup yenilendi.'); },
  'generate-ai': async () => {
    const id = state.detail.group.id;
    notice('Gemini önerileri hazırlanıyor…');
    const ideas = await api(`${groupPath()}/ideas/generate-ai`, { method: 'POST' });
    await openGroup(id);
    notice(`${ideas?.length ?? 0} AI önerisi eklendi.`);
  },
  'delete-group': async () => {
    if (!confirm(`“${state.detail.group.title}” grubu ve tüm fikirleri kalıcı silinsin mi?`)) return;
    await api(groupPath(), { method: 'DELETE' });
    clearGroup();
    await loadGroups();
    notice('Grup silindi.');
  },
  'delete-account': async () => {
    if (!confirm('Hesabın, oluşturduğun gruplar, önerilerin ve oyların kalıcı silinsin mi?')) return;
    await api('/api/v1/me', { method: 'DELETE' });
    clearSession();
    notice('Hesap silindi.');
  },
  'clear-log': () => { state.logs = []; $('#api-log').textContent = 'Günlük temizlendi.'; },
};
document.addEventListener('click', (event) => {
  const button = event.target.closest('button');
  if (!button || busy) return;
  if (clicks[button.id]) return void run(clicks[button.id]);
  if (button.dataset.removeInterest !== undefined) return void run(async () => {
    const id = state.detail.group.id;
    await api(`${groupPath()}/interests`, { method: 'DELETE', body: { interest: button.dataset.removeInterest } });
    await openGroup(id);
    await loadGroups();
    notice('İlgi alanı silindi.');
  });
  if (button.dataset.group) return void run(async () => {
    await openGroup(button.dataset.group);
    $('#group-panel').scrollIntoView({ block: 'start' });
    notice('Grup açıldı.');
  });
  if (button.dataset.idea) run(async () => {
    const action = button.dataset.action;
    const id = state.detail.group.id;
    const path = `/api/v1/ideas/${encodeURIComponent(button.dataset.idea)}`;
    if (action === 'delete' && !confirm('Bu hediye fikri kalıcı silinsin mi?')) return;
    if (action === 'vote') await api(`${path}/vote`, { method: 'POST', body: { vote_type: Number(button.dataset.vote) } });
    if (action === 'reject') await api(`${path}/reject`, { method: 'POST' });
    if (action === 'delete') await api(path, { method: 'DELETE' });
    await openGroup(id);
    notice(action === 'vote' ? 'Oy kaydedildi, skor ve durum yenilendi.' : action === 'reject' ? 'Hediye veto edildi.' : 'Hediye silindi.');
  });
});
$('#idea-filter').addEventListener('change', () => { if (!busy) renderIdeas(); });
document.addEventListener('error', (event) => { if (event.target.tagName === 'IMG') event.target.hidden = true; }, true);

run(async () => {
  try { await checkHealth(); } catch (error) { notice(error.message, true); }
  if (state.token) { await loadProfile(); await loadGroups(); notice('Oturum geri yüklendi.'); }
});

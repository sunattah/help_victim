/* ==========================================================================
   Victim Care — client application
   The server renders the first paint, this file adds the interactive flows:
   authentication, the SOS countdown, reporting, settings, picture uploads,
   theme switching and the moving picture area.
   ========================================================================== */

(function () {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const bootstrap = readBootstrap();

  const state = {
    signedIn: !!bootstrap.signedIn,
    user: bootstrap.user || {},
    csrf: bootstrap.csrf || '',
    pictures: bootstrap.pictures || [],
    contact: bootstrap.contact || {},
    reports: bootstrap.reports || [],
    tab: 'home',
    authMode: 'login',
    reportType: 'Accident',
    countdownTimer: null,
    countdownSeconds: 5
  };

  const views = ['home', 'report', 'aid', 'contacts', 'settings'];

  /* ------------------------------------------------------------ helpers */

  function readBootstrap() {
    const node = document.getElementById('bootstrap');
    if (!node) return {};
    try {
      return JSON.parse(node.textContent || '{}');
    } catch (e) {
      return {};
    }
  }

  async function api(path, options) {
    const opts = Object.assign({ method: 'GET', headers: {} }, options || {});
    opts.headers = Object.assign({}, opts.headers);
    if (state.csrf && opts.method !== 'GET') opts.headers['X-CSRF-Token'] = state.csrf;

    const response = await fetch(path, opts);
    let payload = {};
    try {
      payload = await response.json();
    } catch (e) {
      payload = {};
    }
    if (!response.ok) {
      const error = new Error(payload.error || 'Something went wrong. Please try again.');
      error.field = payload.field || '';
      error.status = response.status;
      throw error;
    }
    return payload;
  }

  function postJSON(path, body) {
    return api(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
  }

  function putJSON(path, body) {
    return api(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
  }

  function patchJSON(path, body) {
    return api(path, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
  }

  function setStatus(element, message, isError) {
    if (!element) return;
    element.textContent = message || '';
    element.classList.toggle('error', !!isError);
  }

  /* -------------------------------------------------------- app screens */

  function showScreen(screen) {
    $('welcome').hidden = screen !== 'welcome';
    $('auth-panel').hidden = screen !== 'auth';
    $('signed-in-view').hidden = screen !== 'app';
    $('top-nav').hidden = screen !== 'app';
    $('signed-in-actions').hidden = screen !== 'app';
  }

  function switchTab(next) {
    state.tab = next;
    views.forEach((name) => { $(name + '-view').hidden = name !== next; });
    document.querySelectorAll('[data-tab]').forEach((button) => {
      if (button.dataset.tab === next) button.setAttribute('aria-current', 'page');
      else button.removeAttribute('aria-current');
    });
    if (next === 'report') renderReports();
    if (next === 'contacts') {
      $('contact-name').value = state.contact.name || '';
      $('contact-phone').value = state.contact.phone || '';
      $('emergency-call').href = 'tel:' + (state.user.emergencyNumber || '112');
    }
    if (next === 'settings') fillSettings();
    if (next === 'home') updateSosLabel();
  }

  function updateSosLabel() {
    const seconds = state.user.countdownSeconds || 5;
    const text = $('.sos small');
    if (text) text.textContent = 'Press for help · ' + seconds + 's to cancel';
  }

  /* ---------------------------------------------------------- auth flow */

  function openAuth(mode) {
    state.authMode = mode;
    $('prototype-form').reset();
    $('auth-error').textContent = '';
    $('login-head').hidden = mode !== 'login';
    $('signup-head').hidden = mode !== 'signup';
    $('login-fields').hidden = mode !== 'login';
    $('signup-fields').hidden = mode !== 'signup';
    $('complete-login').hidden = mode !== 'login';
    $('complete-signup').hidden = mode !== 'signup';
    showScreen('auth');
    const first = mode === 'login' ? $('prototype-email') : $('signup-first-name');
    if (first) first.focus();
  }

  async function submitAuth(event) {
    event.preventDefault();
    const errorBox = $('auth-error');
    errorBox.textContent = '';

    try {
      if (state.authMode === 'signup') {
        const payload = {
          firstName: $('signup-first-name').value.trim(),
          secondName: $('signup-second-name').value.trim(),
          email: $('signup-email').value.trim(),
          state: $('signup-state').value.trim(),
          nation: $('signup-nation').value.trim(),
          password: $('signup-password').value
        };
        if (!payload.firstName || !payload.secondName || !payload.email || !payload.state || !payload.nation || !payload.password) {
          errorBox.textContent = 'Complete every field to continue.';
          return;
        }
        if ($('signup-password').value !== $('signup-confirm-password').value) {
          errorBox.textContent = 'Passwords do not match.';
          return;
        }
        const result = await postJSON('/api/v1/auth/signup', payload);
        state.user = result.user;
      } else {
        const email = $('prototype-email').value.trim();
        const password = $('prototype-password').value;
        if (!email || !password) {
          errorBox.textContent = 'Enter your email address and password to continue.';
          return;
        }
        const result = await postJSON('/api/v1/auth/login', { email: email, password: password });
        state.user = result.user;
      }
      await afterSignIn();
    } catch (error) {
      errorBox.textContent = error.message;
    }
  }

  async function afterSignIn() {
    const me = await api('/api/v1/auth/me');
    state.user = me.user;
    state.csrf = me.csrf;
    state.pictures = me.pictures || [];
    state.signedIn = true;
    applyTheme(state.user.theme);
    showScreen('app');
    switchTab('home');
    $('create').focus();
  }

  async function signOut() {
    try {
      await postJSON('/api/v1/auth/logout', {});
    } catch (e) { /* the cookie is cleared by the server anyway */ }
    try {
      localStorage.removeItem('victimcare.theme');
      localStorage.removeItem('victimcare.motion');
    } catch (e) { /* storage is optional */ }
    window.location.href = '/';
  }

  /* ------------------------------------------------------------- alerts */

  function showOverlay(part) {
    $('overlay').hidden = false;
    ['countdown', 'location', 'ready'].forEach((name) => { $(name + '-state').hidden = name !== part; });
  }

  function closeOverlay() {
    clearInterval(state.countdownTimer);
    state.countdownTimer = null;
    $('overlay').hidden = true;
    switchTab('home');
    $('sos').focus();
  }

  function startCountdown() {
    clearInterval(state.countdownTimer);
    let seconds = state.user.countdownSeconds || 5;
    $('countdown-number').textContent = seconds;
    showOverlay('countdown');
    $('cancel-alert').focus();
    state.countdownTimer = setInterval(() => {
      seconds -= 1;
      if (seconds <= 0) {
        clearInterval(state.countdownTimer);
        state.countdownTimer = null;
        prepareAlert('SOS emergency', '');
      } else {
        $('countdown-number').textContent = seconds;
      }
    }, 1000);
  }

  function buildMessage(kind, note, position) {
    const name = state.user.fullName || 'Someone';
    let location = 'location not shared';
    if (position) {
      location = 'https://maps.google.com/?q=' + position.coords.latitude + ',' + position.coords.longitude;
    }
    return name + ' needs help. ' + kind + '. ' + (note ? note + ' ' : '') + 'Location: ' + location;
  }

  async function prepareAlert(kind, note) {
    showOverlay('location');

    const position = await resolveLocation();
    const message = buildMessage(kind, note, position);

    try {
      const result = await postJSON('/api/v1/incidents', {
        kind: kind,
        note: note || '',
        latitude: position ? position.coords.latitude : null,
        longitude: position ? position.coords.longitude : null,
        locationShared: !!position
      });
      state.reports = result.incidents || [];
    } catch (e) {
      /* the alert still has to be usable when the server is unreachable */
    }

    if (state.user.vibrate && navigator.vibrate) navigator.vibrate([200, 100, 200]);

    const number = state.user.emergencyNumber || '112';
    $('call-link').href = 'tel:' + number;
    $('call-label').textContent = 'Call ' + number;

    const hasContact = !!state.contact.phone;
    $('text-link').hidden = !hasContact;
    $('no-contact').hidden = hasContact;
    if (hasContact) {
      $('text-link').href = 'sms:' + state.contact.phone + '?body=' + encodeURIComponent(message);
      $('text-label').textContent = 'Text ' + (state.contact.name || 'my contact');
    }
    showOverlay('ready');
    $('close-alert').focus();
  }

  function resolveLocation() {
    return new Promise((resolve) => {
      if (!state.user.shareLocation || !navigator.geolocation) {
        resolve(null);
        return;
      }
      navigator.geolocation.getCurrentPosition(
        (position) => resolve(position),
        () => resolve(null),
        { timeout: 8000, enableHighAccuracy: true, maximumAge: 30000 }
      );
    });
  }

  /* ------------------------------------------------------------ reports */

  function renderReports() {
    const list = $('reports-list');
    if (!list) return;
    list.innerHTML = '';
    $('reports-empty').hidden = state.reports.length > 0;
    state.reports.forEach((report) => {
      const card = document.createElement('article');
      card.className = 'report-card';

      const type = document.createElement('strong');
      type.textContent = report.kind;

      const note = document.createElement('span');
      note.textContent = report.note || 'No details';

      const time = document.createElement('small');
      time.textContent = new Date(report.created_at || report.createdAt || Date.now()).toLocaleString();

      card.append(type, document.createElement('br'), note, document.createElement('br'), time);
      list.appendChild(card);
    });
  }

  /* ----------------------------------------------------------- settings */

  function fillSettings() {
    $('profile-first-name').value = state.user.firstName || '';
    $('profile-second-name').value = state.user.secondName || '';
    $('profile-email').value = state.user.email || '';
    $('profile-state').value = state.user.state || '';
    $('profile-nation').value = state.user.nation || '';
    $('setting-number').value = state.user.emergencyNumber || '112';
    $('setting-location').checked = !!state.user.shareLocation;
    $('setting-vibration').checked = !!state.user.vibrate;
    $('setting-motion').checked = state.user.motion !== false;
    $('setting-countdown').value = String(state.user.countdownSeconds || 5);
    updateThemeSwatches(state.user.theme || 'light');
    updateAvatarPreview();
  }

  function updateAvatarPreview() {
    const preview = $('avatar-preview');
    if (!preview) return;
    const url = state.user.avatarUrl || '';
    preview.innerHTML = '';
    if (url) {
      const img = document.createElement('img');
      img.id = 'avatar-image';
      img.src = url;
      img.alt = 'Your profile picture';
      preview.appendChild(img);
    } else {
      const span = document.createElement('span');
      span.className = 'avatar-initials large';
      span.id = 'avatar-initials';
      span.textContent = state.user.initials || 'VC';
      preview.appendChild(span);
    }
    const button = $('avatar-button');
    if (button) {
      button.innerHTML = '';
      if (url) {
        const img = document.createElement('img');
        img.className = 'avatar-image';
        img.src = url;
        img.alt = '';
        button.appendChild(img);
      } else {
        const span = document.createElement('span');
        span.className = 'avatar-initials';
        span.textContent = state.user.initials || 'VC';
        button.appendChild(span);
      }
    }
  }

  function updateThemeSwatches(theme) {
    document.querySelectorAll('.theme-swatch').forEach((swatch) => {
      swatch.setAttribute('aria-checked', String(swatch.dataset.theme === theme));
    });
  }

  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    updateThemeSwatches(theme);
    try {
      localStorage.setItem('victimcare.theme', theme);
    } catch (e) { /* storage is optional */ }
  }

  function applyMotion(motion) {
    document.documentElement.setAttribute('data-motion', motion ? 'on' : 'off');
    try {
      localStorage.setItem('victimcare.motion', motion ? 'on' : 'off');
    } catch (e) { /* storage is optional */ }
  }

  async function saveProfile(event) {
    event.preventDefault();
    const status = $('profile-status');
    try {
      const result = await patchJSON('/api/v1/users/me', {
        firstName: $('profile-first-name').value.trim(),
        secondName: $('profile-second-name').value.trim(),
        email: $('profile-email').value.trim(),
        state: $('profile-state').value.trim(),
        nation: $('profile-nation').value.trim(),
        password: $('profile-password').value
      });
      state.user = result.user;
      $('profile-password').value = '';
      updateAvatarPreview();
      setStatus(status, 'Profile updated.', false);
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  async function saveSettings(event) {
    event.preventDefault();
    const status = $('settings-status');
    const motion = $('setting-motion').checked;
    try {
      const result = await putJSON('/api/v1/users/me/settings', {
        theme: state.user.theme || 'light',
        motion: motion,
        shareLocation: $('setting-location').checked,
        vibrate: $('setting-vibration').checked,
        emergencyNumber: $('setting-number').value.trim(),
        countdownSeconds: parseInt($('setting-countdown').value, 10)
      });
      state.user = result.settings;
      applyMotion(motion);
      updateSosLabel();
      setStatus(status, 'Alert settings updated.', false);
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  async function changeTheme(theme) {
    applyTheme(theme);
    state.user.theme = theme;
    updateThemeSwatches(theme);
    setStatus($('theme-status'), 'Theme changed to ' + theme + '.', false);
    try {
      const result = await putJSON('/api/v1/users/me/settings', {
        theme: theme,
        motion: $('setting-motion').checked,
        shareLocation: $('setting-location').checked,
        vibrate: $('setting-vibration').checked,
        emergencyNumber: $('setting-number').value.trim() || state.user.emergencyNumber || '112',
        countdownSeconds: state.user.countdownSeconds || 5
      });
      state.user = result.settings;
      setStatus($('theme-status'), 'Theme saved.', false);
    } catch (error) {
      setStatus($('theme-status'), error.message, true);
    }
  }

  async function changePassword(event) {
    event.preventDefault();
    const status = $('password-status');
    const current = $('current-password').value;
    const next = $('new-password').value;
    if (next.length < 8) {
      setStatus(status, 'Password must be at least 8 characters.', true);
      return;
    }
    if (next !== $('confirm-password').value) {
      setStatus(status, 'Passwords do not match.', true);
      return;
    }
    try {
      await postJSON('/api/v1/users/me/password', { currentPassword: current, newPassword: next });
      $('current-password').value = '';
      $('new-password').value = '';
      $('confirm-password').value = '';
      setStatus(status, 'Password changed.', false);
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  async function deleteAccount() {
    const password = window.prompt('Confirm your password to delete your account. This cannot be undone.');
    if (password === null) return;
    try {
      await api('/api/v1/users/me', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: password })
      });
      window.location.href = '/';
    } catch (error) {
      setStatus($('account-status'), error.message, true);
    }
  }

  /* ------------------------------------------------------------ pictures */

  function renderGallery(pictures) {
    const gallery = $('picture-gallery');
    gallery.innerHTML = '';
    pictures.forEach((picture) => {
      const figure = document.createElement('figure');
      figure.className = 'gallery-item' + (picture.primary ? ' is-primary' : '');
      figure.dataset.id = picture.id;

      const img = document.createElement('img');
      img.src = picture.thumbUrl;
      img.alt = 'Uploaded picture ' + picture.id;
      img.loading = 'lazy';
      img.addEventListener('click', () => openLightbox(picture.url));

      const caption = document.createElement('figcaption');

      const badge = document.createElement('span');
      badge.className = 'badge' + (picture.primary ? ' badge-primary' : '');
      badge.textContent = picture.primary ? 'Profile picture' : 'Picture';

      const primaryButton = document.createElement('button');
      primaryButton.type = 'button';
      primaryButton.className = 'link-button';
      primaryButton.dataset.action = 'primary';
      primaryButton.dataset.id = picture.id;
      primaryButton.textContent = 'Make profile picture';
      if (picture.primary) primaryButton.hidden = true;

      const deleteButton = document.createElement('button');
      deleteButton.type = 'button';
      deleteButton.className = 'link-button danger';
      deleteButton.dataset.action = 'delete';
      deleteButton.dataset.id = picture.id;
      deleteButton.textContent = 'Delete';

      caption.append(badge, primaryButton, deleteButton);
      figure.append(img, caption);
      gallery.appendChild(figure);
    });
    $('gallery-empty').hidden = pictures.length > 0;
  }

  async function uploadPicture(file) {
    const status = $('upload-status');
    if (!file) return;
    if (!/^image\/(png|jpeg)$/.test(file.type)) {
      setStatus(status, 'Please choose a JPG or PNG picture.', true);
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setStatus(status, 'That picture is too large. Please choose a file under 5 MB.', true);
      return;
    }
    setStatus(status, 'Uploading...', false);

    const form = new FormData();
    form.append('file', file);

    try {
      const result = await api('/api/v1/users/me/pictures', { method: 'POST', body: form });
      state.pictures = result.pictures || [];
      renderGallery(state.pictures);
      const me = await api('/api/v1/auth/me');
      state.user = me.user;
      updateAvatarPreview();
      setStatus(status, 'Picture uploaded.', false);
      $('avatar-input').value = '';
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  async function galleryAction(action, id) {
    const status = $('gallery-status');
    try {
      let result;
      if (action === 'primary') {
        result = await postJSON('/api/v1/users/me/pictures/' + id + '/primary', {});
      } else {
        result = await api('/api/v1/users/me/pictures/' + id, { method: 'DELETE' });
      }
      state.pictures = result.pictures || [];
      renderGallery(state.pictures);
      const me = await api('/api/v1/auth/me');
      state.user = me.user;
      updateAvatarPreview();
      setStatus(status, action === 'primary' ? 'Profile picture updated.' : 'Picture deleted.', false);
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  function openLightbox(url) {
    if (!url) return;
    $('lightbox-image').src = url;
    $('lightbox').hidden = false;
    $('lightbox-close').focus();
  }

  function closeLightbox() {
    $('lightbox').hidden = true;
    $('lightbox-image').src = '';
  }

  /* ----------------------------------------------- moving picture area */

  function initPictureArea() {
    const area = $('picture-area');
    const slides = Array.prototype.slice.call(area.querySelectorAll('.slide'));
    const dotsBox = $('picture-dots');
    if (!slides.length) return;

    let index = 0;
    let timer = null;
    let paused = false;
    const interval = 6000;

    const dots = slides.map((slide, i) => {
      const dot = document.createElement('button');
      dot.type = 'button';
      dot.className = 'picture-dot' + (i === 0 ? ' is-active' : '');
      dot.setAttribute('aria-label', 'Show picture ' + (i + 1));
      dot.addEventListener('click', () => show(i));
      dotsBox.appendChild(dot);
      return dot;
    });

    function show(next) {
      index = (next + slides.length) % slides.length;
      slides.forEach((slide, i) => slide.classList.toggle('is-active', i === index));
      dots.forEach((dot, i) => dot.classList.toggle('is-active', i === index));
    }

    function start() {
      stop();
      if (motionEnabled()) {
        timer = setInterval(() => {
          if (!paused) show(index + 1);
        }, interval);
      }
    }

    function stop() {
      if (timer) clearInterval(timer);
      timer = null;
    }

    area.addEventListener('mouseenter', () => { paused = true; });
    area.addEventListener('mouseleave', () => { paused = false; });
    area.addEventListener('focusin', () => { paused = true; });
    area.addEventListener('focusout', () => { paused = false; });

    document.addEventListener('visibilitychange', () => {
      if (document.hidden) stop();
      else start();
    });

    start();
  }

  function motionEnabled() {
    const attribute = document.documentElement.getAttribute('data-motion');
    if (attribute === 'off') return false;
    const stored = (function () {
      try { return localStorage.getItem('victimcare.motion'); } catch (e) { return null; }
    })();
    if (stored === 'off') return false;
    return !window.matchMedia || !window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  }

  /* --------------------------------------------------------- resize bar */

  function initResize() {
    const control = $('app-size');
    const reset = $('reset-size');
    const original = 100;
    function apply(value) {
      const size = Math.max(85, Math.min(120, Number(value) || original));
      $('app-shell').style.width = size + '%';
      $('app-shell').style.marginLeft = ((100 - size) / 2) + '%';
      $('app-shell').style.marginRight = ((100 - size) / 2) + '%';
      control.value = size;
      control.setAttribute('aria-valuenow', String(size));
    }
    control.addEventListener('input', (event) => apply(event.target.value));
    reset.addEventListener('click', () => apply(original));
  }

  /* --------------------------------------------------------------- wire */

  function wire() {
    $('open-login').addEventListener('click', () => openAuth('login'));
    $('open-signup').addEventListener('click', () => openAuth('signup'));
    $('back-to-welcome').addEventListener('click', () => showScreen('welcome'));

    document.querySelectorAll('.password-toggle').forEach((button) => {
      button.addEventListener('click', () => {
        const input = $(button.dataset.passwordTarget);
        const visible = input.type === 'text';
        input.type = visible ? 'password' : 'text';
        button.setAttribute('aria-pressed', String(!visible));
        button.textContent = visible ? 'Show' : 'Hide';
      });
    });

    $('prototype-form').addEventListener('submit', submitAuth);
    $('signout').addEventListener('click', signOut);
    $('settings-signout').addEventListener('click', signOut);
    $('avatar-button').addEventListener('click', () => switchTab('settings'));

    $('create').addEventListener('click', () => { switchTab('report'); $('note').focus(); });
    $('tabs').addEventListener('click', (event) => {
      const button = event.target.closest('[data-tab]');
      if (button) switchTab(button.dataset.tab);
    });
    $('home-view').addEventListener('click', (event) => {
      const tile = event.target.closest('[data-go]');
      if (tile) switchTab(tile.dataset.go);
    });

    document.querySelectorAll('[data-type]').forEach((button) => {
      button.addEventListener('click', () => {
        state.reportType = button.dataset.type;
        document.querySelectorAll('[data-type]').forEach((chip) => {
          chip.setAttribute('aria-pressed', String(chip === button));
        });
      });
    });

    $('send-report').addEventListener('click', async () => {
      const note = $('note').value.trim();
      $('note').value = '';
      await prepareAlert(state.reportType, note);
    });

    $('sos').addEventListener('click', startCountdown);
    $('cancel-alert').addEventListener('click', closeOverlay);
    $('close-alert').addEventListener('click', closeOverlay);

    $('contact-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const status = $('contact-status');
      try {
        const result = await postJSON('/api/v1/contacts', {
          name: $('contact-name').value.trim(),
          phone: $('contact-phone').value.trim()
        });
        state.contact = result.contact;
        setStatus(status, 'Contact updated.', false);
      } catch (error) {
        setStatus(status, error.message, true);
      }
    });

    $('profile-form').addEventListener('submit', saveProfile);
    $('settings-form').addEventListener('submit', saveSettings);
    $('password-form').addEventListener('submit', changePassword);
    $('delete-account').addEventListener('click', deleteAccount);

    $('clear-reports').addEventListener('click', async () => {
      try {
        const result = await api('/api/v1/incidents', { method: 'DELETE' });
        state.reports = result.incidents || [];
        renderReports();
        setStatus($('account-status'), 'Past reports cleared.', false);
      } catch (error) {
        setStatus($('account-status'), error.message, true);
      }
    });

    document.querySelectorAll('.theme-swatch').forEach((swatch) => {
      swatch.addEventListener('click', () => changeTheme(swatch.dataset.theme));
      swatch.addEventListener('keydown', (event) => {
        if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
        event.preventDefault();
        const swatches = Array.prototype.slice.call(document.querySelectorAll('.theme-swatch'));
        const current = swatches.indexOf(swatch);
        const next = (current + (event.key === 'ArrowRight' ? 1 : swatches.length - 1)) % swatches.length;
        swatches[next].focus();
        changeTheme(swatches[next].dataset.theme);
      });
    });

    $('setting-motion').addEventListener('change', (event) => {
      applyMotion(event.target.checked);
      setStatus($('theme-status'), event.target.checked ? 'Animations on.' : 'Animations off.', false);
    });

    $('avatar-input').addEventListener('change', (event) => uploadPicture(event.target.files[0]));
    $('picture-gallery').addEventListener('click', (event) => {
      const button = event.target.closest('[data-action]');
      if (button) galleryAction(button.dataset.action, button.dataset.id);
    });

    const preview = $('avatar-preview');
    if (preview) {
      preview.addEventListener('click', () => openLightbox(state.user.avatarUrl));
      preview.addEventListener('keydown', (event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          openLightbox(state.user.avatarUrl);
        }
      });
    }
    $('lightbox-close').addEventListener('click', closeLightbox);
    $('lightbox').addEventListener('click', (event) => {
      if (event.target === $('lightbox')) closeLightbox();
    });

    document.querySelectorAll('[data-doc]').forEach((link) => {
      link.addEventListener('click', (event) => {
        event.preventDefault();
        setStatus($('about-note'), 'The ' + link.dataset.doc + ' document is part of the full release of this app.', false);
      });
    });

    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        if (!$('lightbox').hidden) closeLightbox();
        else if (!$('overlay').hidden) closeOverlay();
      }
    });
  }

  /* --------------------------------------------------------------- boot */

  function boot() {
    wire();
    initResize();
    initPictureArea();

    if (state.signedIn) {
      applyMotion(state.user.motion !== false);
      state.countdownSeconds = state.user.countdownSeconds || 5;
      showScreen('app');
      switchTab('home');
      renderGallery(state.pictures);
    } else {
      showScreen('welcome');
      switchTab('home');
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
